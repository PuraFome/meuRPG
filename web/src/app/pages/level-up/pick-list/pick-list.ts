import { Component, computed, input, output, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { PickItem } from '../../../core/levelup/levelup-flow';

/** How many rows a long list shows before "Ver os outros N". */
const FIRST_ROWS = 4;

/**
 * One list of choices of the guided level-up (MR-040): the new cantrips, the
 * spells for the book, the ones to prepare, the subclass, a feature's options,
 * skills and expertise. A panel with its title, how many of how many are
 * picked (a warning icon and the words while one is missing, a tick when it
 * is complete), the rows with a real radio (one place) or checkbox (several),
 * and, for a long list, a search and "Ver os outros N". A spell's row has the
 * "?" for its description. The parent owns the picks: `pick` says a row was
 * tapped. A full list disables its unpicked rows, so nothing is picked past
 * the count.
 */
@Component({
  selector: 'app-pick-list',
  imports: [MatIconModule],
  templateUrl: './pick-list.html',
  styleUrl: './pick-list.scss',
  host: { '[attr.data-missing]': 'missing() ? "" : null', '[id]': '"pick-" + pickId()' },
})
export class PickList {
  /** The picker's id, which the page uses to focus the first missing choice. */
  readonly pickId = input.required<string>();
  readonly title = input.required<string>();
  readonly lead = input('');
  readonly items = input.required<readonly PickItem[]>();
  readonly picked = input.required<ReadonlySet<string>>();
  /** How many to pick: never more than `items` has. */
  readonly count = input.required<number>();
  /** Already picked before this level, counted in "9 de 9" (the prepared spells of today). */
  readonly base = input(0);
  /** "magia", for the search's label and the empty state. */
  readonly noun = input('opção');
  /** "truques", "magias": what "Ver os outros 7 truques" counts. */
  readonly nounMany = input('opções');
  readonly searchLabel = input('');
  /** The "?" beside a spell. */
  readonly describable = input(false);
  /** The line under the title that is not the lead: who is prepared already, and so on. */
  readonly note = input('');
  /** A bold first line before the lead ("Prepare mais 2."). */
  readonly leadStrong = input('');
  /** Why the unpicked rows are off once the list is full ("Limite de 9 preparadas"). */
  readonly fullNote = input('');
  /** A line at the end of the card, inside it. */
  readonly footnote = input('');

  readonly pick = output<string>();
  readonly describe = output<PickItem>();

  protected readonly single = computed(() => this.count() === 1);
  protected readonly done = computed(() => this.picked().size >= this.count());
  protected readonly missing = computed(() => this.picked().size < this.count());
  protected readonly full = computed(() => this.picked().size >= this.count() && this.count() > 1);

  protected readonly query = signal('');
  protected readonly expanded = signal(false);
  protected readonly searchable = computed(
    () => this.searchLabel() !== '' && this.items().length > FIRST_ROWS,
  );

  /** The rows shown: the picked ones and the first few, or the matches of the search, or all. */
  protected readonly rows = computed(() => {
    const all = this.items();
    const q = this.query().trim().toLocaleLowerCase('pt-BR');
    if (q !== '') {
      return all.filter((i) => i.name.toLocaleLowerCase('pt-BR').includes(q));
    }
    if (this.expanded() || all.length <= FIRST_ROWS + 1) {
      return all;
    }
    const picked = this.picked();
    return all.filter((item, index) => index < FIRST_ROWS || picked.has(item.key));
  });
  protected readonly hidden = computed(() =>
    this.query().trim() === '' ? this.items().length - this.rows().length : 0,
  );

  /** "1 de 2", with the words a screen reader needs. */
  protected readonly countText = computed(
    () =>
      `${this.base() + Math.min(this.picked().size, this.count())} de ${this.base() + this.count()}`,
  );

  protected onSearch(event: Event): void {
    this.query.set((event.target as HTMLInputElement).value);
  }
}
