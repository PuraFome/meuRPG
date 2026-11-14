import { TestBed } from '@angular/core/testing';

import type { MarksVm } from '../../core/characters/gear';
import { WeaponTags } from './weapon-tags';

function render(marks: MarksVm | null | undefined, compact = false): HTMLElement {
  const fixture = TestBed.createComponent(WeaponTags);
  fixture.componentRef.setInput('marks', marks);
  fixture.componentRef.setInput('compact', compact);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('WeaponTags', () => {
  it('draws nothing for a plain weapon, or for no marks at all', () => {
    expect(
      render({ silvered: false, adamantine: false, magical: false }).querySelector('ul'),
    ).toBeNull();
    expect(render(undefined).querySelector('ul')).toBeNull();
    expect(render(null).querySelector('ul')).toBeNull();
  });

  it('says each mark in words, in the order of the grid, with the icon only as a help', () => {
    const el = render({ silvered: true, adamantine: true, magical: true });
    const words = Array.from(el.querySelectorAll('li')).map((li) => li.textContent!.trim());
    expect(words).toEqual(['dark_modePrateada', 'diamondAdamantina', 'auto_awesomeMágica']);
    expect(el.querySelector('ul')!.getAttribute('aria-label')).toBe('Marcas da arma');
    el.querySelectorAll('mat-icon').forEach((i) =>
      expect(i.getAttribute('aria-hidden')).toBe('true'),
    );
  });

  it('draws only the marks that are set', () => {
    const el = render({ silvered: false, adamantine: false, magical: true });
    expect(Array.from(el.querySelectorAll('li')).map((li) => li.textContent)).toEqual([
      expect.stringContaining('Mágica'),
    ]);
  });

  it('draws each mark as a round chip with only its icon when compact, the word kept for a screen reader', () => {
    const el = render({ silvered: true, adamantine: false, magical: true }, true);
    const chips = Array.from(el.querySelectorAll('li'));
    expect(chips.map((li) => li.querySelector('.mr-visually-hidden')!.textContent)).toEqual([
      'Prateada',
      'Mágica',
    ]);
    chips.forEach((li) => {
      expect(li.classList).toContain('weapon-tags__chip');
      expect(li.classList).not.toContain('mr-tag');
      expect(li.querySelector('mat-icon')!.getAttribute('aria-hidden')).toBe('true');
    });
  });
});
