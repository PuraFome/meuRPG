import { TestBed } from '@angular/core/testing';

import { flat } from '../../../core/creatures/creatures-testing';
import {
  BUGBEAR,
  GOBLIN,
  HOBGOBLIN,
  OGRE,
  evaluation,
  line,
} from '../../../core/encounters/encounters-testing';
import { BudgetBar } from './budget-bar';

describe('BudgetBar: the difficulty bar of the encounter builder (MR-043, RN-29, E10-09)', () => {
  function bar(ev: ReturnType<typeof evaluation> | null, stale = false) {
    const fixture = TestBed.createComponent(BudgetBar);
    fixture.componentRef.setInput('evaluation', ev);
    fixture.componentRef.setInput('stale', stale);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }
  const moderate = () =>
    evaluation([line(OGRE, 1), line(BUGBEAR, 2), line(HOBGOBLIN, 4), line(GOBLIN, 6)]);
  const over = () =>
    evaluation([line(OGRE, 4), line(BUGBEAR, 2), line(HOBGOBLIN, 4), line(GOBLIN, 6)]);

  it('draws nothing before the server has answered', () => {
    expect(bar(null).querySelector('.bar')).toBeNull();
  });

  it('puts the three budgets above the bar, the total on the marker and the band in an outlined word', () => {
    const el = bar(moderate());
    expect(Array.from(el.querySelectorAll('.bar__budget')).map((n) => flat(n))).toEqual([
      '1.250',
      '1.875',
      '2.600',
    ]);
    expect(flat(el.querySelector('.bar__marker'))).toBe('1.550 XP');
    expect(flat(el.querySelector('.bar__word--on'))).toBe('Moderada');
    expect(Array.from(el.querySelectorAll('.bar__word')).map((n) => flat(n))).toEqual([
      'Baixa',
      'Moderada',
      'Alta',
      'Acima de alta',
    ]);
    expect(el.querySelector('[role=img]')?.getAttribute('aria-label')).toBe(
      'Dificuldade Moderada: 1.550 XP. Orçamentos: baixa até 1.250, moderada até 1.875, alta até 2.600 XP.',
    );
  });

  it("the phone's list of bands has the limit of each, the current one marked for assistive technology and by its dot and bold word", () => {
    const el = bar(moderate());
    const items = Array.from(el.querySelectorAll('.band'));
    expect(items.map((i) => flat(i))).toEqual([
      'Baixa até 1.250 XP',
      'Moderada até 1.875 XP',
      'Alta até 2.600 XP',
      'Acima de alta mais de 2.600 XP',
    ]);
    expect(items.map((i) => i.getAttribute('aria-current'))).toEqual([null, 'true', null, null]);
  });

  it('past the high budget the bar is full, the marker sits at the tip with "›", the band is "Acima de alta" and it is never "mortal"', () => {
    const el = bar(over());
    expect(el.querySelector<HTMLElement>('.bar__fill')!.style.width).toBe('100%');
    expect(flat(el.querySelector('.bar__marker'))).toBe('› 2.900 XP');
    expect(el.querySelector('.bar__marker--end')).not.toBeNull();
    expect(flat(el.querySelector('.bar__word--on'))).toBe('Acima de alta');
    expect(el.textContent?.toLowerCase()).not.toContain('mortal');
  });

  it('dims while a newer measure is on its way', () => {
    expect(bar(moderate(), true).querySelector('.bar--stale')).not.toBeNull();
    TestBed.resetTestingModule();
    expect(bar(moderate(), false).querySelector('.bar--stale')).toBeNull();
  });
});
