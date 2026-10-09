import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';

import type {
  Character,
  ChoiceGroup,
  FullSheet,
  PreviewChoicesResponse,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { describeCharacterError } from '../../core/characters/character-errors';
import {
  CharacterChoicesClient,
  type ChoiceAnswer,
} from '../../core/characters/character-choices-client';
import { describeCompletionError } from '../../core/characters/choice-completion-errors';
import {
  type ChoiceSelection,
  type ChoiceTextEdit,
  ChoiceGroups,
} from '../../shared/choice-groups/choice-groups';
import { applySelection, withChoiceText } from '../../shared/choice-groups/choice-picks';

type PageState =
  | { readonly status: 'loading' }
  | { readonly status: 'error'; readonly message: string }
  /** The sheet has no choice open (or no choices at all): nothing to complete. */
  | { readonly status: 'none'; readonly name: string }
  | { readonly status: 'ready' };

/** The quiet after a pick before the server is asked again. */
export const QUIET_MS = 150;

/**
 * "/campaigns/:id/characters/:characterId/choices" (PM-05): "Completar escolhas pendentes". A sheet that locked
 * with class or race choices still open (a half-elf's two +1, a Fighting Style the sheet never picked) is
 * completed here, by its owner or the master, without opening the editor. `PreviewChoices` says which choices
 * are open, what each option gives and why one cannot be taken; the page keeps the picks, asks again after
 * each one, and "Salvar escolhas" sends only the open choices to `CompleteCharacterChoices`. What is chosen
 * never changes again. Nothing here knows a rule.
 */
@Component({
  selector: 'app-character-choices',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ChoiceGroups, MatButtonModule, MatIconModule, MatProgressSpinnerModule, RouterLink],
  templateUrl: './character-choices.html',
  styleUrl: './character-choices.scss',
})
export class CharacterChoicesPage {
  private readonly client = inject(CharacterChoicesClient);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  private readonly groupsView = viewChild(ChoiceGroups);

  protected readonly state = signal<PageState>({ status: 'loading' });
  protected readonly campaignId = signal('');
  protected readonly characterId = signal('');
  protected readonly character = signal<Character | null>(null);
  protected readonly response = signal<PreviewChoicesResponse | null>(null);
  protected readonly saving = signal(false);
  protected readonly failure = signal('');
  /** A read after a pick failed: the picks are kept, the choices on screen may be one step behind. */
  protected readonly previewFailure = signal('');

  private fullSheet: FullSheet | null = null;
  private keys: string[] = [];
  private texts: Record<string, string> = {};
  /** The choices that were open when the page opened: they stay on the page after they are picked. */
  private readonly openKeys = signal<ReadonlySet<string>>(new Set());
  /** The picks of the open choices at the start, to tell what the player added. */
  private startPicked = new Map<string, number>();
  private timer: ReturnType<typeof setTimeout> | undefined;
  private seq = 0;

  protected readonly sheetLink = computed(() => [
    '/campaigns',
    this.campaignId(),
    'characters',
    this.characterId(),
  ]);

  /** The groups of the latest answer, only the choices that were open when the page opened. */
  protected readonly groups = computed<readonly ChoiceGroup[]>(() => {
    const open = this.openKeys();
    return (this.response()?.groups ?? [])
      .map((g) => ({ ...g, choices: g.choices.filter((c) => open.has(c.key)) }))
      .filter((g) => g.choices.length > 0) as ChoiceGroup[];
  });

  /** What "Salvar escolhas" sends: the open choices the player added to. */
  protected readonly answers = computed<ChoiceAnswer[]>(() => {
    const out: ChoiceAnswer[] = [];
    for (const g of this.groups()) {
      for (const c of g.choices) {
        const added = c.picked.length > (this.startPicked.get(c.key) ?? 0);
        const needsText = c.options.some((o) => c.picked.includes(o.key) && o.needsText);
        const texts = [c.texts[0] ?? '', c.texts[1] ?? ''];
        // A favored enemy that names humanoid races is whole only with both written.
        if (!added || (needsText && texts.some((t) => t.trim() === ''))) {
          continue;
        }
        out.push({ choiceKey: c.key, optionKeys: c.picked, texts: needsText ? texts : [] });
      }
    }
    return out;
  });

