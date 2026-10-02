import { Component, DestroyRef, computed, inject, input, output, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

export interface TocItem {
  /** The heading's block index, which is its id's suffix. */
  readonly index: number;
  readonly text: string;
}

/**
 * The "Sumário" (E5-27, E5-30): the document's `##` headings. A sticky
 * column on a desktop (the section being read in `accent-soft` and bold,
 * `aria-current="location"`); a closed disclosure on a phone, with 44px
 * rows. Choosing one scrolls the page to the heading and moves focus
 * there, so a keyboard continues reading from it.
 */
@Component({
  selector: 'app-document-toc',
  imports: [MatIconModule],
  templateUrl: './document-toc.html',
  styleUrl: './document-toc.scss',
})
export class DocumentToc {
  readonly items = input.required<readonly TocItem[]>();
  /** The index of the heading being read. */
  readonly current = input<number | null>(null);
  readonly pick = output<number>();

  protected readonly expanded = signal(false);
  /** The phone layout shows a disclosure. */
  protected readonly narrow = signal(
    typeof matchMedia === 'function' && matchMedia('(max-width: 899.98px)').matches,
  );
  protected readonly open = computed(() => !this.narrow() || this.expanded());

  constructor() {
    if (typeof matchMedia === 'function') {
      const query = matchMedia('(max-width: 899.98px)');
      const listener = (e: MediaQueryListEvent) => this.narrow.set(e.matches);
      query.addEventListener('change', listener);
      inject(DestroyRef).onDestroy(() => query.removeEventListener('change', listener));
    }
  }
}
