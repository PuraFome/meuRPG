import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { GLYPH_PATHS } from '../../core/puzzles/puzzle-symbols';

/**
 * One face of a wheel or a pillar: our own stroke drawing for a rune or a glyph, or the character itself for a digit or
 * a letter. Decorative (`aria-hidden`): every screen writes the face's name beside it, so the drawing is never the only way to tell
 * two faces apart. The drawing takes the color of the words around it.
 */
@Component({
  selector: 'app-symbol-glyph',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (path(); as p) {
      <svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
        <path [attr.d]="p.d" />
        @if (p.fill) {
          <path [attr.d]="p.fill" fill="currentColor" />
        }
      </svg>
    } @else {
      <span class="ch" aria-hidden="true">{{ face() }}</span>
    }
  `,
  styles: `
    :host {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--glyph, 28px);
      height: var(--glyph, 28px);
      color: inherit;
    }

    svg {
      width: 100%;
      height: 100%;
    }

    .ch {
      font-family: var(--mr-font-display);
      font-weight: 700;
      font-size: calc(var(--glyph, 28px) * 0.85);
      line-height: 1;
    }
  `,
})
export class SymbolGlyph {
  /** The face's key: "moon", "owl", "7", "K". */
  readonly face = input.required<string>();
  protected readonly path = computed(() => GLYPH_PATHS[this.face()] ?? null);
}
