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
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  hitNames,
  questionKicker,
  questionTitle,
  questions,
} from '../../../../core/combat/hidden-reveal';
import { ofThe } from '../../../../core/combat/move-plan';
import { tieNumbers } from '../../../../core/format/text';

/** The master's answer to one question: reveal the hidden creatures the area hit, or keep them hidden. */
export interface RevealAnswer {
  readonly id: string;
  readonly reveal: boolean;
}

/**
 * "Bola de Fogo atingiu 2 criaturas escondidas" (PM-02c states 9, 9b and 9c): the master's question when a player's area
 * spell hit hidden creatures and the table asks. Each question is a card at the top of the order column, in the warning
 * surface: who cast, the hidden ones it hit by name, that the turn waits for him, and "Revelar" or "Manter escondidas".
 * The questions are answered in order: only the oldest one's buttons work, the others are dashed with "depois da
 * primeira". The cards come from `Encounter.pending_hidden_reveals`, so a reload brings them back as they were.
 *
 * The focus is not taken when a card arrives (an alert says it once). After an answer it goes to the next question's
 * "Revelar", or, with none left, the page puts it on the order (`answered`).
 */
@Component({
  selector: 'app-hidden-reveal-card',
  imports: [MatButtonModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @for (q of cards(); track q.id; let i = $index) {
      <section class="hr" role="group" [attr.aria-labelledby]="'hr-t-' + q.id">
        <mat-icon class="hr__icon" aria-hidden="true">visibility_off</mat-icon>
        <div class="hr__body">
          @if (q.kicker) {
            <p class="hr__kicker" [id]="'hr-k-' + q.id">{{ q.kicker }}</p>
          }
          <h2 class="hr__title" [id]="'hr-t-' + q.id">{{ q.title }}</h2>
          <p class="hr__text">
            @if (q.none) {
              {{ q.caster }} conjurou no ponto marcado. Nenhuma criatura escondida foi atingida; o turno só espera a sua resposta.
            } @else {
              {{ q.caster }} conjurou no ponto marcado. O efeito já valeu para
              @for (n of q.names; track $index) {<b>{{ n.name }}</b>{{ n.sep }}}.
              @if (cards().length === 1) {
                Os jogadores ainda não sabem delas.
              }
            }
            @if (i === 0) {
              <b>{{ q.waits }}</b>
            }
          </p>
          @if (q.none) {
            <div class="hr__pair">
              <button
                mat-flat-button
                type="button"
                class="hr__btn"
                data-reveal
                [class.hr__btn--off]="i > 0"
                [disabled]="busy() || i > 0"
                disabledInteractive
                [attr.aria-describedby]="i > 0 ? 'hr-k-' + q.id : null"
                (click)="i === 0 && !busy() && answer.emit({ id: q.id, reveal: false })"
              >
                <mat-icon aria-hidden="true">check</mat-icon>Sem escondidas
              </button>
            </div>
          } @else {
          <div class="hr__pair">
            <button
              mat-flat-button
              type="button"
              class="hr__btn"
              data-reveal
              [class.hr__btn--off]="i > 0"
              [disabled]="busy() || i > 0"
              disabledInteractive
              [attr.aria-describedby]="i > 0 ? 'hr-k-' + q.id : null"
              (click)="i === 0 && !busy() && answer.emit({ id: q.id, reveal: true })"
            >
              <mat-icon aria-hidden="true">visibility</mat-icon>Revelar
            </button>
            <button
              mat-stroked-button
              type="button"
              class="hr__btn"
              [class.hr__btn--off]="i > 0"
              [disabled]="busy() || i > 0"
              disabledInteractive
              [attr.aria-describedby]="i > 0 ? 'hr-k-' + q.id : null"
              (click)="i === 0 && !busy() && answer.emit({ id: q.id, reveal: false })"
            >
              <mat-icon aria-hidden="true">visibility_off</mat-icon>Manter escondidas
            </button>
          </div>
          }
          @if (i === 0) {
            <p class="hr__rule">A regra da mesa é “Perguntar a cada vez”.</p>
          }
        </div>
      </section>
    }
    <p class="mr-visually-hidden" role="alert">{{ news() }}</p>
  `,
  styleUrl: './hidden-reveal-card.scss',
})
export class HiddenRevealCard {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly encounter = input.required<Encounter>();
  /** The Portuguese name of each spell by key ("spell:fireball" → "Bola de Fogo"); a spell not read yet is "A magia". */
  readonly spellNames = input<ReadonlyMap<string, string>>(new Map());
  readonly busy = input(false);

  readonly answer = output<RevealAnswer>();
  /** The last question was answered while the focus was on a card: the page moves it on. */
  readonly answered = output<void>();

  /** The title of a question that just arrived, said once. */
  protected readonly news = signal('');

  protected readonly cards = computed(() => {
    const e = this.encounter();
    const all = questions(e);
    return all.map((q, i) => {
      const caster = e.combatants.find((c) => c.id === q.casterId)?.label ?? 'Um jogador';
      const names = hitNames(e, q);
      return {
        id: q.id,
        kicker: questionKicker(i, all.length),
        title: questionTitle(this.spellNames().get(q.spellKey) ?? 'A magia', names.length),
        caster: tieNumbers(caster),
        names: names.map((name, k) => ({
          name: tieNumbers(name),
          sep: k === names.length - 1 ? '' : k === names.length - 2 ? ' e ' : ', ',
        })),
        none: names.length === 0,
        waits: `O turno ${ofThe([caster])} espera por você.`,
      };
    });
  });

  constructor() {
    let seen: readonly string[] = [];
    effect(() => {
      const ids = this.cards().map((c) => c.id);
      const titles = new Map(this.cards().map((c) => [c.id, c.title]));
      untracked(() => {
        const fresh = ids.filter((id) => !seen.includes(id));
        const gone = seen.some((id) => !ids.includes(id));
        if (fresh.length > 0) {
          this.news.set(fresh.map((id) => titles.get(id)).join('. '));
        }
        if (gone) {
          this.moveFocus(ids.length > 0);
        }
        seen = ids;
      });
    });
  }

  /** After an answer: the next question's "Revelar", or the page's next stop when none is left. Only when the focus was on a
   * card (or fell to the page with the card that held it): an answer from another tab never steals it. */
  private moveFocus(more: boolean): void {
    const active = document.activeElement;
    const ours = !active || active === document.body || this.host.nativeElement.contains(active);
    if (!ours) {
      return;
    }
    afterNextRender(
      () => {
        if (more) {
          this.host.nativeElement.querySelector<HTMLButtonElement>('[data-reveal]')?.focus();
        } else {
          this.answered.emit();
        }
      },
      { injector: this.injector },
    );
  }
}
