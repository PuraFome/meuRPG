import { NgTemplateOutlet } from '@angular/common';
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
  signal,
  untracked,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Encounter, ReactionWindow } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  CombatClient,
  type ConcentrationAnswer,
  type ReactionAnswer,
} from '../../../../core/combat/combat-client';
import { combatErrorMessage } from '../../../../core/combat/combat-errors';
import type { CombatState } from '../../../../core/combat/combat-state';
import { ofThe } from '../../../../core/combat/move-plan';
import {
  type QueueRow,
  type ReactionCard,
  type SlotOption,
  reactionCards,
} from '../../../../core/combat/reaction-master';
import { ActionKey } from '../../../../core/connect/idempotency';
import { autoPassText } from '../../../../core/combat/reaction-autopass';
import { rollText } from '../../../../core/combat/combat-dice';
import { ExtraDiceState } from '../../../../core/effects/extra-dice-state';
import { ExtraDice } from '../../effects/extra-dice/extra-dice';
import { RollPicker } from '../roll-picker/roll-picker';

let nextId = 0;

/**
 * The master's side of the reaction windows (PM-04b 8, PM-04c 9 and 12b, PM-04d 2): every open window of the combat,
 * as the server lists them, in the order they are answered. The windows of one action share a card ("Reações a uma
 * magia", "Reações a um ataque"), each row with its reactor, the "Jogador" or "NPC" tag and "Usar ..." with "Deixar
 * passar"; the one to answer now is first, the others wait grey ("Depois do Mago 1"). An NPC alone is a card of its
 * own ("Esperando a sua reação: ..."). A player's window reads "Respondendo no celular", and the master may answer
 * for them ("pelo jogador"). The second steps (the aggressor's saving throw of Repreensão Infernal, a concentration
 * save), and the "Sem reação" check of the table rule "Sempre", are cards too. Every answer carries its own key, made
 * once, so a repeated tap never answers twice. The numbers on these cards are the master's only (RN-20).
 */
