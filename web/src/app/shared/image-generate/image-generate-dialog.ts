import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { MatBottomSheet, MatBottomSheetRef } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog, MatDialogRef } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import type { Observable } from 'rxjs';

import type { GalleryImage } from '../../../gen/meurpg/maps/v1/gallery_pb';
import {
  ImageAspectRatio,
  type ImageEdit,
  ImageGenerationBlockedReason,
  ImageGenerationKind,
  type ImageGenerationStatus,
  ImageStyle,
} from '../../../gen/meurpg/maps/v1/imagegen_pb';
import type { Map as MapMessage } from '../../../gen/meurpg/maps/v1/maps_pb';
import { newRequestKey, pngDataUrl } from '../../core/images/bytes-url';
import { MapsClient } from '../../core/maps/maps-client';
import { RosterClient } from '../../core/maps/roster-client';
import { OpenSessionLookup } from '../../core/play/open-session';
import { ImageGenClient } from '../../core/images/imagegen-client';
import {
  type GenerateOrigin,
  KIND_LABEL,
  defaultImageName,
  type KindKey,
  SYNTHID,
  kindChoices,
  remainingText,
  requestLine,
  resultCaption,
} from '../../core/images/imagegen-copy';
import {
  type GenerateIssue,
  blockedOf,
  blockedText,
  generateIssue,
  invalidField,
  useIssue,
} from '../../core/images/imagegen-errors';
import {
  type ImageForm,
  type PickableNpc,
  buildRequest,
  formProblem,
} from '../../core/images/imagegen-form';
import { ImageRun } from '../../core/images/imagegen-run';
import { ShownImageClient, showImageIssue } from '../../core/images/shown-client';
import { SheetFrame } from '../sheet/sheet-frame/sheet-frame';
import { injectSheet, openSheet } from '../sheet/sheet-host';
import { openImagePicker } from '../gallery-picker/image-picker-dialog/image-picker-dialog';
import { type ReferenceView, GenerateForm } from './generate-form';
import { GenerateResult, type ResultView, type ShowStage, type UseStage } from './generate-result';
import { GenerateRunning, WAIT_NOTES } from './generate-running';

/** What the opener hands the dialog. */
export interface ImageGenerateData {
  readonly campaignId: string;
  readonly origin: GenerateOrigin;
  /** An older generated image of the gallery: the dialog opens on it, to adjust it (E10-07 7). */
  readonly image?: GalleryImage;
}

/** What the dialog tells the opener when it closes: how many pictures it made, and the map it changed ("Usar como imagem do mapa"). */
export interface GenerateOutcome {
  readonly generated: number;
  readonly map: MapMessage | null;
}

type Stage = 'loading' | 'form' | 'running' | 'result';

