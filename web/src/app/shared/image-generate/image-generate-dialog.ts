import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  Injector,
  computed,
  effect,
  inject,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { MatBottomSheetRef } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog, MatDialogRef } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import type { GalleryImage } from '../../../gen/meurpg/maps/v1/gallery_pb';
import { type ImageGenerationStatus } from '../../../gen/meurpg/maps/v1/imagegen_pb';
import { MapsClient } from '../../core/maps/maps-client';
import { RosterClient } from '../../core/maps/roster-client';
import { OpenSessionLookup } from '../../core/play/open-session';
import { ImageGenClient } from '../../core/images/imagegen-client';
import { SYNTHID, kindChoices, remainingText } from '../../core/images/imagegen-copy';
import { type GenerateIssue, generateIssue } from '../../core/images/imagegen-errors';
import { type ImageForm, buildRequest } from '../../core/images/imagegen-form';
import { ImageRun } from '../../core/images/imagegen-run';
import { ShownImageClient } from '../../core/images/shown-client';
import { SheetFrame } from '../sheet/sheet-frame/sheet-frame';
import { injectSheet } from '../sheet/sheet-host';
import { openImagePicker } from '../gallery-picker/image-picker-dialog/image-picker-dialog';
import { GenerateForm } from './generate-form';
import { GenerateResult } from './generate-result';
import { GenerateRunning, WAIT_NOTES } from './generate-running';
import type { GenerateOutcome, ImageGenerateData, Stage } from './image-generate.types';
import {
  doneView,
  editWhyOf,
  focusFieldOf,
  isStale,
  mainOf,
  namesReference,
  resultKindOf,
  subtitleOf,
  summaryOf,
  whyOf,
} from './image-generate-view';
import { ImageReferences } from './image-references';
import { focusAfterStage, scrollToQuestion } from './image-generate-focus';
import { ImageFormState } from './image-form-state';
import { RunFlow, guardClose } from './run-flow';
import { GeneratedResult } from './image-result';
import { referencePickerOptions } from './reference-picker';

/**
 * "Gerar imagem" (MR-039, RN-28; E10-07): the master asks the image service for a picture from a map, a scene or nothing, waits for it, sees
 * it, shows it to the players or adjusts it. A dialog on a computer (580 px) and a bottom sheet with a fixed footer on a phone; one filled
 * button at a time. The browser draws nothing: the server makes the picture and keeps it in the gallery, hidden until the master shows it.
 *
 * Four stages in the same frame (the title is the first focus of each, and the body scrolls to its top):
 * - `form`: the way, what goes along (the server's drawing of what the players see), who appears, the references, the text, the style, the
 *   ratio, the month's count, and under every field what goes to Google. A refusal (the month's limit, nobody to see, a wall that changed...)
 *   says what to do, by its typed detail, over the form, which keeps what was written;
 * - `running`: "Enviando o pedido…" and then "Gerando a imagem…" with the long poll behind it. "Cancelar" and "Parar de esperar" are the
 *   same call and the words under them say what became of the slot. Closing asks "Parar de esperar a imagem?" first;
 * - `result`: the picture, its actions and the chain of adjustments;
 * - a request that ended without a picture returns to the form (or to the picture it was an adjustment of) with the reason.
 */
