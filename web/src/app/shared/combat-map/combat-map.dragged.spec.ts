import { TestBed } from '@angular/core/testing';

import { combatant } from '../../core/combat/combat-testing';
import { CombatMap } from './combat-map';

function map(inputs: Record<string, unknown>) {
  const fixture = TestBed.createComponent(CombatMap);
  const ref = fixture.componentRef;
  ref.setInput('image', { url: '/images/x', width: 2000, height: 1400 });
  ref.setInput('columns', 20);
  ref.setInput('rows', 14);
  ref.setInput('combatants', [combatant({ id: 'a', label: 'Toren', col: 8, row: 7 })]);
  for (const [k, v] of Object.entries(inputs)) {
    ref.setInput(k, v);
  }
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('CombatMap: where a dragged creature ends (W7-X)', () => {
  it('draws a dashed token with the initial on the square it is given, and hides it from a screen reader', () => {
    const el = map({ dragged: { square: { col: 7, row: 7 }, initial: 'H' } });
    const mark = el.querySelector<HTMLElement>('.cm__dragged')!;
    expect(mark.textContent?.trim()).toBe('H');
    expect(mark.getAttribute('aria-hidden')).toBe('true');
    expect(mark.style.left).toBe(`${(7 / 20) * 100}%`);
    expect(mark.style.top).toBe(`${(7 / 14) * 100}%`);
  });

  it('draws nothing by default', () => {
    expect(map({}).querySelector('.cm__dragged')).toBeNull();
  });
});
