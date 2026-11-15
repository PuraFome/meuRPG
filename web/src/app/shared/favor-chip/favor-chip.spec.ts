import { TestBed } from '@angular/core/testing';

import { FavorChip } from './favor-chip';

describe('FavorChip', () => {
  function render(variant?: 'pill' | 'inline') {
    const fixture = TestBed.createComponent(FavorChip);
    if (variant) {
      fixture.componentRef.setInput('variant', variant);
    }
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('is a star and the word "Inspiração", the star hidden from a screen reader', () => {
    const el = render();
    expect(el.textContent?.replace(/\s+/g, ' ').trim()).toBe('starInspiração');
    expect(el.querySelector('mat-icon')?.getAttribute('aria-hidden')).toBe('true');
  });

  it('reads "Tem Inspiração" in the order row, where it sits beside the name', () => {
    const el = render('inline');
    expect(el.textContent?.replace(/\s+/g, ' ').trim()).toBe('starTem Inspiração');
    expect(el.querySelector('.mr-visually-hidden')?.textContent).toBe('Tem ');
  });

  it('is a pill by default and a line of the row when asked', () => {
    expect(render().classList.contains('favor--pill')).toBe(true);
    TestBed.resetTestingModule();
    expect(render('inline').classList.contains('favor--inline')).toBe(true);
  });
});
