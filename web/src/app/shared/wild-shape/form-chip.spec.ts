import { TestBed } from '@angular/core/testing';

import { FormChip } from './form-chip';

function chip(beast: string, time = '') {
  const f = TestBed.createComponent(FormChip);
  f.componentRef.setInput('beast', beast);
  f.componentRef.setInput('time', time);
  f.detectChanges();
  return (f.nativeElement as HTMLElement).textContent!.replace(/\s+/g, ' ').trim();
}

describe('FormChip (584-01, 584-02)', () => {
  it('reads the beast alone for the readers who may not have the time', () => {
    const t = chip('Lobo');
    expect(t).toContain('Na forma de Lobo');
    expect(t).not.toContain('hourglass');
  });

  it('adds the time after the name, without the words its hourglass already says', () => {
    expect(chip('Lobo', '52 min restantes')).toContain('Na forma de Lobo');
    expect(chip('Lobo', '52 min restantes')).toMatch(/hourglass_empty ?52 min$/);
    expect(chip('Lobo', '1 h 52 min restantes')).toMatch(/1 h 52 min$/);
    expect(chip('Lobo', 'Restam 4 rodadas')).toMatch(/4 rodadas$/);
  });
});