@Component({
  selector: 'app-reaction-queue',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ExtraDice, MatButtonModule, MatIconModule, NgTemplateOutlet, RollPicker],
  template: `
    @for (card of cards(); track card.id) {
      @switch (card.type) {
        @case ('queue') {
          <section class="card" [attr.aria-labelledby]="uid + card.id">
            <h2 class="card__title" [id]="uid + card.id">{{ card.title }}</h2>
            <p class="card__sub">{{ card.subtitle }}</p>
            <ul class="rows">
              @for (row of card.rows; track row.window.id) {
                <li class="row" [class.row--wait]="!row.answerNow">
                  <ng-container *ngTemplateOutlet="rowTpl; context: { row: row }" />
                </li>
              }
            </ul>
          </section>
        }
        @case ('single') {
          <section class="card card--single" [attr.aria-label]="card.row.window.reactorLabel">
            <div class="warn">
              <mat-icon aria-hidden="true">shield</mat-icon>
              <p>{{ card.text }}</p>
            </div>
            <ng-container *ngTemplateOutlet="slotsTpl; context: { row: card.row }" />
            <div class="pair">
              <button
                mat-flat-button
                type="button"
                class="btn"
                data-initial
                [disabled]="busy() || !card.row.answerNow"
                disabledInteractive
                (click)="use(card.row)"
              >
                {{ singleUseLabel(card.row) }}
              </button>
              <button
                mat-stroked-button
                type="button"
                class="btn"
                [disabled]="busy() || !card.row.answerNow"
                disabledInteractive
                (click)="pass(card.row.window)"
              >
                Deixar passar
              </button>
            </div>
          </section>
        }
        @case ('check') {
          <section class="card card--single" [attr.aria-label]="'Sem reação: ' + card.summary">
            <h2 class="card__title">{{ card.summary }}</h2>
            @if (card.canReact) {
              <p class="card__text">Um inimigo pode usar uma reação para isto. Responda com um toque.</p>
            } @else {
              <p class="card__text">
                Nenhum inimigo tem uma reação para isto. Responda com um toque; os jogadores veem só “Esperando o mestre”.
              </p>
            }
            <div class="pair">
              <button
                mat-flat-button
                type="button"
                class="btn"
                data-initial
                [disabled]="busy()"
                (click)="pass(card.window)"
              >
                Sem reação
              </button>
            </div>
          </section>
        }
        @case ('rebukeSave') {
          <section class="card card--single" [attr.aria-labelledby]="uid + card.id">
            <h2 class="card__title" [id]="uid + card.id">{{ rebukeTitle(card.window) }}</h2>
            <p class="card__text">{{ rebukeText(card.window) }}</p>
            <p class="card__sub">{{ rebukeBonus(card.window) }}</p>
            <div class="warn">
              <mat-icon aria-hidden="true">info</mat-icon>
              <p>
                <strong>O teste do agressor é do mestre.</strong> O jogador lê só “falhou” ou “passou”.
              </p>
            </div>
            <app-roll-picker
              [outlined]="true"
              [min]="1"
              [max]="20"
              [modifier]="rebukeModifier(card.window)"
              [label]="'Role 1d20 para o teste de ' + rebukeAggressor(card.window)"
              hint="Role o seu dado e digite o número que saiu (1 a 20)."
              totalNote="Teste de Destreza"
              [appLabel]="'Rolar o teste ' + ofTheLabel(rebukeAggressor(card.window))"
              [busy]="busy()"
              (app)="answer(card.window, { use: true, die: { inApp: true } })"
              (typed)="answer(card.window, { use: true, die: { typed: $event } })"
            />
          </section>
        }
        @case ('concentration') {
          <section class="card card--single" [attr.aria-labelledby]="uid + card.id">
            <h2 class="card__title" [id]="uid + card.id">{{ concentrationTitle(card.window) }}</h2>
            @if (card.player) {
              <p class="card__text">
                {{ concentrationLead(card.window) }} O ataque que deu o dano espera o teste (CD {{ concentrationDc(card.window) }}).
              </p>
              <p class="card__sub" role="status">
                Esperando o teste de Constituição de {{ card.window.reactorLabel }}. O jogador está respondendo no celular.
              </p>
              <div class="pair">
                <button
                  mat-flat-button
                  type="button"
                  class="btn"
                  data-initial
                  [disabled]="busy()"
                  (click)="concentrate(card.window, { kind: 'app' })"
                >
                  Rolar o teste por {{ card.window.reactorLabel }}
                </button>
                <button
                  mat-stroked-button
                  type="button"
                  class="btn"
                  [disabled]="busy()"
                  (click)="concentrate(card.window, { kind: 'keep' })"
                >
                  Manter a concentração por {{ card.window.reactorLabel }}
                </button>
              </div>
            } @else {
              <p class="card__text">{{ concentrationLead(card.window) }} O ataque que deu o dano espera este teste.</p>
              <p class="card__sub">{{ concentrationRule(card.window) }}</p>
              <div class="warn">
                <mat-icon aria-hidden="true">info</mat-icon>
                <p>
                  <strong>O teste é seu.</strong> Rolar no app usa d20 {{ bonusWord(card.window) }}; você pode decidir o resultado à mão.
                </p>
              </div>
              @if (extraWindow() === card.window.id && extra.fields().length > 0) {
                <app-extra-dice [fields]="extra.fields()" [(faces)]="extra.faces" />
              }
              <app-roll-picker
                [outlined]="true"
                [min]="1"
                [max]="20"
                [modifier]="concentrationBonus(card.window)"
                [label]="'Role 1d20 para o teste de Constituição ' + ofTheLabel(card.window.reactorLabel)"
                hint="Role o seu dado e digite o número que saiu (1 a 20)."
                totalNote="Teste de Constituição"
                [appLabel]="'Rolar o teste ' + ofTheLabel(card.window.reactorLabel)"
                [busy]="busy()"
                (app)="concentrate(card.window, { kind: 'app' })"
                (typed)="concentrate(card.window, { kind: 'typed', face: $event })"
              />
              <button
                mat-stroked-button
                type="button"
                class="btn btn--one"
                [disabled]="busy()"
                (click)="concentrate(card.window, { kind: 'keep' })"
              >
                Manter a concentração
              </button>
            }
          </section>
        }
      }
    }
    <span class="mr-visually-hidden" role="status" aria-live="polite">{{ said() }}</span>
    @if (error()) {
      <div class="mr-notice mr-notice--danger" role="alert">
        <mat-icon aria-hidden="true">error</mat-icon>
        <p>{{ error() }}</p>
      </div>
    }

    <ng-template #rowTpl let-row="row">
      <span class="avatar" aria-hidden="true">{{ row.label.charAt(0) }}</span>
      <div class="row__main">
        <p class="row__name">
          <b>{{ row.label }}</b>
          <span class="tag" [class.tag--player]="row.isPlayer">{{ row.tag }}</span>
        </p>
        <p class="row__desc">{{ row.description }}</p>
        @if (row.status) {
          <p class="row__status" [id]="uid + row.window.id + '-why'">{{ row.status }}</p>
        }
        @if (autoLeft()[row.window.id] !== undefined) {
          <p class="row__status">{{ autoText(autoLeft()[row.window.id]) }}</p>
        }
        <ng-container *ngTemplateOutlet="slotsTpl; context: { row: row }" />
        <div class="btns">
          <button
            mat-stroked-button
            type="button"
            class="btn"
            [class.btn--off]="!row.answerNow"
            [attr.data-initial]="row.answerNow ? '' : null"
            [disabled]="busy() || !row.answerNow"
            disabledInteractive
            [attr.aria-describedby]="row.status ? uid + row.window.id + '-why' : null"
            (click)="use(row)"
          >
            {{ row.useLabel }}
          </button>
          <button
            mat-stroked-button
            type="button"
            class="btn"
            [class.btn--off]="!row.answerNow"
            [disabled]="busy() || !row.answerNow"
            disabledInteractive
            [attr.aria-describedby]="row.status ? uid + row.window.id + '-why' : null"
            (click)="pass(row.window)"
          >
            {{ row.passLabel }}
          </button>
        </div>
      </div>
    </ng-template>

    <ng-template #slotsTpl let-row="row">
      @if (row.slots.length > 0) {
        <fieldset class="slots" [disabled]="!row.answerNow">
          <legend class="slots__cap">Espaço de magia</legend>
          @for (o of row.slots; track o.title) {
            <label class="slots__row">
              <input
                type="radio"
                class="mr-visually-hidden"
                [name]="uid + row.window.id"
                [checked]="isSlot(row.window.id, o)"
                (change)="pickSlot(row.window.id, o)"
              />
              <span class="slots__dot" aria-hidden="true"></span>
              <span><b>{{ o.title }}</b> · {{ o.effect }}</span>
            </label>
          }
        </fieldset>
      }
    </ng-template>
  `,
  styleUrl: './reaction-queue.scss',
})
export class ReactionQueue {
  private readonly api = inject(CombatClient);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly encounter = input.required<Encounter>();
  readonly campaignId = input.required<string>();
  readonly state = input.required<CombatState>();
  /** The seconds left before a player's optional reaction passes by itself, by window id (the master's screen sends the pass). */
  readonly autoLeft = input<Readonly<Record<string, number>>>({});

