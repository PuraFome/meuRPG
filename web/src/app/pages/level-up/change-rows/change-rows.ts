import { Component, input } from '@angular/core';

import type { ChangeRow } from '../../../core/levelup/levelup-summary';

/**
 * Rows of "before → after" (the summary, "O que muda com Inteligência 20", the right
 * column of the desktop page): the label and a small line on the left, the old value
 * muted and the new one bold on the right. The arrow is an icon-like glyph with the words
 * "para" read by a screen reader, so "18 → 20" never sounds like "18 20".
 */
@Component({
  selector: 'app-change-rows',
  template: `
    <ul class="rows">
      @for (r of rows(); track r.key) {
        <li class="row" [class.row--big]="!compact()">
          <span class="row__text">
            <span class="row__label">{{ r.label }}</span>
            @if (r.sub) {
              <span class="row__sub">{{ r.sub }}</span>
            }
          </span>
          <span class="row__values">
            @if (r.before !== '') {
              <span class="row__before">{{ r.before }}</span>
              <span class="mr-visually-hidden"> para </span>
              <span aria-hidden="true" class="row__arrow">→</span>
            }
            <strong class="row__after">{{ r.after }}</strong>
          </span>
        </li>
      }
    </ul>
  `,
  styleUrl: './change-rows.scss',
})
export class ChangeRowsList {
  readonly rows = input.required<readonly ChangeRow[]>();
  /** The right column's size: tighter rows. */
  readonly compact = input(false);
}
