import { DestroyRef, Injectable, effect, inject } from '@angular/core';
import { Title } from '@angular/platform-browser';
import { RouterStateSnapshot, TitleStrategy } from '@angular/router';

/** The app's name, last in every title ("Minhas campanhas · MeuRPG"). */
export const APP_NAME = 'MeuRPG';

/** The words between the parts of a title. */
const SEPARATOR = ' · ';

/**
 * The tab's title (WCAG 2.4.2): the page's name from its route, then the name
 * of the thing the page shows (a campaign, a character) once the page has
 * loaded it, then the app's name: "Pensantus · Ficha · MeuRPG".
 *
 * The page's name comes first from the route's `title`, at once, so the tab
 * never shows the previous page's title. The subject arrives later, from the
 * component (`setPageSubject`); until it does, the title is simply the shorter
 * one, never a wrong one. Every navigation clears the subject.
 */
@Injectable({ providedIn: 'root' })
export class PageTitle {
  private readonly title = inject(Title);
  private page = '';
  private subject = '';
  /** Who set the subject, so a component that goes away never clears the next page's. */
  private owner: object | null = null;

  /**
   * The router calls this on every navigation that ends, query-only ones too.
   * A different page drops the subject; the same page (a filter in the query,
   * or a campaign's own page loading another campaign, which resets its own
   * subject while it loads) keeps it.
   */
  begin(page: string | undefined): void {
    const next = page ?? '';
    if (next !== this.page) {
      this.subject = '';
      this.owner = null;
    }
    this.page = next;
    this.render();
  }

  /** The name of what the page shows ("" clears it). */
  setSubject(owner: object, subject: string | null | undefined): void {
    this.owner = owner;
    this.subject = subject?.trim() ?? '';
    this.render();
  }

  /** The owner goes away: its subject goes with it, unless someone else took over. */
  release(owner: object): void {
    if (this.owner === owner) {
      this.owner = null;
      this.subject = '';
      this.render();
    }
  }

  private render(): void {
    this.title.setTitle([this.subject, this.page, APP_NAME].filter(Boolean).join(SEPARATOR));
  }
}

/** Sets the tab's title from the route's `title` (see `PageTitle`). */
@Injectable({ providedIn: 'root' })
export class PageTitleStrategy extends TitleStrategy {
  private readonly pageTitle = inject(PageTitle);

  override updateTitle(snapshot: RouterStateSnapshot): void {
    this.pageTitle.begin(this.buildTitle(snapshot));
  }
}

/**
 * Called in a component's constructor: shows what `subject()` returns in the
 * tab's title while the component lives. The effect dies with the component,
 * so a page that loads late can never write onto the next page's title.
 */
export function setPageSubject(subject: () => string | null | undefined): void {
  const pageTitle = inject(PageTitle);
  const owner = {};
  effect(() => pageTitle.setSubject(owner, subject()));
  inject(DestroyRef).onDestroy(() => pageTitle.release(owner));
}
