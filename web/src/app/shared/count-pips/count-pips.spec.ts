import { TestBed } from '@angular/core/testing';

import { CountPips } from './count-pips';

describe('CountPips', () => {
  function render(left: number, total: number) {
    const fixture = TestBed.createComponent(CountPips);
    fixture.componentRef.setInput('left', left);
    fixture.componentRef.setInput('total', total);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('draws a filled dot for each use left and a ring for each one spent', () => {
    const el = render(2, 3);
    const pips = Array.from(el.querySelectorAll('.pip'));
    expect(pips.map((p) => p.classList.contains('pip--left'))).toEqual([true, true, false]);
  });

  it('is decoration: a screen reader skips it', () => {
    expect(render(1, 1).getAttribute('aria-hidden')).toBe('true');
  });

  it('draws nothing for a total of none', () => {
    expect(render(0, 0).querySelectorAll('.pip')).toHaveLength(0);
  });
});
