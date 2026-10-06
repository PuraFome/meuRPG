import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import type { SymbolFace } from '../../core/puzzles/puzzle-symbols';
import { SymbolGlyph } from './symbol-glyph';

/**
 * The row of steps of a sequence (MR-038, E10-12): a slot for each step, with the bell of the step in it when it is known. The master's
 * form and live card draw the whole sequence; a player's phone draws only the steps the server has revealed in the play that runs
 * (`steps` is as long as what was revealed, and the slots after it stay dashed and empty). `current` frames the step playing now.
 * A list for a screen reader: "Passo 3: Sino pequeno", or "Passo 4: ainda não mostrado".
 */
@Component({
  selector: 'app-sequence-strip',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [SymbolGlyph],
  template: `
    <ol class="strip" [attr.aria-label]="label()">
      @for (slot of slots(); track $index) {
        <li class="slot" [class.slot--empty]="slot === null" [class.slot--lit]="$index === current()" [attr.aria-label]="describe($index, slot)">
          @if (slot !== null) {
            <app-symbol-glyph [face]="slot.key" aria-hidden="true" />
          }
          <span class="slot__n" aria-hidden="true">{{ $index + 1 }}</span>
        </li>
      }
    </ol>
  `,
  styles: `
    :host {
      display: block;
    }

    // A grid: six steps stay on one row down to a 320 px phone (each slot shrinks to 38 px, no less), and more wrap evenly.
    .strip {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(38px, 44px));
      gap: 6px;
      margin: 0;
      padding: 0;
      list-style: none;
    }

    .slot {
      --glyph: 28px;
      position: relative;
      box-sizing: border-box;
      display: flex;
      align-items: center;
      justify-content: center;
      height: 44px;
      border: 1.5px solid var(--mr-control-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-surface);
      color: var(--mr-ink);
    }

    .slot--empty {
      border-style: dashed;
      background: none;
    }

    // The step that is playing: the accent frame and ground.
    .slot--lit {
      border-color: var(--mr-accent);
      background: var(--mr-accent-soft);
    }

    // The number of the step, small, in the corner: the order is never only the position.
    .slot__n {
      position: absolute;
      top: 1px;
      left: 4px;
      font-size: 11px;
      line-height: 12px;
      color: var(--mr-ink-muted);
    }
  `,
})
export class SequenceStrip {
  readonly faces = input.required<readonly SymbolFace[]>();
  /** The bell of each step known, in order; may be shorter than `total`. */
  readonly steps = input.required<readonly number[]>();
  /** How many slots: the sequence's steps. */
  readonly total = input.required<number>();
  /** The step playing now (an index), or -1. */
  readonly current = input(-1);
  readonly label = input('Os passos da sequência');

  protected readonly slots = computed<readonly (SymbolFace | null)[]>(() =>
    Array.from({ length: Math.max(this.total(), this.steps().length) }, (_, i) => {
      const bell = this.steps()[i];
      return bell === undefined ? null : (this.faces()[bell] ?? null);
    }),
  );

  protected describe(index: number, slot: SymbolFace | null): string {
    return slot ? `Passo ${index + 1}: ${slot.namePt}` : `Passo ${index + 1}: ainda não mostrado`;
  }
}
