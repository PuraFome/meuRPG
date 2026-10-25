import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import {
  type CatalogEffect,
  type CharacterEffect,
  EffectAudience,
  EffectEndScope,
} from '../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { ActionKey } from '../../../core/connect/idempotency';
import { joinDots } from '../../../core/format/text';
import { focusWithRing } from '../../../core/creatures/focus-ring';
import { EffectsClient } from '../../../core/effects/effects-client';
import { effectsErrorMessage } from '../../../core/effects/effects-errors';
import {
  MAX_SECONDS,
  TIME_PRESETS,
  advanceDoneText,
  elapsedText,
  validSeconds,
} from '../../../core/effects/effects-text';
import { TextField } from '../../../shared/form-fields/text-field';
import { openSheet } from '../../../shared/sheet/sheet-host';
import type { VitalsVm } from '../../../core/play/live-session.types';
import {
  AddCharacterEffectDialog,
  type AddCharacterEffectData,
  type AddCharacterEffectResult,
} from './add-character-effect-dialog';
import {
  ExhaustionDialog,
  type ExhaustionData,
  type ExhaustionOption,
  type ExhaustionResult,
} from './exhaustion-dialog';

/** A card of the panel with what the template reads. */
interface Card {
  readonly effect: CharacterEffect;
  /** "Restam 7 h 40 min · Os jogadores veem: o dono", and the tags and conditions after it. */
  readonly line: string;
  readonly questionTitle: string;
}

/**
 * "Efeitos em jogo" outside a combat (W7-E): the effects on the characters, as the master reads them (all of them, with
 * what the players see), each with "Encerrar" (a concentration asks first, in place), the exhaustion of each character,
 * "Dar efeito" (an effect of the catalog on characters, given between fights) and "Passar o tempo": outside a combat the app does not count time by itself, so the master says how much went by (1
 * second to 24 hours, with presets of a round, a minute, ten minutes and an hour) and the effects whose time runs out
 * end. Read again whenever the page says something changed (`tick`) and after its own writes.
 */
@Component({
  selector: 'app-character-effects-panel',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule, TextField],
  templateUrl: './character-effects-panel.html',
  styleUrl: './character-effects-panel.scss',
})
export class CharacterEffectsPanel {
  readonly campaignId = input.required<string>();
  /** The party as the page has it: who may have their exhaustion changed, and at which level. */
  readonly party = input<readonly VitalsVm[]>([]);
  /** Goes up when something that may change the effects happened (a spell cast, a rest). */
  readonly tick = input(0);
  /** The conditions the effects put on each character, for the party's rows ("Envenenada"). */
  readonly conditions = output<ReadonlyMap<string, readonly string[]>>();

  private readonly api = inject(EffectsClient);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  protected readonly effects = signal<readonly CharacterEffect[] | null>(null);
  protected readonly loadError = signal('');
  protected readonly error = signal('');
  protected readonly done = signal('');
  protected readonly asking = signal<string | null>(null);
  protected readonly busy = signal(false);
  /** The catalog is being read for "Dar efeito". */
  protected readonly opening = signal(false);

  protected readonly presets = TIME_PRESETS;
  protected readonly maxSeconds = MAX_SECONDS;
  protected readonly seconds = signal('60');
  protected readonly advancing = signal(false);
  protected readonly advanceError = signal('');
  protected readonly advanced = signal('');

  /** The levels the master just set, until the page's own numbers catch up. */
  private readonly levels = signal<Readonly<Record<string, number>>>({});

  private readonly endKey = new ActionKey();
  private readonly timeKey = new ActionKey();
  private reads = 0;

  protected readonly cards = computed<readonly Card[]>(() =>
    (this.effects() ?? []).map((effect) => ({
      effect,
      line: joinDots([
        effect.durationTextPt,
        !effect.playerVisible
          ? 'Os jogadores não veem'
          : effect.audience === EffectAudience.OWNER
            ? 'Os jogadores veem: o dono'
            : 'Os jogadores veem: todos',
        ...effect.tagsPt,
        ...effect.conditionNamesPt,
      ]),
      questionTitle: `Encerrar ${effect.sourceNamePt} em ${effect.characterName}?`,
    })),
  );
  protected readonly countText = computed(() => String(this.cards().length));
  /** "Outro tempo": the seconds field and its button, for a time the four buttons do not have. */
  protected readonly custom = signal(false);

  protected readonly secondsNumber = computed(() => Number(this.seconds().trim() || NaN));
  protected readonly secondsIssues = computed(() =>
    validSeconds(this.secondsNumber()) ? [] : ['Digite de 1 a 86.400 segundos.'],
  );
  protected readonly preview = computed(() =>
    validSeconds(this.secondsNumber()) ? `Vai passar ${elapsedText(this.secondsNumber())}.` : '',
  );

  constructor() {
    effect(() => {
      this.tick();
      untracked(() => void this.reload());
    });
    effect(() => {
      const byCharacter = new Map<string, string[]>();
      for (const e of this.effects() ?? []) {
        for (const name of e.conditionNamesPt) {
          const list = byCharacter.get(e.characterId) ?? [];
          if (!list.includes(name)) {
            list.push(name);
          }
          byCharacter.set(e.characterId, list);
        }
      }
      this.conditions.emit(byCharacter);
    });
    // The page's own numbers are newer than what the master set a moment ago.
    effect(() => {
      this.party();
      untracked(() => this.levels.set({}));
    });
  }

