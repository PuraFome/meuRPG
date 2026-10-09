import { ChangeDetectionStrategy, Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';

import { AppFooter } from './app-footer';

@Component({ template: '', changeDetection: ChangeDetectionStrategy.OnPush })
class Blank {}

describe('AppFooter', () => {
  function render() {
    TestBed.configureTestingModule({
      imports: [AppFooter],
      providers: [
        provideRouter([
          { path: 'terms', component: Blank },
          { path: 'privacy', component: Blank },
          { path: 'credits', component: Blank },
        ]),
      ],
    });
    const fixture = TestBed.createComponent(AppFooter);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('links the three pages that are always reachable, in the nav named "Rodapé"', () => {
    const { el } = render();
    const nav = el.querySelector('nav[aria-label="Rodapé"]')!;
    const links = Array.from(nav.querySelectorAll('a'));
    expect(links.map((a) => a.textContent?.trim())).toEqual([
      'Termos de uso',
      'Privacidade',
      'Créditos',
    ]);
    expect(links.map((a) => a.getAttribute('href'))).toEqual(['/terms', '/privacy', '/credits']);
  });

  it('shows the current year with the one-colour d20, hidden from a screen reader', () => {
    const { el } = render();
    expect(el.querySelector('.footer__year')?.textContent).toContain(
      `© ${new Date().getFullYear()} MeuRPG`,
    );
    const d20 = el.querySelector('.footer__year svg')!;
    expect(d20.getAttribute('aria-hidden')).toBe('true');
    // One colour: every stroke and fill is the text colour, never a hand-written hex.
    expect(d20.innerHTML).not.toMatch(/#[0-9a-f]{3,6}/i);
  });

  it('marks the page that is open with aria-current="page"', async () => {
    const { fixture, el } = render();
    await TestBed.inject(Router).navigateByUrl('/privacy');
    fixture.detectChanges();
    const current = el.querySelectorAll('a[aria-current="page"]');
    expect(current).toHaveLength(1);
    expect(current[0].textContent?.trim()).toBe('Privacidade');
  });
});