@Component({
  selector: 'app-image-generate-dialog',
  imports: [
    GenerateForm,
    GenerateResult,
    GenerateRunning,
    MatButtonModule,
    MatIconModule,
    SheetFrame,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './image-generate-dialog.html',
  styleUrl: './image-generate-dialog.scss',
})
export class ImageGenerateDialog {
  private readonly api = inject(ImageGenClient);
  private readonly dialog = inject(MatDialog);
  private readonly injector = inject(Injector);
  private readonly sessions = inject(OpenSessionLookup);
  private readonly sheet = injectSheet<ImageGenerateData, GenerateOutcome>();
  private readonly dialogRef = inject<MatDialogRef<unknown, GenerateOutcome>>(MatDialogRef, {
    optional: true,
  });
  private readonly sheetRef = inject<MatBottomSheetRef<unknown, GenerateOutcome>>(
    MatBottomSheetRef,
    { optional: true },
  );
  private readonly frame = viewChild(SheetFrame);

  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  protected readonly synthid = SYNTHID;
  protected readonly wait = WAIT_NOTES;

  protected readonly stage = signal<Stage>('loading');
  protected readonly status = signal<ImageGenerationStatus | null>(null);
  /** The form the master fills, the references he picked and the refusal over them. */
  protected readonly formState = new ImageFormState(this.data.origin);
  protected readonly form = this.formState.form;
  protected readonly references = this.formState.references;
  protected readonly issue = this.formState.issue;
  /** A calm line over the form (after a cancel): what became of the slot. */
  protected readonly notice = signal<string | null>(null);

  /** What the server drew of the map and who is on it. */
  private readonly refs = new ImageReferences({
    api: this.api,
    roster: inject(RosterClient),
    maps: inject(MapsClient),
    campaignId: this.data.campaignId,
  });
  protected readonly npcs = this.refs.npcs;
  protected readonly referenceState = this.refs.state;
  protected readonly textureGrid = this.refs.textureGrid;
  protected readonly hiddenPortraits = this.refs.hiddenPortraits;

  /** The request in the air and the question that closing asks. */
  protected readonly flow = new RunFlow({
    api: this.api,
    campaignId: this.data.campaignId,
    injector: this.injector,
    stage: () => this.stage(),
    started: () => {
      this.stage.set('running');
      this.focusAfterStage();
    },
    status: (s) => this.status.set(s),
    done: (run) => this.onDone(run),
    ended: (text, failed) => this.backWith(text, failed),
    finish: () => this.finish(),
  });
  protected readonly run = this.flow.run;
  private readonly runFor = this.flow.runFor;

  /** The picture on screen and what can be done with it. */
  protected readonly result = new GeneratedResult({
    api: this.api,
    shownApi: inject(ShownImageClient),
    campaignId: this.data.campaignId,
    toTheQuestion: () => scrollToQuestion(this.injector),
  });
  protected readonly pickChain = (imageId: string) =>
    this.result.pickChain(imageId, this.mapId() !== null);

  /** The campaign has an open session (unknown counts as open: the call says when it is not). */
  protected readonly sessionOpen = signal(true);
  protected readonly instruction = signal('');
  protected readonly editError = signal<string | null>(null);
  protected readonly runPrompt = signal('');
  private generated = 0;
  private lastImageId: string | null = null;
  private readonly lastForm = signal<ImageForm | null>(null);
  private lastGrid = '';

  private readonly mapId = computed(() =>
    this.data.origin.kind === 'map' && this.data.origin.hasGrid !== false
      ? (this.data.origin.mapId ?? null)
      : null,
  );
  protected readonly choices = computed(() =>
    kindChoices(this.data.origin, this.refs.textureTooLarge()),
  );
  protected readonly reference = computed(() =>
    this.form().kind === 'texture' ? this.refs.texture() : this.refs.scene(),
  );

  /** Why "Gerar imagem" cannot be pressed yet, in words (the button is dashed and the line is above it); empty when it can. */
  protected readonly why = computed(() =>
    whyOf({
      status: this.status(),
      form: this.form(),
      mapId: this.mapId(),
      referenceState: this.referenceState(),
      reference: this.reference(),
    }),
  );

  protected readonly title = computed(() =>
    // The image's name, never "Imagem N": the players read the same name when it is shown.
    this.stage() === 'result' ? this.result.view()?.image.name || 'Imagem' : 'Gerar imagem',
  );

  protected readonly subtitle = computed(() =>
    subtitleOf({
      origin: this.data.origin,
      stage: this.stage(),
      sending: this.run()?.phase() === 'sending',
      view: this.result.view(),
      kindText: this.result.kindText(),
    }),
  );

  /** What the running request is called over its text. */
  protected readonly summary = computed(() =>
    summaryOf(this.runFor(), this.lastForm() ?? this.form(), this.mapId() !== null),
  );

  /** The result's main action, in the footer (the artboard's order: the adjustment, then the one that goes on): use the textured map, or show the picture. */
  protected readonly main = computed(() =>
    mainOf(
      this.result.view(),
      this.mapId() !== null,
      this.result.useStage() === 'done',
      this.result.shown(),
    ),
  );
  protected readonly canUse = computed(
    () => this.mapId() !== null && (this.result.view()?.wholeMap ?? false),
  );
  protected readonly editWhy = computed(() => editWhyOf(this.status(), this.instruction()));
  protected readonly remaining = computed(() => {
    const s = this.status();
    return s ? remainingText(s) : '';
  });

  constructor() {
    // Closing must always go through `requestClose`: a request in the air is asked about first.
    guardClose(this.dialogRef, this.flow);
    guardClose(this.sheetRef, this.flow);
    inject(DestroyRef).onDestroy(() => this.run()?.destroy());
    effect(() => {
      const run = this.run();
      if (!run) {
        return;
      }
      const phase = run.phase();
      untracked(() => this.flow.onPhase(run, phase));
    });
    void this.load();
  }

  // ---- Opening

  private async load(): Promise<void> {
    const cid = this.data.campaignId;
    const mapId = this.mapId();
    const status = this.api.status(cid).then(
      (s) => this.status.set(s),
      () => undefined,
    );
    if (mapId !== null) {
      this.referenceState.set('loading');
    }
    const refs = mapId !== null ? this.refs.load(mapId) : Promise.resolve();
    const edits = this.data.image ? this.openOn(this.data.image) : Promise.resolve();
    void this.sessions.currentMap(cid).then((open) => this.sessionOpen.set(open !== null));
    await Promise.all([status, refs, edits]);
    if (mapId !== null) {
      void this.loadHiddenPortraits(mapId);
    }
    if (!this.data.image) {
      this.stage.set('form');
    }
  }

  private async loadHiddenPortraits(mapId: string): Promise<void> {
    const hidden = await this.refs.readHiddenPortraits(mapId);
    if (hidden && hidden.size > 0 && this.references().some((r) => hidden.has(r.id))) {
      this.formState.dropHidden(hidden);
    }
  }

  protected retryReference(): void {
    const mapId = this.mapId();
    if (mapId !== null) {
      void this.refs.load(mapId);
    }
  }

  /** An older generated image: its chain, and its place in it. */
  private async openOn(image: GalleryImage): Promise<void> {
    await this.result.openOn(image);
    this.stage.set('result');
    this.focusAfterStage();
  }

  // ---- The form

  protected pickReference(): void {
    openImagePicker(
      this.dialog,
      referencePickerOptions({
        campaignId: this.data.campaignId,
        excluded: this.refs.hiddenPortraits(),
        has: (id) => this.formState.hasReference(id),
        add: (image) => this.formState.addReference(image),
      }),
      this.injector,
      this.inSheet,
    );
  }

  protected async submit(): Promise<void> {
    if (this.why() !== '' || this.stage() !== 'form') {
      return;
    }
    const f = this.form();
    const cid = this.data.campaignId;
    const built = buildRequest(
      f,
      this.mapId(),
      this.flow.keyFor(JSON.stringify(['generate', this.mapId(), f])),
    );
    this.issue.set(null);
    this.notice.set(null);
    this.lastForm.set(f);
    this.runFor.set('generate');
    this.runPrompt.set(f.prompt.trim());
    const ref = this.reference();
    this.lastGrid = ref && ref.gridColumns > 0 ? `${ref.gridColumns} × ${ref.gridRows}` : '';
    this.flow.begin(
      (signal) =>
        built.via === 'map'
          ? this.api.generateMap(cid, built.request, signal)
          : this.api.generateScene(cid, built.request, signal),
      (err) => this.refused(err, false),
    );
  }

  /** A request refused before anything was reserved: back to the form (or to the picture), with the reason. */
  private refused(err: unknown, edit: boolean): void {
    const issue = generateIssue(err, edit ? 'edit' : undefined);
    // A typed refusal is an answer: the next try is a new request. A lost answer is not: the key stays, and a retry never spends two slots.
    this.flow.dropKeyOnAnswer(err);
    if (issue.status) {
      this.status.set(issue.status);
    }
    if (edit) {
      this.editError.set(issue.text);
      this.stage.set('result');
      this.focusAfterStage();
      return;
    }
    this.issue.set(issue);
    this.stage.set('form');
    void this.refreshAfter(issue);
  }

  /**
   * After a refusal that says the form is out of date (an NPC the players no longer see, nobody on the map, a map that changed): the drawing
   * and the NPC list are read again, the marked NPCs that are gone are dropped, and the field the server named gets the focus.
   */
  private async refreshAfter(issue: GenerateIssue): Promise<void> {
    const mapId = this.mapId();
    if (isStale(issue) && mapId !== null) {
      await this.refs.load(mapId);
      this.formState.keepNpcs(new Set(this.npcs().map((n) => n.characterId)));
      void this.loadHiddenPortraits(mapId);
    }
    if (namesReference(issue)) {
      this.formState.dropHidden(this.refs.hiddenPortraits());
    }
    this.focusAfterStage(focusFieldOf(issue));
  }

  // ---- Waiting

  protected finish(): void {
    this.sheet.close({
      generated: this.generated,
      lastImageId: this.lastImageId,
      map: this.result.changedMap,
    });
  }

  /** A request that ended without a picture: back to the form (or to the picture it adjusted), with the words. */
  private backWith(text: string, failed: boolean): void {
    this.run.set(null);
    if (this.runFor() === 'edit') {
      this.editError.set(failed ? text : null);
      this.notice.set(failed ? null : text);
      this.stage.set('result');
    } else {
      this.issue.set(failed ? { text, field: null, reason: null, status: null } : null);
      this.notice.set(failed ? null : text);
      this.stage.set('form');
    }
    this.focusAfterStage();
  }

  private onDone(run: ImageRun): void {
    const image = run.image();
    const generation = run.generation();
    if (!image || !generation) {
      return;
    }
    this.generated++;
    this.lastImageId = image.id;
    const form = this.lastForm() ?? this.form();
    const edit = this.runFor() === 'edit';
    this.result.present(
      doneView({
        image,
        number: generation.number,
        form,
        edit,
        instruction: this.instruction(),
        fromMap: this.mapId() !== null,
        grid: this.lastGrid,
      }),
      resultKindOf(edit, form.kind),
    );
    this.editError.set(null);
    this.instruction.set('');
    this.notice.set(null);
    this.run.set(null);
    this.stage.set('result');
    this.focusAfterStage();
    void this.result.loadChain(image);
  }

  // ---- The result

  protected submitEdit(): void {
    const image = this.result.view()?.image;
    const text = this.instruction().trim();
    if (!image || this.editWhy() !== '' || this.stage() !== 'result') {
      return;
    }
    const cid = this.data.campaignId;
    this.editError.set(null);
    this.notice.set(null);
    this.runFor.set('edit');
    this.runPrompt.set(text);
    const key = this.flow.keyFor(JSON.stringify(['edit', image.id, text]));
    this.flow.begin(
      (signal) => this.api.edit(cid, image.id, text, key, signal),
      (err) => this.refused(err, true),
    );
  }

  // ---- Focus

  private focusAfterStage(field?: 'npc' | 'refs'): void {
    focusAfterStage(this.injector, () => this.frame()?.scrollToTop(), field);
  }
}