/** Where a request goes back to when it ends without a picture: a new image to the form, an adjustment to the result it started from. */
type RunFor = 'generate' | 'edit';

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
  private readonly shownApi = inject(ShownImageClient);
  private readonly dialog = inject(MatDialog);
  private readonly injector = inject(Injector);
  private readonly sessions = inject(OpenSessionLookup);
  private readonly roster = inject(RosterClient);
  private readonly mapsApi = inject(MapsClient);
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
  protected readonly form = signal<ImageForm>({
    kind: 'scene',
    prompt: this.data.origin.kind === 'scene' ? (this.data.origin.name ?? '') : '',
    name: defaultImageName(this.data.origin, 'scene'),
    style: ImageStyle.OIL_PAINTING,
    ratio: ImageAspectRatio.IMAGE_ASPECT_RATIO_UNSPECIFIED,
    objectImageIds: [],
    npcIds: [],
  });
  protected readonly references = signal<readonly GalleryImage[]>([]);
  protected readonly issue = signal<GenerateIssue | null>(null);
  /** A calm line over the form (after a cancel): what became of the slot. */
  protected readonly notice = signal<string | null>(null);

  // What the server drew of the map, for the way chosen: one for the scene art and the isometric view, one for the textured map.
  private readonly sceneRef = signal<ReferenceView | null>(null);
  private readonly textureRef = signal<ReferenceView | null>(null);
  protected readonly npcs = signal<readonly PickableNpc[]>([]);
  protected readonly textureTooLarge = signal(false);
  protected readonly referenceState = signal<'none' | 'loading' | 'ready' | 'error'>('none');

  protected readonly run = signal<ImageRun | null>(null);
  private readonly runFor = signal<RunFor>('generate');
  protected readonly closing = signal(false);

  // The result.
  protected readonly view = signal<ResultView | null>(null);
  protected readonly chain = signal<readonly ImageEdit[]>([]);
  protected readonly shownIds = signal<ReadonlySet<string>>(new Set());
  protected readonly showBusy = signal(false);
  protected readonly showError = signal<string | null>(null);
  protected readonly showStage = signal<ShowStage>('idle');
  /** The campaign has an open session (unknown counts as open: the call says when it is not). */
  protected readonly sessionOpen = signal(true);
  /** The portraits of NPCs on this map that the players do not see: a request would be refused with them, so the picker leaves them out. */
  protected readonly hiddenPortraits = signal<ReadonlySet<string>>(new Set());
  private nameTouched = false;
  /** The key of the try in the air: kept through a lost answer, dropped once the server answered. */
  private pendingKey: { readonly sig: string; readonly key: string } | null = null;
  protected readonly useStage = signal<UseStage>('idle');
  protected readonly useError = signal<string | null>(null);
  protected readonly instruction = signal('');
  protected readonly editError = signal<string | null>(null);
  private generated = 0;
  private changedMap: MapMessage | null = null;
  private readonly lastForm = signal<ImageForm | null>(null);
  private lastGrid = '';

  private readonly mapId = computed(() =>
    this.data.origin.kind === 'map' && this.data.origin.hasGrid !== false
      ? (this.data.origin.mapId ?? null)
      : null,
  );
  protected readonly choices = computed(() =>
    kindChoices(this.data.origin, this.textureTooLarge()),
  );
  protected readonly textureGrid = computed(() => {
    const t = this.textureRef();
    return t ? { columns: t.gridColumns, rows: t.gridRows } : null;
  });
  protected readonly reference = computed(() =>
    this.form().kind === 'texture' ? this.textureRef() : this.sceneRef(),
  );

  /** Why "Gerar imagem" cannot be pressed yet, in words (the button is dashed and the line is above it); empty when it can. */
  protected readonly why = computed(() => {
    const s = this.status();
    if (s && !s.enabled) {
      return 'A geração de imagens não está ligada neste servidor.';
    }
    if (s && s.enabled && s.remaining <= 0) {
      return blockedText(ImageGenerationBlockedReason.LIMIT_REACHED, s);
    }
    if (
      this.mapId() !== null &&
      this.form().kind !== 'texture' &&
      this.referenceState() === 'ready' &&
      this.reference()?.playersSee === false
    ) {
      return blockedText(ImageGenerationBlockedReason.PLAYERS_SEE_NOTHING, null);
    }
    if (this.mapId() !== null && this.referenceState() === 'loading') {
      return 'Montando o desenho que vai junto…';
    }
    return formProblem(this.form(), s?.maxPromptCharacters || undefined) ?? '';
  });

  protected readonly title = computed(() => {
    switch (this.stage()) {
      // The image's name, never "Imagem N": the players read the same name when it is shown.
      case 'result':
        return this.view()?.image.name || 'Imagem';
      default:
        return 'Gerar imagem';
    }
  });

  protected readonly subtitle = computed(() => {
    const origin = this.data.origin;
    const place = origin.kind === 'scene' ? `Cena: ${origin.name ?? ''}` : (origin.name ?? '');
    switch (this.stage()) {
      case 'running':
        return this.run()?.phase() === 'sending'
          ? 'Antes de o pedido sair do servidor'
          : 'O servidor está gerando a imagem';
      case 'result': {
        const v = this.view();
        // From the gallery there is no map to name: the way alone ("O mapa com textura").
        // The heading is the image's name: when it already holds the map's name the subtitle says only the way.
        const named =
          v !== null && (origin.name ?? '') !== '' && v.image.name.includes(origin.name ?? '');
        return [origin.kind === 'gallery' || named ? '' : place, v ? this.resultKindText() : '']
          .filter((t) => t !== '')
          .join(' · ');
      }
      default:
        return place;
    }
  });

  private readonly resultKindText = signal('');

  /** What the running request is called over its text. */
  protected readonly summary = computed(() => {
    const f = this.lastForm() ?? this.form();
    return this.runFor() === 'edit'
      ? requestLine('edit', f.style, 0)
      : requestLine(
          f.kind,
          f.style,
          this.mapId() !== null && f.kind !== 'texture' ? f.npcIds.length : 0,
        );
  });
  protected readonly runPrompt = signal('');

  /** The result's main action, in the footer (the artboard's order: the adjustment, then the one that goes on): use the textured map, or show the picture. */
  protected readonly main = computed<'use' | 'show' | 'ask-show' | null>(() => {
    const v = this.view();
    if (!v) {
      return null;
    }
    if (v.wholeMap && this.mapId() !== null) {
      return this.useStage() === 'done' ? null : 'use';
    }
    if (this.shown()) {
      return null;
    }
    return v.wholeMap ? 'ask-show' : 'show';
  });
  protected readonly canUse = computed(
    () => this.mapId() !== null && (this.view()?.wholeMap ?? false),
  );
  /** The result's actions are not offered while a question over them is open ("Usar…", "Mostrar o mapa inteiro?"). */
  protected readonly asking = computed(
    () =>
      this.useStage() === 'asking' || this.useStage() === 'busy' || this.showStage() === 'asking',
  );
  protected readonly editWhy = computed(() => {
    const s = this.status();
    if (s && s.enabled && s.remaining <= 0) {
      return blockedText(ImageGenerationBlockedReason.LIMIT_REACHED, s);
    }
    return this.instruction().trim() === '' ? 'Escreva o ajuste que você quer pedir.' : '';
  });
  protected readonly remaining = computed(() => {
    const s = this.status();
    return s ? remainingText(s) : '';
  });

  constructor() {
    // Closing must always go through `requestClose`: a request in the air is asked about first.
    if (this.dialogRef) {
      this.dialogRef.disableClose = true;
      this.dialogRef
        .keydownEvents()
        .subscribe(
          (e) => e.key === 'Escape' && (this.closing() ? this.keepWaiting() : this.requestClose()),
        );
      this.dialogRef.backdropClick().subscribe(() => this.requestClose());
    }
    if (this.sheetRef) {
      this.sheetRef.disableClose = true;
      this.sheetRef
        .keydownEvents()
        .subscribe(
          (e) => e.key === 'Escape' && (this.closing() ? this.keepWaiting() : this.requestClose()),
        );
      this.sheetRef.backdropClick().subscribe(() => this.requestClose());
    }
    inject(DestroyRef).onDestroy(() => this.run()?.destroy());
    effect(() => {
      const run = this.run();
      if (!run) {
        return;
      }
      const phase = run.phase();
      untracked(() => this.onPhase(run, phase));
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
    const refs = mapId !== null ? this.loadReferences(mapId) : Promise.resolve();
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

  /** The drawing for the scene art (and the isometric view, which starts from the same view) and for the textured map, in parallel. */
  private async loadReferences(mapId: string): Promise<void> {
    this.referenceState.set('loading');
    const cid = this.data.campaignId;
    try {
      const [scene, texture] = await Promise.all([
        this.api.reference(cid, mapId, ImageGenerationKind.MAP_SCENE),
        this.api.reference(cid, mapId, ImageGenerationKind.TEXTURED_MAP),
      ]);
      this.sceneRef.set(viewOf(scene));
      this.textureRef.set(viewOf(texture));
      this.textureTooLarge.set(texture.textureTooLarge);
      this.npcs.set(
        scene.creatures.map((c) => ({
          characterId: c.characterId,
          name: c.name,
          portraitImageId: c.portraitImageId,
        })),
      );
      this.referenceState.set('ready');
    } catch {
      this.referenceState.set('error');
    }
  }

  /**
   * The portraits of the NPCs that have a token on this map and are not on the players' list: the server refuses them as references (also
   * as objects), so the picker does not offer them. Best effort: when it cannot be read, the server's refusal says it.
   */
  private async loadHiddenPortraits(mapId: string): Promise<void> {
    try {
      const [list, map] = await Promise.all([
        this.roster.list(this.data.campaignId),
        this.mapsApi.get(this.data.campaignId, mapId),
      ]);
      const seen = new Set(this.npcs().map((n) => n.characterId));
      const onMap = new Set(map.tokens.map((t) => t.characterId));
      const hidden = new Set(
        list
          .filter(
            (c) =>
              c.playerUserId === '' &&
              onMap.has(c.id) &&
              !seen.has(c.id) &&
              c.portraitImageId !== '',
          )
          .map((c) => c.portraitImageId),
      );
      this.hiddenPortraits.set(hidden);
      if (hidden.size > 0 && this.references().some((r) => hidden.has(r.id))) {
        this.references.update((l) => l.filter((r) => !hidden.has(r.id)));
        this.form.update((f) => ({
          ...f,
          objectImageIds: f.objectImageIds.filter((id) => !hidden.has(id)),
        }));
      }
    } catch {
      // The picker offers everything; a refused portrait is said by the server.
    }
  }

  protected retryReference(): void {
    const mapId = this.mapId();
    if (mapId !== null) {
      void this.loadReferences(mapId);
    }
  }

  /** An older generated image: its chain, and its place in it. */
  private async openOn(image: GalleryImage): Promise<void> {
    let chain: ImageEdit[] = [];
    try {
      chain = await this.api.edits(this.data.campaignId, image.id);
    } catch {
      // The chain is a detail: the picture opens without it.
    }
    this.chain.set(chain);
    const here = chain.find((e) => e.image?.id === image.id);
    this.resultKindText.set(image.showsWholeMap ? KIND_LABEL.texture.long : 'Imagem gerada');
    this.view.set({
      image,
      number: here?.number ?? 0,
      caption: here?.prompt ? `Pedido: ${here.prompt}` : '',
      texture: false,
      wholeMap: image.showsWholeMap,
    });
    this.stage.set('result');
    this.focusAfterStage();
  }

  // ---- The form

  protected patch(patch: Partial<ImageForm>): void {
    if (patch.name !== undefined) {
      this.nameTouched = true;
    }
    this.form.update((f) => {
      const next = { ...f, ...patch };
      // Until the master writes a name, it follows the way ("… · vista isométrica").
      return patch.kind !== undefined && !this.nameTouched
        ? { ...next, name: defaultImageName(this.data.origin, patch.kind) }
        : next;
    });
    this.issue.set(null);
  }

  protected pickReference(): void {
    const cid = this.data.campaignId;
    openImagePicker(
      this.dialog,
      {
        campaignId: cid,
        title: 'Escolher uma referência',
        lead: 'A imagem vai ao Google como referência para a nova imagem.',
        confirmLabel: 'Usar como referência',
        confirmIcon: 'image',
        current: null,
        currentTag: '',
        currentNote: '',
        note: () => 'Não use foto de pessoa: a imagem inteira vai ao Google.',
        noteIcon: 'info',
        hiddenMapImages: new Map(),
        excluded: this.hiddenPortraits(),
        emptyError: 'Escolha uma imagem para usar como referência.',
        submit: async (image) => {
          if (this.form().objectImageIds.includes(image.id)) {
            throw new Error('duplicate');
          }
          this.references.update((list) => [...list, image]);
          this.patch({ objectImageIds: [...this.form().objectImageIds, image.id] });
        },
        errorMessage: () => 'Essa imagem já é uma referência.',
      },
      this.injector,
      this.inSheet,
    );
  }

  protected removeReference(id: string): void {
    this.references.update((list) => list.filter((i) => i.id !== id));
    this.patch({ objectImageIds: this.form().objectImageIds.filter((i) => i !== id) });
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
      this.keyFor(JSON.stringify(['generate', this.mapId(), f])),
    );
    this.issue.set(null);
    this.notice.set(null);
    this.lastForm.set(f);
    this.runFor.set('generate');
    this.runPrompt.set(f.prompt.trim());
    const ref = this.reference();
    this.lastGrid = ref && ref.gridColumns > 0 ? `${ref.gridColumns} × ${ref.gridRows}` : '';
    this.begin(
      (signal) =>
        built.via === 'map'
          ? this.api.generateMap(cid, built.request, signal)
          : this.api.generateScene(cid, built.request, signal),
      (err) => {
        const issue = generateIssue(err);
        // A typed refusal is an answer: the next try is a new request. A lost answer is not: the key stays, and a retry never spends two slots.
        this.dropKeyOnAnswer(err);
        if (issue.status) {
          this.status.set(issue.status);
        }
        this.issue.set(issue);
        this.stage.set('form');
        void this.refreshAfter(issue);
      },
    );
  }

  /** The key of the try with this content: the same while nothing came back and nothing changed; a new one otherwise. */
  private keyFor(signature: string): string {
    if (this.pendingKey?.sig !== signature) {
      this.pendingKey = { sig: signature, key: newRequestKey() };
    }
    return this.pendingKey.key;
  }

  private dropKeyOnAnswer(err: unknown): void {
    if (blockedOf(err) !== null || invalidField(err) !== null) {
      this.pendingKey = null;
    }
  }

  /**
   * After a refusal that says the form is out of date (an NPC the players no longer see, nobody on the map, a map that changed): the drawing
   * and the NPC list are read again, the marked NPCs that are gone are dropped, and the field the server named gets the focus.
   */
  private async refreshAfter(issue: GenerateIssue): Promise<void> {
    const stale =
      issue.field === 'npc_character_ids' ||
      issue.reason === ImageGenerationBlockedReason.PLAYERS_SEE_NOTHING ||
      issue.reason === ImageGenerationBlockedReason.MAP_CHANGED;
    const mapId = this.mapId();
    if (stale && mapId !== null) {
      await this.loadReferences(mapId);
      const listed = new Set(this.npcs().map((n) => n.characterId));
      this.form.update((f) => ({ ...f, npcIds: f.npcIds.filter((id) => listed.has(id)) }));
      void this.loadHiddenPortraits(mapId);
    }
    if (issue.field === 'object_image_ids' || issue.field === 'character_image_ids') {
      const hidden = this.hiddenPortraits();
      this.references.update((l) => l.filter((r) => !hidden.has(r.id)));
      this.form.update((f) => ({
        ...f,
        objectImageIds: f.objectImageIds.filter((id) => !hidden.has(id)),
      }));
    }
    this.focusAfterStage(
      issue.field === 'npc_character_ids'
        ? 'npc'
        : issue.field === 'object_image_ids' || issue.field === 'character_image_ids'
          ? 'refs'
          : undefined,
    );
  }

  // ---- Waiting

  private begin(
    ask: (signal: AbortSignal) => ReturnType<ImageGenClient['generateScene']>,
    refused: (err: unknown) => void,
  ): void {
    const run = new ImageRun(this.api, this.data.campaignId, ask);
    this.closing.set(false);
    this.run.set(run);
    this.stage.set('running');
    this.focusAfterStage();
    run.start().catch((err: unknown) => {
      // Refused before anything was reserved: back to where it came from, with the reason.
      this.run.set(null);
      refused(err);
    });
  }

  protected cancelRun(): void {
    void this.run()?.cancel();
  }

  protected askClose(): void {
    this.closing.set(true);
    // The question is read at once, and its answers are in the footer.
    afterNextRender(() => document.getElementById('gen-close-t')?.focus(), {
      injector: this.injector,
    });
  }

  protected keepWaiting(): void {
    this.closing.set(false);
  }

  /** Esc, the ✕ and the backdrop: a request still going asks first (E10-07 4); anything else closes. */
  protected requestClose(): void {
    const run = this.run();
    if (this.stage() === 'running' && run?.running()) {
      this.askClose();
      return;
    }
    this.finish();
  }

  /** "Parar de esperar" in the question: stops the wait and closes. */
  protected async stopAndClose(): Promise<void> {
    const run = this.run();
    if (run?.running()) {
      await run.cancel();
    }
    this.finish();
  }

  protected finish(): void {
    this.sheet.close({ generated: this.generated, map: this.changedMap });
  }

  private onPhase(run: ImageRun, phase: ReturnType<ImageRun['phase']>): void {
    const s = run.status();
    if (s) {
      this.status.set(s);
    }
    if (phase !== 'sending') {
      // The server has the request (or the master canceled it): the next try is another request, with its own key.
      this.pendingKey = null;
    }
    if (phase === 'done') {
      this.onDone(run);
    } else if (phase === 'failed') {
      this.backWith(run.failure() ?? '', true);
    } else if (phase === 'canceled') {
      this.backWith(run.note() ?? '', false);
    }
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
    const f = this.lastForm() ?? this.form();
    const edit = this.runFor() === 'edit';
    const kind: KindKey = f.kind;
    this.resultKindText.set(edit ? 'Ajuste' : KIND_LABEL[kind].long);
    this.showStage.set('idle');
    const whole = image.showsWholeMap;
    const texture = whole && this.mapId() !== null;
    this.view.set({
      image,
      number: generation.number,
      texture,
      wholeMap: whole,
      caption: edit
        ? resultCaption({ edit: this.instruction().trim() })
        : resultCaption({
            kind,
            fromMap: this.mapId() !== null,
            style: f.style,
            npcs: whole || this.mapId() === null ? 0 : f.npcIds.length,
            grid: this.lastGrid,
          }),
    });
    this.useStage.set('idle');
    this.useError.set(null);
    this.showError.set(null);
    this.editError.set(null);
    this.instruction.set('');
    this.notice.set(null);
    this.run.set(null);
    this.stage.set('result');
    this.focusAfterStage();
    void this.loadChain(image);
  }

  private async loadChain(image: GalleryImage): Promise<void> {
    try {
      const chain = await this.api.edits(this.data.campaignId, image.id);
      if (
        this.view()?.image.id === image.id ||
        chain.some((e) => e.image?.id === this.view()?.image.id)
      ) {
        this.chain.set(chain);
      }
    } catch {
      this.chain.set([]);
    }
  }

  // ---- The result

  protected pickChain(imageId: string): void {
    const edit = this.chain().find((e) => e.image?.id === imageId);
    if (!edit?.image) {
      return;
    }
    this.view.set({
      image: edit.image,
      number: edit.number,
      caption: edit.prompt ? `Pedido: ${edit.prompt}` : '',
      texture: edit.image.showsWholeMap && this.mapId() !== null,
      wholeMap: edit.image.showsWholeMap,
    });
    this.useStage.set('idle');
    this.showStage.set('idle');
    this.showError.set(null);
  }

  protected shown(): boolean {
    const id = this.view()?.image.id;
    return id !== undefined && this.shownIds().has(id);
  }

  /** "Mostrar aos jogadores" of a picture that shows the whole map asks first, in place (RN-10). */
  protected askShow(): void {
    this.showError.set(null);
    this.showStage.set('asking');
    this.toTheQuestion();
  }

  protected cancelShow(): void {
    this.showStage.set('idle');
    this.showError.set(null);
  }

  protected async show(): Promise<void> {
    const image = this.view()?.image;
    if (!image || this.showBusy()) {
      return;
    }
    this.showBusy.set(true);
    this.showError.set(null);
    try {
      await this.shownApi.show(this.data.campaignId, image.id);
      this.shownIds.update((ids) => new Set([...ids, image.id]));
      this.showStage.set('idle');
    } catch (err) {
      this.showError.set(showImageIssue(err));
    } finally {
      this.showBusy.set(false);
    }
  }

  protected askUse(): void {
    this.useError.set(null);
    this.useStage.set('asking');
    this.toTheQuestion();
  }

  /** The question is at the end of the body, with its two buttons: it scrolls there. */
  private toTheQuestion(): void {
    afterNextRender(
      () => {
        const title = document.getElementById('gen-ask-t');
        title?.scrollIntoView?.({ block: 'nearest' });
        title?.focus({ preventScroll: true });
      },
      { injector: this.injector },
    );
  }

  protected cancelUse(): void {
    this.useStage.set('idle');
    this.useError.set(null);
  }

  protected async confirmUse(): Promise<void> {
    const image = this.view()?.image;
    if (!image || this.useStage() === 'busy') {
      return;
    }
    this.useStage.set('busy');
    this.useError.set(null);
    try {
      this.changedMap = await this.api.useAsMapImage(this.data.campaignId, image.id);
      this.useStage.set('done');
    } catch (err) {
      this.useError.set(useIssue(err));
      this.useStage.set('asking');
    }
  }

  protected submitEdit(): void {
    const image = this.view()?.image;
    const text = this.instruction().trim();
    if (!image || this.editWhy() !== '' || this.stage() !== 'result') {
      return;
    }
    const cid = this.data.campaignId;
    this.editError.set(null);
    this.notice.set(null);
    this.runFor.set('edit');
    this.runPrompt.set(text);
    const key = this.keyFor(JSON.stringify(['edit', image.id, text]));
    this.begin(
      (signal) => this.api.edit(cid, image.id, text, key, signal),
      (err) => {
        const issue = generateIssue(err, 'edit');
        this.dropKeyOnAnswer(err);
        if (issue.status) {
          this.status.set(issue.status);
        }
        this.editError.set(issue.text);
        this.stage.set('result');
        this.focusAfterStage();
      },
    );
  }

  // ---- Focus

  /** A new stage starts at the top, with the title focused (the screen reader reads where it is). */
  private focusAfterStage(field?: 'npc' | 'refs'): void {
    afterNextRender(
      () => {
        this.frame()?.scrollToTop();
        // The field the server named, when it did, otherwise the title.
        const target = field
          ? document.querySelector<HTMLElement>(`[data-field="${field}"]`)
          : null;
        (target ?? document.getElementById('gen-t'))?.focus({ preventScroll: !target });
      },
      { injector: this.injector },
    );
  }
}

function viewOf(res: Awaited<ReturnType<ImageGenClient['reference']>>): ReferenceView {
  return {
    previewUrl: pngDataUrl(res.preview, res.previewContentType || 'image/png'),
    seenSquares: res.seenSquares,
    totalSquares: res.totalSquares,
    playersSee: res.playersSeeSomething,
    rooms: res.roomsListed,
    gridColumns: res.gridColumns,
    gridRows: res.gridRows,
  };
}

/** Opens "Gerar imagem": a dialog from a tablet up and a bottom sheet with a fixed footer on a phone. The opener gets what was made when it closes. */
export function openImageGenerate(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: ImageGenerateData,
): Observable<GenerateOutcome | undefined> {
  return openSheet<ImageGenerateDialog, ImageGenerateData, GenerateOutcome>(
    dialog,
    bottomSheet,
    ImageGenerateDialog,
    {
      data,
      ariaLabel: 'Gerar imagem',
      labelledBy: 'gen-t',
      width: '580px',
      panelClass: 'mr-sheet-image',
    },
  );
}
