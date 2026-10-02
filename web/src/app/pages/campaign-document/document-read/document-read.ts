import { Component, ElementRef, computed, inject, input, output, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { FictionNotice } from '../../../shared/fiction-notice/fiction-notice';
import { outline, parseMarkdown } from '../../../shared/markdown/markdown';
import { MarkdownView, type MarkdownRefs, type RefOpen } from '../../../shared/markdown/markdown-view';
import { DocumentToc } from '../document-toc/document-toc';

/**
 * The document's read mode (E5-27, E5-30): the "Sumário" and the rendered
 * body in a panel. From 900px the Sumário is a sticky column on the left;
 * on a phone it is a disclosure above the text, and images go edge to edge
 * inside the panel.
 */
@Component({
  selector: 'app-document-read',
  imports: [DocumentToc, FictionNotice, MarkdownView, MatIconModule],
  templateUrl: './document-read.html',
  styleUrl: './document-read.scss',
  host: { '(window:scroll)': 'track()' },
})
export class DocumentRead {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly body = input.required<string>();
  readonly refs = input<MarkdownRefs | null>(null);
  readonly openRef = output<RefOpen>();

  protected readonly toc = computed(() => outline(parseMarkdown(this.body())));
  protected readonly isEmpty = computed(() => this.body().trim() === '');
  protected readonly clicked = signal<number | null>(null);
  protected readonly scrolled = signal<number | null>(null);
  protected readonly current = computed(
    () => this.clicked() ?? this.scrolled() ?? this.toc()[0]?.index ?? null,
  );

  /** Takes the reader to a section and puts focus on its heading. */
  protected goTo(index: number): void {
    const heading = document.getElementById(`doc-h-${index}`);
    if (!heading) {
      return;
    }
    const reduce = typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
    heading.scrollIntoView({ behavior: reduce ? 'auto' : 'smooth', block: 'start' });
    heading.tabIndex = -1;
    heading.focus({ preventScroll: true });
    this.clicked.set(index);
  }

  /** The section being read: the last heading above the top of the page. */
  protected track(): void {
    this.clicked.set(null);
    let found: number | null = null;
    for (const item of this.toc()) {
      const el = this.host.nativeElement.querySelector(`#doc-h-${item.index}`);
      if (el && el.getBoundingClientRect().top <= 120) {
        found = item.index;
      }
    }
    this.scrolled.set(found);
  }
}
