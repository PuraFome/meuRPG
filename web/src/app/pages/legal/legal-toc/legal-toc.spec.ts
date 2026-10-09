import { ChangeDetectionStrategy, Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { LegalToc, type LegalSection } from './legal-toc';

const SECTIONS: readonly LegalSection[] = [
  { id: 'um', title: '1. Primeira' },
  { id: 'dois', title: '2. Segunda' },
];

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [LegalToc],
  template: `
    <app-legal-toc [sections]="sections" />
    <h2 id="um">1. Primeira</h2>
    <h2 id="dois">2. Segunda</h2>
  `,
})
class Host {
  readonly sections = SECTIONS;
}

describe('LegalToc', () => {
  function render() {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('lists the sections with their numbers apart from their titles', () => {
    const { el } = render();
    expect(el.querySelector('nav')?.getAttribute('aria-label')).toBe('Nesta página');
    const links = Array.from(el.querySelectorAll('a'));
    expect(links.map((a) => a.textContent?.replace(/\s+/g, ' ').trim())).toEqual([
      '1 Primeira',
      '2 Segunda',
    ]);
  });

  it('scrolls to the heading and gives it the focus when a link is pressed', () => {
    const { fixture, el } = render();
    const heading = el.querySelector<HTMLElement>('#dois')!;
    const scroll = vi.fn();
    heading.scrollIntoView = scroll;
    el.querySelectorAll('a')[1].click();
    fixture.detectChanges();
    expect(scroll).toHaveBeenCalledOnce();
    expect(heading.getAttribute('tabindex')).toBe('-1');
  });
});