  protected readonly uid = `rq-${nextId++}-`;
  protected readonly busy = signal(false);
  /** The d4 an effect adds to a typed concentration save, asked for the window whose roll the server refused. */
  protected readonly extra = new ExtraDiceState();
  protected readonly extraWindow = signal('');
  protected readonly error = signal('');
  /** What the last answer did, for the live region. */
  protected readonly said = signal('');
  protected readonly ofTheLabel = (label: string): string => ofThe([label]);
  /** The slot the master picked for an NPC's Counterspell, by window. */
  private readonly picks = signal<Readonly<Record<string, SlotOption>>>({});
  private readonly keys = new ActionKey();

  protected readonly cards = computed<readonly ReactionCard[]>(() =>
    reactionCards(this.encounter()),
  );

  constructor() {
    // The focus opens on the first button of the first window to answer, when it is a new one.
    let first = '';
    effect(() => {
      const id = this.cards()[0]?.id ?? '';
      if (id !== first) {
        first = id;
        if (id) {
          untracked(() =>
            afterNextRender(
              () =>
                this.host.nativeElement
                  .querySelector<HTMLElement>('[data-initial]:not([disabled])')
                  ?.focus(),
              { injector: this.injector },
            ),
          );
        }
      }
    });
  }

  protected readonly autoText = autoPassText;