  protected readonly subtitle = computed(() => {
    const ch = this.character();
    const d = ch?.derived;
    if (!ch || !d) {
      return ch?.name ?? '';
    }
    const classes = d.classes.map((c) => `${c.namePt} ${c.level}`).join(' / ');
    return [ch.name, classes, d.subraceNamePt || d.raceNamePt].filter((p) => p !== '').join(' · ');
  });

  constructor() {
    const destroyRef = inject(DestroyRef);
    destroyRef.onDestroy(() => {
      clearTimeout(this.timer);
      this.seq++;
    });
    this.route.paramMap.subscribe((params) => {
      const campaignId = params.get('id');
      const characterId = params.get('characterId');
      if (campaignId && characterId) {
        this.campaignId.set(campaignId);
        this.characterId.set(characterId);
        void this.load(campaignId, characterId);
      }
    });
  }

  private async load(campaignId: string, characterId: string): Promise<void> {
    this.state.set({ status: 'loading' });
    this.failure.set('');
    try {
      const character = await this.client.character(campaignId, characterId);
      const content = character.sheet?.content;
      if (content?.case !== 'full') {
        this.character.set(character);
        this.state.set({ status: 'none', name: character.name });
        return;
      }
      this.character.set(character);
      this.fullSheet = content.value;
      this.keys = [...content.value.featureChoiceKeys];
      this.texts = { ...content.value.featureChoiceText };
      const res = await this.client.preview(
        campaignId,
        characterId,
        content.value,
        this.keys,
        this.texts,
      );
      const open = res.groups.flatMap((g) => g.choices.filter((c) => c.missing > 0));
      if (open.length === 0) {
        this.state.set({ status: 'none', name: character.name });
        return;
      }
      this.openKeys.set(new Set(open.map((c) => c.key)));
      this.startPicked = new Map(open.map((c) => [c.key, c.picked.length]));
      this.response.set(res);
      this.state.set({ status: 'ready' });
      afterNextRender(() => this.groupsView()?.focusFirstPending(), { injector: this.injector });
    } catch (err) {
      this.state.set({ status: 'error', message: describeCharacterError(err) });
    }
  }

  protected onSelected(sel: ChoiceSelection): void {
    this.keys = applySelection(this.keys, sel.choice, sel.optionKeys);
    this.failure.set('');
    this.ask();
  }

  protected onText(edit: ChoiceTextEdit): void {
    this.texts = withChoiceText(this.texts, edit.choice.key, edit.n, edit.text);
    this.failure.set('');
    this.ask();
  }

  /** Asks the server what the picks so far mean, after a short quiet; an older answer never overwrites a newer one. */
  private ask(): void {
    clearTimeout(this.timer);
    const seq = ++this.seq;
    this.timer = setTimeout(async () => {
      try {
        const res = await this.client.preview(
          this.campaignId(),
          this.characterId(),
          this.fullSheet!,
          this.keys,
          this.texts,
        );
        if (seq === this.seq) {
          this.previewFailure.set('');
          this.response.set(res);
        }
      } catch (err) {
        if (seq === this.seq) {
          this.previewFailure.set(describeCharacterError(err));
        }
      }
    }, QUIET_MS);
  }

  protected async save(): Promise<void> {
    const answers = this.answers();
    const ch = this.character();
    if (this.saving() || !ch) {
      return;
    }
    if (answers.length === 0) {
      this.failure.set('Faça pelo menos uma escolha antes de salvar.');
      this.focusFailure();
      return;
    }
    clearTimeout(this.timer);
    this.seq++;
    this.saving.set(true);
    this.failure.set('');
    try {
      await this.client.complete(this.campaignId(), this.characterId(), ch.revision, answers);
      await this.router.navigate(this.sheetLink());
    } catch (err) {
      this.failure.set(describeCompletionError(err));
      this.focusFailure();
    } finally {
      this.saving.set(false);
    }
  }

  private focusFailure(): void {
    afterNextRender(
      () => this.host.nativeElement.querySelector<HTMLElement>('.js-failure')?.focus(),
      { injector: this.injector },
    );
  }
}
