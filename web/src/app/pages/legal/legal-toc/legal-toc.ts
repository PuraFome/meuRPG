import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  afterNextRender,
  inject,
  input,
} from '@angular/core';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { DOCUMENT } from '@angular/common';

export interface LegalSection {
  /** The anchor: the `id` of the section's `h2`. */
  readonly id: string;
  readonly title: string;
}

/** From a tablet up the contents are open beside the text; on a phone they start closed. */
const OPEN_FROM = '(min-width: 768px)';

/**
 * "Nesta página": the table of contents of a legal page, a closed `details` on a phone and an open
 * one beside the text on a wider screen. The links are anchors: the router keeps the fragment in
 * the address, and the section's heading is scrolled to and takes the focus (the `<base href="/">`
 * would make a bare `#id` link leave the page).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-legal-toc',
  imports: [RouterLink],
  templateUrl: './legal-toc.html',
  styleUrl: './legal-toc.scss',
})
export class LegalToc {
  readonly sections = input.required<readonly LegalSection[]>();

  private readonly document = inject(DOCUMENT);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly fragment = inject(ActivatedRoute).snapshot.fragment;

  constructor() {
    afterNextRender(() => {
      const view = this.document.defaultView;
      const details = this.host.nativeElement.querySelector('details');
      if (details) {
        // After the first render: `matchMedia` does not exist on the server or in every test DOM.
        details.open = view?.matchMedia?.(OPEN_FROM).matches ?? false;
      }
      // A link with `#section` in it: the browser cannot do it, the page is drawn by the app.
      if (this.fragment) {
        this.show(this.fragment);
      }
    });
  }

  /** The title without its number, which the list draws on its own. */
  protected label(section: LegalSection): string {
    return section.title.replace(/^\d+\.\s*/, '');
  }

  protected go(id: string): void {
    this.show(id);
  }

  private show(id: string): void {
    const heading = this.document.getElementById(id);
    if (!heading) {
      return;
    }
    heading.setAttribute('tabindex', '-1');
    heading.scrollIntoView({ block: 'start' });
    heading.focus({ preventScroll: true });
  }
}