  protected singleUseLabel(row: QueueRow): string {
    const slot = this.slotOf(row);
    return slot ? `${row.useLabel} (${slot.title})` : row.useLabel;
  }

  private slotOf(row: QueueRow): SlotOption | null {
    return row.slots.length === 0 ? null : (this.picks()[row.window.id] ?? row.slots[0]);
  }

  protected isSlot(windowId: string, option: SlotOption): boolean {
    const row = this.cards()
      .flatMap((c) => (c.type === 'queue' ? c.rows : c.type === 'single' ? [c.row] : []))
      .find((r) => r.window.id === windowId);
    const chosen = row ? this.slotOf(row) : null;
    return (
      !!chosen && chosen.slot.level === option.slot.level && chosen.slot.pact === option.slot.pact
    );
  }

  protected pickSlot(windowId: string, option: SlotOption): void {
    this.picks.update((p) => ({ ...p, [windowId]: option }));
  }

  /** "Usar ... pelo ...": the slot an NPC spends is the chosen one (Counterspell) or the lowest free (Escudo). */
  protected use(row: QueueRow): Promise<void> {
    const w = row.window;
    const chosen = this.slotOf(row);
    if (chosen) {
      return this.answer(w, {
        use: true,
        slot: { level: chosen.slot.level, pact: chosen.slot.pact },
      });
    }
    const shield = w.prompt.case === 'shield' ? w.prompt.value.slots : [];
    const slot = [...shield].filter((s) => s.free > 0).sort((a, b) => a.level - b.level)[0];
    return this.answer(w, {
      use: true,
      ...(slot ? { slot: { level: slot.level, pact: slot.pact } } : {}),
    });
  }

  protected pass(w: ReactionWindow): Promise<void> {
    return this.answer(w, { use: false });
  }

