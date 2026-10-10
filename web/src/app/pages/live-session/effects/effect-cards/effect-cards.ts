import { ChangeDetectionStrategy, Component, computed, effect, input, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import {
  type EffectCardView,
  EXHAUSTION_LEVELS,
  exhaustionLines,
} from '../../../../core/effects/effects';

let nextId = 0;

/**
 * "Seus efeitos" on the live sheet (RN-22, board W7-Ea 2): under the hit points and the armor class, a card for each
 * effect the server lets this player read, with its name, whose it is ("De alguém que você não vê" when the caster is
 * an NPC they do not see), the small labels, the clock, the saving throw it asks again (the ability, never a DC) and,
 * in a box under the card, "O que isso muda". Exhaustion has its own card with the level and the lines it adds up.
 * Nothing is drawn without effects. Every card is a `role="group"` named by its title; the cards that come and go
 * are said once in a polite live region.
 */
@Component({
  selector: 'app-effect-cards',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    @if (level() > 0) {
      <section class="fx" [attr.aria-labelledby]="id + '-ex'">
        <h3 class="fx__heading" [id]="id + '-ex'">Exaustão</h3>
        <div class="fx__card mr-panel" role="group" [attr.aria-labelledby]="id + '-ex-lvl'">
          <p class="fx__title" [id]="id + '-ex-lvl'">Nível {{ level() }}</p>
          <p class="fx__from">Definida pelo mestre. Cada nível soma aos de baixo.</p>
          <ul class="fx__lines" aria-label="O que a exaustão muda">
            @for (line of lines(); track line) {
              <li>{{ line }}</li>
            }
          </ul>
        </div>
      </section>
    }
    @if (cards().length > 0) {
      <section class="fx" [attr.aria-labelledby]="id + '-h'">
        <h3 class="fx__heading" [id]="id + '-h'">Seus efeitos</h3>
        <ul class="fx__list">
          @for (c of cards(); track c.key; let i = $index) {
            <li class="fx__item">
              <div class="fx__card mr-panel" role="group" [attr.aria-labelledby]="id + '-t' + i">
                <p class="fx__title" [id]="id + '-t' + i">{{ c.name }}</p>
                @if (c.origin) {
                  <p class="fx__from">{{ c.origin }}</p>
                }
                @if (c.tags.length > 0) {
                  <ul class="fx__tags" aria-label="O que {{ c.name }} faz">
                    @for (t of c.tags; track t) {
                      <li class="fx__tag">{{ t }}</li>
                    }
                  </ul>
                }
                @if (c.clock) {
                  <p class="fx__row"><mat-icon aria-hidden="true">schedule</mat-icon><span>{{ c.clock }}</span></p>
                }
                @if (c.save) {
                  <p class="fx__row"><mat-icon aria-hidden="true">casino</mat-icon><span>{{ c.save }}</span></p>
                }
              </div>
              @if (c.changes.length > 0) {
                <div class="fx__changes" role="group" [attr.aria-labelledby]="id + '-c' + i">
                  <p class="fx__changes-title" [id]="id + '-c' + i">O que isso muda</p>
                  <ul>
                    @for (line of c.changes; track line) {
                      <li>{{ line }}</li>
                    }
                  </ul>
                </div>
              }
            </li>
          }
        </ul>
      </section>
    }
    <p class="mr-visually-hidden" role="status" aria-live="polite">{{ announcement() }}</p>
  `,
  styleUrl: './effect-cards.scss',
})
export class EffectCards {
  /** The effects this player may read, in the server's order. */
  readonly cards = input<readonly EffectCardView[]>([]);
  /** The exhaustion level, 0 to 6; 0 draws nothing. */
  readonly exhaustion = input(0);

  protected readonly id = `fx-${nextId++}`;
  protected readonly level = computed(() =>
    Math.max(0, Math.min(this.exhaustion(), EXHAUSTION_LEVELS.length)),
  );
  protected readonly lines = computed(() => exhaustionLines(this.level()));
  /** What came or went since the last time, for a screen reader. */
  protected readonly announcement = signal('');
  private previous: ReadonlyMap<string, string> | null = null;
  private previousLevel = 0;

  constructor() {
    effect(() => {
      const now = new Map(this.cards().map((c) => [c.key, c.name]));
      const level = this.level();
      const before = this.previous;
      const beforeLevel = this.previousLevel;
      this.previous = now;
      this.previousLevel = level;
      if (!before) {
        return;
      }
      const said: string[] = [];
      for (const [key, name] of now) {
        if (!before.has(key)) {
          said.push(`Novo efeito: ${name}.`);
        }
      }
      for (const [key, name] of before) {
        if (!now.has(key)) {
          said.push(`${name} acabou.`);
        }
      }
      if (level !== beforeLevel) {
        said.push(level === 0 ? 'A exaustão acabou.' : `Exaustão nível ${level}.`);
      }
      if (said.length > 0) {
        this.announcement.set(said.join(' '));
      }
    });
  }
}
