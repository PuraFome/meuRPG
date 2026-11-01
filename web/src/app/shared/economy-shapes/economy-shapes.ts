import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { cannotAct } from '../../core/combat/conditions';
import { type ReactionRecord, reactionUsedText } from '../../core/combat/reaction-record';

/** The four things a turn has: the Action (a circle), the Bonus action (a triangle), the Reaction (a square) and the movement (a walker). */
export type EconomyKind = 'action' | 'bonus' | 'reaction' | 'move';

/** One shape of the economy: filled while it is there, an outline with a slash once it is spent. */
export interface EconomyShape {
  readonly kind: EconomyKind;
  /** The short word beside the shape ("Ação", "Bônus", "Reação", "9 m"). */
  readonly text: string;
  /** The words a screen reader reads ("Ação disponível", "Ação bônus usada"). */
  readonly name: string;
  readonly spent: boolean;
}

/** The part of a combatant the shapes read. */
export interface EconomyFacts extends ReactionRecord {
  readonly actionUsed: boolean;
  readonly bonusActionUsed: boolean;
  readonly reactionUsed: boolean;
  readonly conditions?: readonly string[];
}

/** What the shapes need besides the combatant: the movement left as the screen words it, and Extra Attack's count. */
export interface EconomyOptions {
  /** "9 m": the metres left, drawn as the fourth shape; `null` leaves the walker out (no map, or the card that shows it elsewhere). */
  readonly moveText?: string | null;
  /** Extra Attack: the attacks still left once the first spent the Action. */
  readonly attacksLeft?: number;
  readonly attacksPerAction?: number;
}

/** "Reação usada (Escudo Arcano, rodada 2)" while the reaction is spent with a record; otherwise "Reação" and the word. */
function reactionName(c: EconomyFacts, word: string, cannot: boolean): string {
  return c.reactionUsed && !cannot ? reactionUsedText(c) : `Reação ${word}`;
}

/**
 * The turn's economy as shapes (proposal 2.1, boards C4-03, C4-04, C6-07): a circle for the Ação, a triangle for the Bônus, a
 * square for the Reação and a walker for the metres left. A shape is filled while it is available; spent, it is a dashed outline
 * with a slash. The words stay beside the shapes and in the accessible name, so the shape is never the only sign. A creature that
 * cannot act has all three spent. With Extra Attack, an Action spent by the first attack stays filled while attacks remain.
 */
export function economyShapes(c: EconomyFacts, options: EconomyOptions = {}): EconomyShape[] {
  const cannot = cannotAct(c);
  const left = options.attacksLeft ?? 0;
  const partial = c.actionUsed && left > 0 && (options.attacksPerAction ?? 1) > 1 && !cannot;
  const word = (spent: boolean): string =>
    cannot ? 'indisponível' : spent ? 'usada' : 'disponível';
  const shapes: EconomyShape[] = [
    {
      kind: 'action',
      text: 'Ação',
      name: partial
        ? `Ação, ${left} ${left === 1 ? 'ataque restante' : 'ataques restantes'}`
        : `Ação ${word(c.actionUsed)}`,
      spent: cannot || (c.actionUsed && !partial),
    },
    {
      kind: 'bonus',
      text: 'Bônus',
      name: `Ação bônus ${word(c.bonusActionUsed)}`,
      spent: cannot || c.bonusActionUsed,
    },
    {
      kind: 'reaction',
      text: 'Reação',
      name: reactionName(c, word(c.reactionUsed), cannot),
      spent: cannot || c.reactionUsed,
    },
  ];
  if (options.moveText) {
    shapes.push({
      kind: 'move',
      text: options.moveText,
      name: `Movimento: ${options.moveText}`,
      spent: false,
    });
  }
  return shapes;
}

/**
 * One shape, 1em wide: filled while it is there, a dashed outline with a slash once it is spent. Decorative: the words beside it
 * say the same.
 */
@Component({
  selector: 'app-economy-mark',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg viewBox="0 0 24 24" aria-hidden="true" [class.mark--spent]="spent()">
      @switch (kind()) {
        @case ('action') {
          <circle class="mark" cx="12" cy="12" r="8" />
        }
        @case ('bonus') {
          <path class="mark" d="M12 4.2l8.3 14.8H3.7z" />
        }
        @case ('reaction') {
          <rect class="mark" x="4.5" y="4.5" width="15" height="15" rx="2" />
        }
        @case ('free') {
          <path class="mark mark--line" d="M13 3L5 14h6l-1 7 8-11h-6z" />
        }
        @default {
          <g class="walker">
            <circle cx="13" cy="4.5" r="1.8" />
            <path d="M10 21l2-6 3 3v3M9 11l3-3 3 3 3 1M12 8l-1 7" />
          </g>
        }
      }
      @if (spent()) {
        <path class="slash" d="M4 20L20 4" />
      }
    </svg>
  `,
  styleUrl: './economy-mark.scss',
})
export class EconomyMark {
  readonly kind = input.required<EconomyKind | 'free'>();
  readonly spent = input(false);
}

/**
 * The shapes as chips (`pills`, the turn card) or as one line of plain words (`inline`, the bar above the phone's nav). With
 * `onlyLeft` the spent ones are left out: "Ainda tem ○ Ação ■ Reação".
 */
@Component({
  selector: 'app-economy-shapes',
  imports: [EconomyMark],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ul class="eco" [class.eco--inline]="variant() === 'inline'" [attr.aria-label]="label()">
      @for (s of shown(); track s.kind) {
        <li class="shape" [class.shape--spent]="s.spent" [attr.aria-label]="s.name">
          <app-economy-mark class="shape__icon" [kind]="s.kind" [spent]="s.spent" />
          <span class="shape__text" aria-hidden="true">{{ s.text }}</span>
        </li>
      }
    </ul>
  `,
  styleUrl: './economy-shapes.scss',
})
export class EconomyShapes {
  readonly shapes = input.required<readonly EconomyShape[]>();
  readonly variant = input<'pills' | 'inline'>('pills');
  /** Only what is still there. */
  readonly onlyLeft = input(false);
  /** What the list is called to a screen reader. */
  readonly label = input('O que tem neste turno');

  protected readonly shown = computed(() =>
    this.onlyLeft() ? this.shapes().filter((s) => !s.spent) : this.shapes(),
  );
}
