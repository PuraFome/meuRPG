import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { RollModeKind, RollNoteSchema } from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { checkRoll, textOf } from '../../../../core/combat/contest-testing';
import { ContestRoll } from './contest-roll';

function show(roll: ReturnType<typeof checkRoll>, skill = '') {
  const fixture = TestBed.createComponent(ContestRoll);
  fixture.componentRef.setInput('roll', roll);
  fixture.componentRef.setInput('skill', skill);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('ContestRoll', () => {
  it('draws the d20 that counts in a box, the total and the formula, with the skill the roller chose', () => {
    const el = show(checkRoll(), 'Atletismo');
    expect(el.querySelector('.roll__box')?.textContent).toBe('15');
    expect(el.querySelector('.roll__total')?.textContent).toBe('20');
    expect(textOf(el.querySelector('.roll__formula')!)).toBe('1d20 (15) + 5 · Atletismo');
    // A screen reader hears the total once, in a status region.
    expect(el.querySelector('[role="status"]')).toBeTruthy();
    expect(textOf(el.querySelector('.mr-visually-hidden')!)).toBe(
      'Seu total: 20. 1d20 (15) + 5 · Atletismo.',
    );
    expect(el.querySelector('.why')).toBeNull();
  });

  it('says the dice were physical', () => {
    const el = show(checkRoll({ physical: true }));
    expect(textOf(el.querySelector('.roll__formula')!)).toBe('1d20 (15) + 5 · dado físico');
  });

  it('shows the pair, the one that counts marked "vale" and the other "descartado", and why (never colour alone)', () => {
    const el = show(
      checkRoll({
        faces: [8, 15],
        mode: RollModeKind.ADVANTAGE,
        total: 20,
        notes: [
          create(RollNoteSchema, { kind: 'help', labelPt: 'Ajuda de Orla', advantage: true }),
        ],
      }),
    );
    expect(el.querySelector('.roll__box')?.textContent).toBe('15');
    expect(textOf(el.querySelector('.why__mode')!)).toBe('Vantagem');
    expect(Array.from(el.querySelectorAll('.die')).map((d) => textOf(d))).toEqual([
      '8 descartado',
      '15 vale',
    ]);
    expect(textOf(el.querySelector('.why')!)).toContain('Vantagem: Ajuda de Orla');
  });

  it('says the master rolled when the player left the roll to them', () => {
    const el = show(checkRoll({ rolledByMaster: true }));
    expect(textOf(el)).toContain('O mestre rolou por você.');
  });
});
