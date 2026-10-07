import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Title } from '@angular/platform-browser';
import { Router, TitleStrategy, provideRouter } from '@angular/router';

import { routes } from '../../app.routes';
import { CONTENT_ROUTES } from '../../pages/content/content.routes';
import { PageTitle, PageTitleStrategy, setPageSubject } from './page-title';

@Component({ template: '' })
class Page {
  readonly name = signal<string | null>(null);
  constructor() {
    setPageSubject(() => this.name());
  }
}
@Component({ template: '' })
class Other {}

describe('the tab\'s title', () => {
  let title: Title;
  let router: Router;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        provideRouter([
          { path: 'one', title: 'Campanha', component: Page },
          { path: 'one-too', title: 'Campanha', component: Page },
          { path: 'two', title: 'Ficha', component: Other },
          { path: 'bare', component: Other },
        ]),
        { provide: TitleStrategy, useExisting: PageTitleStrategy },
      ],
    });
    title = TestBed.inject(Title);
    router = TestBed.inject(Router);
  });

  it('is the route\'s title, then the app\'s name', async () => {
    await router.navigateByUrl('/two');
    expect(title.getTitle()).toBe('Ficha · MeuRPG');
  });

  it('is only the app\'s name for a route with no title', async () => {
    await router.navigateByUrl('/bare');
    expect(title.getTitle()).toBe('MeuRPG');
  });

  it('puts the name of what the page shows in front once the page has it, and drops it when the page forgets it', async () => {
    await router.navigateByUrl('/one');
    const page = TestBed.createComponent(Page);
    page.detectChanges();
    expect(title.getTitle()).toBe('Campanha · MeuRPG');
    page.componentInstance.name.set('  Mirathel ');
    page.detectChanges();
    expect(title.getTitle()).toBe('Mirathel · Campanha · MeuRPG');
    page.componentInstance.name.set(null);
    page.detectChanges();
    expect(title.getTitle()).toBe('Campanha · MeuRPG');
  });

  it('never shows the previous page\'s name on the next page', async () => {
    await router.navigateByUrl('/one');
    const page = TestBed.createComponent(Page);
    page.componentInstance.name.set('Mirathel');
    page.detectChanges();
    expect(title.getTitle()).toBe('Mirathel · Campanha · MeuRPG');
    await router.navigateByUrl('/two');
    expect(title.getTitle()).toBe('Ficha · MeuRPG');
    page.destroy();
    expect(title.getTitle()).toBe('Ficha · MeuRPG');
  });

  it('keeps the name through a navigation that stays on the same page (a filter in the query)', async () => {
    await router.navigateByUrl('/one');
    const page = TestBed.createComponent(Page);
    page.componentInstance.name.set('Mirathel');
    page.detectChanges();
    await router.navigateByUrl('/one?filter=a');
    expect(title.getTitle()).toBe('Mirathel · Campanha · MeuRPG');
  });

  it('does not let a component that goes away clear the name the next one set', () => {
    const pageTitle = TestBed.inject(PageTitle);
    const a = {};
    const b = {};
    pageTitle.begin('Ficha');
    pageTitle.setSubject(a, 'A');
    pageTitle.setSubject(b, 'B');
    pageTitle.release(a);
    expect(title.getTitle()).toBe('B · Ficha · MeuRPG');
    pageTitle.release(b);
    expect(title.getTitle()).toBe('Ficha · MeuRPG');
  });
});

describe('every route has a title (WCAG 2.4.2)', () => {
  /** The routes that carry a page: those with a component or lazy component (a `loadChildren` one holds its pages in the children). */
  function missing(list: typeof routes, inherited: boolean, where: string): string[] {
    return list.flatMap((r) => {
      const path = `${where}/${r.path}`;
      const has = typeof r.title === 'string' && r.title.length > 0;
      const own = r.loadComponent || r.component ? (has || inherited ? [] : [path]) : [];
      return [...own, ...missing(r.children ?? [], has || inherited, path)];
    });
  }

  it('in the main table, and in the content pages', () => {
    expect(missing(routes, false, '')).toEqual([]);
    expect(missing(CONTENT_ROUTES, false, '/content')).toEqual([]);
  });

  it('is a sentence-case Portuguese name, never the app\'s name twice', () => {
    for (const r of routes) {
      expect(r.title).toBeTypeOf('string');
      expect(r.title).not.toContain('MeuRPG');
    }
  });
});