  async reload(): Promise<void> {
    const seq = ++this.reads;
    try {
      const res = await this.api.listCharacterEffects(this.campaignId());
      if (seq === this.reads) {
        this.effects.set(res);
        this.loadError.set('');
        if (this.asking() !== null && !res.some((e) => e.id === this.asking())) {
          this.asking.set(null);
        }
      }
    } catch (err) {
      if (seq === this.reads) {
        this.loadError.set(effectsErrorMessage(err, 'ler os efeitos', 'character'));
      }
    }
  }

  protected end(card: Card): void {
    if (this.busy()) {
      return;
    }
    if (card.effect.concentration) {
      this.asking.set(card.effect.id);
      afterNextRender(
        () =>
          focusWithRing(
            this.host.nativeElement.querySelector<HTMLElement>(
              `[data-ask="${card.effect.id}"] [data-initial-focus]`,
            ),
          ),
        { injector: this.injector },
      );
      return;
    }
    void this.finishEnd(card.effect, EffectEndScope.THIS);
  }

  protected confirmEnd(card: Card): Promise<void> {
    return this.finishEnd(card.effect, EffectEndScope.CONCENTRATION_GROUP);
  }

  protected cancelAsk(card: Card): void {
    this.asking.set(null);
    focusWithRing(
      this.host.nativeElement.querySelector<HTMLElement>(`[data-end="${card.effect.id}"]`),
    );
  }

  private async finishEnd(target: CharacterEffect, scope: EffectEndScope): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      // An empty encounter id: the effect is on a character outside a combat.
      await this.api.end(
        this.campaignId(),
        '',
        target.id,
        scope,
        this.endKey.keyFor([target.id, scope]),
      );
      this.endKey.renew();
      this.asking.set(null);
      this.done.set(`Efeito encerrado: ${target.sourceNamePt} em ${target.characterName}.`);
      await this.reload();
      this.focusTitle();
    } catch (err) {
      this.error.set(effectsErrorMessage(err, 'encerrar o efeito', 'character'));
    } finally {
      this.busy.set(false);
    }
  }

  private focusTitle(): void {
    afterNextRender(
      () =>
        focusWithRing(this.host.nativeElement.querySelector<HTMLElement>('[data-effects-title]')),
      { injector: this.injector },
    );
  }

  protected openExhaustion(): void {
    const levels = this.levels();
    const options: ExhaustionOption[] = this.party().map((v) => ({
      key: v.characterId,
      label: v.name,
      level: levels[v.characterId] ?? v.exhaustionLevel ?? 0,
      subject: { characterId: v.characterId },
    }));
    if (options.length === 0) {
      return;
    }
    const data: ExhaustionData = { campaignId: this.campaignId(), options, state: null };
    openSheet<ExhaustionDialog, ExhaustionData, ExhaustionResult>(
      this.dialog,
      this.bottomSheet,
      ExhaustionDialog,
      {
        data,
        ariaLabel: 'Exaustão',
        labelledBy: 'exhaustion-t',
        width: '600px',
        tall: true,
      },
    ).subscribe((result) => {
      if (result) {
        this.levels.update((all) => ({ ...all, [result.key]: result.level }));
        this.done.set(
          `Exaustão de ${options.find((o) => o.key === result.key)?.label}: nível ${result.level}.`,
        );
      }
    });
  }

  /** "Dar efeito": reads the catalog, then asks which effect goes on which characters. */
  protected async openGive(): Promise<void> {
    const characters = this.party().map((v) => ({ id: v.characterId, name: v.name }));
    if (this.opening() || characters.length === 0) {
      return;
    }
    this.opening.set(true);
    this.error.set('');
    let catalog: CatalogEffect[];
    try {
      catalog = await this.api.catalog(this.campaignId());
    } catch (err) {
      this.error.set(effectsErrorMessage(err, 'ler o catálogo de efeitos', 'character'));
      return;
    } finally {
      this.opening.set(false);
    }
    const data: AddCharacterEffectData = { campaignId: this.campaignId(), characters, catalog };
    openSheet<AddCharacterEffectDialog, AddCharacterEffectData, AddCharacterEffectResult | null>(
      this.dialog,
      this.bottomSheet,
      AddCharacterEffectDialog,
      {
        data,
        ariaLabel: 'Dar um efeito',
        labelledBy: 'give-effect-t',
        width: '600px',
        tall: true,
      },
    ).subscribe((result) => {
      if (result) {
        this.done.set(`${result.effectName} dado a ${result.characterNames.join(', ')}.`);
        void this.reload();
      }
    });
  }

  /** A button of "Passar o tempo": that much time went by. */
  protected advanceBy(seconds: number): Promise<void> {
    this.seconds.set(String(seconds));
    return this.advance();
  }

  /** "Passar o tempo": says how much time went by; the effects whose time ran out end. */
  protected async advance(): Promise<void> {
    const seconds = this.secondsNumber();
    if (this.advancing() || !validSeconds(seconds)) {
      return;
    }
    this.advancing.set(true);
    this.advanceError.set('');
    this.advanced.set('');
    try {
      const ended = await this.api.advanceTime(
        this.campaignId(),
        seconds,
        this.timeKey.keyFor([seconds]),
      );
      this.timeKey.renew();
      this.advanced.set(advanceDoneText(seconds, ended));
      await this.reload();
    } catch (err) {
      this.advanceError.set(effectsErrorMessage(err, 'passar o tempo', 'character'));
    } finally {
      this.advancing.set(false);
    }
  }
}
