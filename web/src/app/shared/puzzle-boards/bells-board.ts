import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { SymbolFace } from '../../core/puzzles/puzzle-symbols';
import { SymbolGlyph } from './symbol-glyph';

/** "Sino pequeno" is written "Pequeno" under its drawing: the group's label already says they are bells. */
export function bellWord(name: string): string {
  const word = name.replace(/^Sino\s+/i, '');
  return word.charAt(0).toUpperCase() + word.slice(1);
}

/**
 * The bells of a sequence (MR-038, E10-12): one big button for each, our own drawing with its name written. `play` is the players'
 * (any of them strikes a bell; the server checks the order) and `pick` the master's form (a tap puts the next step). A bell is
 * always a named button, so a screen reader hears "Sino alto"; `lit` is the bell that is playing, and `disabled` is a play that
 * runs or has not happened (the page says why, in words beside it). 84 px wide and 88 tall in play, 64 in the form.
 */
@Component({
  selector: 'app-bells-board',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule, SymbolGlyph],
  host: { '[class.bb--pick]': "mode() === 'pick'", '[class.bb--many]': 'count() > 4' },
  template: `
    <div class="bells" role="group" [attr.aria-label]="label()" [attr.aria-disabled]="disabled() ? 'true' : null">
      @for (face of faces(); track face.key; let i = $index) {
        <button type="button" class="bell" [class.bell--lit]="i === lit()" [attr.aria-label]="face.namePt" [attr.aria-disabled]="disabled() ? 'true' : null" (click)="!disabled() && strike.emit(i)">
          <!-- The drawing sits in a mat-icon box, so the layout checks read it as the button's icon, above its word. -->
          <mat-icon class="bell__icon" aria-hidden="true"><app-symbol-glyph [face]="face.key" /></mat-icon>
          <span class="bell__name" aria-hidden="true">{{ word(face) }}</span>
        </button>
      }
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .bells {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(84px, 1fr));
      gap: var(--mr-space-2);
      max-width: 460px;
    }

    :host(.bb--pick) .bells {
      grid-template-columns: repeat(auto-fill, 64px);
      max-width: none;
    }

    // On a phone four bells are two by two, and five to eight three across: never three and one left over.
    @media (max-width: 480px) {
      :host(:not(.bb--pick)) .bells {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }

      :host(.bb--many:not(.bb--pick)) .bells {
        grid-template-columns: repeat(3, minmax(0, 1fr));
      }
    }

    .bell {
      --glyph: 40px;
      box-sizing: border-box;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 6px;
      min-height: 88px;
      // The word's line has room under its capitals, so the drawing's top gap is the bigger one: the content sits in the middle.
      padding: 10px 4px 6px;
      border: 1.5px solid var(--mr-control-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-surface);
      color: var(--mr-ink);
      font: inherit;
      cursor: pointer;

      &:hover {
        background: var(--mr-ground);
      }

      &:focus-visible {
        outline: 3px solid var(--mr-focus);
        outline-offset: 2px;
      }

      &[aria-disabled='true'] {
        cursor: default;
        opacity: 0.6;
        border-style: dashed;
      }
    }

    :host(.bb--pick) .bell {
      --glyph: 32px;
      min-height: 64px;
      padding: 6px 2px 2px;
      gap: 2px;
    }

    .bell__icon {
      width: var(--glyph);
      height: var(--glyph);
      font-size: var(--glyph);
      line-height: 1;
    }

    .bell__name {
      font-size: 14px;
      line-height: 16px;
      color: var(--mr-ink-muted);
      white-space: nowrap;
    }

    :host(.bb--pick) .bell__name {
      font-size: 12px;
    }

    // The bell that is playing: the accent frame and ground, and the word in ink (the frame is never the only sign: the page says "passo 3 de 6").
    .bell--lit {
      border-color: var(--mr-accent);
      background: var(--mr-accent-soft);

      .bell__name {
        color: var(--mr-ink);
        font-weight: 700;
      }
    }

    @media (max-width: 767.98px) {
      .bell {
        min-height: 92px;
      }

      :host(.bb--pick) .bell {
        min-height: 64px;
      }
    }
  `,
})
export class BellsBoard {
  readonly faces = input.required<readonly SymbolFace[]>();
  readonly mode = input<'play' | 'pick'>('play');
  /** The bell that is playing now (an index), or -1. */
  readonly lit = input(-1);
  readonly disabled = input(false);
  readonly label = input('Os sinos');

  /** The bell struck, from 0. */
  readonly strike = output<number>();

  protected readonly word = (face: SymbolFace): string => bellWord(face.namePt);
  protected readonly count = computed(() => this.faces().length);
}