  protected async answer(w: ReactionWindow, answer: ReactionAnswer): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const res = await this.api.answerReaction(
        this.campaignId(),
        this.encounter().id,
        w.id,
        answer,
        this.keys.keyFor({ window: w.id, answer }),
      );
      this.state().apply(res.encounter);
      this.said.set(this.sayAnswer(w, answer, res.result?.result));
    } catch (err) {
      this.error.set(combatErrorMessage(err, 'responder a reação'));
    } finally {
      this.busy.set(false);
    }
  }

  protected async concentrate(w: ReactionWindow, how: ConcentrationAnswer): Promise<void> {
    if (this.busy()) {
      return;
    }
    const extra = this.extraWindow() === w.id ? this.extra.take(how.kind === 'typed') : [];
    if (extra === null) {
      this.error.set(this.extra.missingText());
      return;
    }
    const sent: ConcentrationAnswer =
      how.kind === 'typed' && extra.length > 0 ? { ...how, extra } : how;
    this.busy.set(true);
    this.error.set('');
    try {
      const res = await this.api.resolveConcentrationSave(
        this.campaignId(),
        this.encounter().id,
        w.id,
        sent,
        this.keys.keyFor({ window: w.id, how: sent }),
      );
      this.state().apply(res.encounter);
      const r = res.result;
      this.said.set(
        r
          ? `${r.save ? `${rollText(r.save)} ` : ''}contra CD ${r.dc}: ${w.reactorLabel} ${r.kept ? 'manteve' : 'perdeu'} a concentração${r.spellNamePt ? ` em ${r.spellNamePt}` : ''}.`
          : how.kind === 'keep'
            ? `Concentração mantida ${ofThe([w.reactorLabel])}.`
            : 'Respondido.',
      );
    } catch (err) {
      const more = this.extra.fromRefusal(err);
      if (more) {
        this.extraWindow.set(w.id);
      }
      this.error.set(more || combatErrorMessage(err, 'resolver o teste de concentração'));
    } finally {
      this.busy.set(false);
    }
  }

  private sayAnswer(
    w: ReactionWindow,
    answer: ReactionAnswer,
    result: { readonly case?: string; readonly value?: unknown } | undefined,
  ): string {
    if (!answer.use) {
      return `Deixou passar ${ofThe([w.reactorLabel])}.`;
    }
    if (result?.case === 'shield') {
      const stopped = (result.value as { stopped: boolean }).stopped;
      return stopped
        ? 'Escudo Arcano usado: o ataque errou.'
        : 'Escudo Arcano usado: o ataque ainda acerta.';
    }
    return 'Reação usada.';
  }

  // ---- the second steps ----

  protected rebukeTitle(w: ReactionWindow): string {
    return `Repreensão Infernal ${w.reactorIsPlayer ? `de ${w.reactorLabel}` : ofThe([w.reactorLabel])}`;
  }

  protected rebukeAggressor(w: ReactionWindow): string {
    return w.prompt.case === 'hellishRebukeSave' ? w.prompt.value.aggressorLabel : '';
  }

  protected rebukeText(w: ReactionWindow): string {
    const p = w.prompt.case === 'hellishRebukeSave' ? w.prompt.value : null;
    return p
      ? `${p.aggressorLabel} sofre ${p.diceCount}d10 de fogo, ou metade se passar no teste de Destreza contra CD ${p.saveDc}.`
      : '';
  }

  protected rebukeBonus(w: ReactionWindow): string {
    const p = w.prompt.case === 'hellishRebukeSave' ? w.prompt.value : null;
    return p
      ? `Destreza ${ofThe([p.aggressorLabel])}: ${p.bonusKnown ? signed(p.saveBonus) : 'desconhecida'} · o teste é seu`
      : '';
  }

  protected rebukeModifier(w: ReactionWindow): number {
    const p = w.prompt.case === 'hellishRebukeSave' ? w.prompt.value : null;
    return p?.bonusKnown ? p.saveBonus : 0;
  }

  protected concentrationTitle(w: ReactionWindow): string {
    const damage = w.trigger?.damageTaken ?? 0;
    return `${w.reactorLabel} sofreu ${damage} de dano`;
  }

  private concentrationSpell(w: ReactionWindow): string {
    return w.prompt.case === 'concentrationSave'
      ? w.prompt.value.spellNamePt
      : (w.trigger?.concentrationSpellKey ?? '');
  }

  protected concentrationDc(w: ReactionWindow): number {
    return w.prompt.case === 'concentrationSave' ? w.prompt.value.dc : 10;
  }

  protected concentrationLead(w: ReactionWindow): string {
    return `${w.reactorLabel} concentra em ${this.concentrationSpell(w)}.`;
  }

  protected concentrationRule(w: ReactionWindow): string {
    const damage = w.trigger?.damageTaken ?? 0;
    const p = w.prompt.case === 'concentrationSave' ? w.prompt.value : null;
    const bonus = p?.bonusKnown
      ? ` · Constituição ${ofThe([w.reactorLabel])}: ${signed(p.saveBonus)}`
      : '';
    return `Teste de resistência de Constituição contra CD ${this.concentrationDc(w)} (metade de ${damage} seria ${Math.floor(damage / 2)}; o mínimo é 10)${bonus}`;
  }

  protected concentrationBonus(w: ReactionWindow): number {
    return w.prompt.case === 'concentrationSave' && w.prompt.value.bonusKnown
      ? w.prompt.value.saveBonus
      : 0;
  }

  protected bonusWord(w: ReactionWindow): string {
    const bonus = this.concentrationBonus(w);
    return `${bonus < 0 ? '−' : '+'} ${Math.abs(bonus)}`;
  }
}

function signed(n: number): string {
  return n < 0 ? `−${Math.abs(n)}` : `+${n}`;
}
