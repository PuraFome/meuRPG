import { Component, computed, input } from '@angular/core';

import {
  type Split,
  divisionSentence,
  eachLine,
  lostShort,
  shortDivision,
  splitAnnouncement,
} from '../../core/progression/xp-math';

/**
 * The live division of an award (E7-06, E7-07): the big "116 XP para cada" and
 * the sum written out under it. `block` is the end-of-combat card (the
 * division in full words, in a grey box); `foot` is the sheet's fixed footer
 * (one line, the big number and the short sum side by side; on a screen 600px
 * tall or less, only the number and what is lost, the sum going to the body). With nobody
 * checked it says so and the button beside it waits.
 *
 * What a screen reader hears when the number changes is a separate polite
 * status ("116 XP para cada."), so the sum is never read at every tick.
 */
@Component({
  selector: 'app-xp-split',
  template: `
    <div class="split" [class.split--foot]="variant() === 'foot'" [class.split--none]="split().count === 0">
      @if (split().count === 0) {
        <span class="split__big">Ninguém marcado</span>
      } @else if (total() === 0 && emptyText()) {
        <span class="split__big split__big--quiet">{{ emptyText() }}</span>
      } @else {
        <span class="split__big">{{ each() }}</span>
        @if (split().lost > 0) {
          <span class="split__lost">· {{ lost() }}</span>
        }
        <span class="split__sum">{{ sum() }}</span>
      }
    </div>
    <p class="mr-visually-hidden" role="status" aria-live="polite">{{ announcement() }}</p>
  `,
  styleUrl: './xp-split.scss',
})
export class XpSplit {
  readonly split = input.required<Split>();
  /** The XP being split. */
  readonly total = input.required<number>();
  /** The gold of a gold award, for "120 PO = 120 XP ÷ 3 = 40"; 0 for the rest. */
  readonly gold = input(0);
  readonly variant = input<'block' | 'foot'>('block');
  /** Said, quietly, while there is nothing to divide yet (instead of "0 XP para cada"). */
  readonly emptyText = input('');

  protected readonly each = computed(() => eachLine(this.split()));
  protected readonly sum = computed(() =>
    this.variant() === 'foot'
      ? shortDivision(this.total(), this.split(), this.gold())
      : divisionSentence(this.total(), this.split()),
  );
  protected readonly lost = computed(() => lostShort(this.split().lost));
  protected readonly announcement = computed(() =>
    this.total() === 0 && this.emptyText() && this.split().count > 0
      ? ''
      : splitAnnouncement(this.split()),
  );
}
