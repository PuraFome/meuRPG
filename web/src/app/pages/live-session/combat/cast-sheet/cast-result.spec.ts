import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import { CombatantState, SpellCastSchema, SpellEffectKind, SpellEffectOutcome } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { castRows, effectSentence } from '../../../../core/combat/cast-flow';
import { CastResult } from './cast-result';

describe('CastResult for a spell that reads hit points (E8-03, state 3)', () => {
  const labels = new Map([
    ['g1', { label: 'Goblin 1', state: CombatantState.UNHURT }],
    ['cap', { label: 'Capitão Goblin', state: CombatantState.UNHURT }],
  ]);
  const cast = create(SpellCastSchema, {
    spellKey: 'spell:sleep',
    effectKind: SpellEffectKind.POOL,
    effectConditionKey: 'condition:unconscious',
    // The caster's own roll: the server sends it to them and to the master only.
    poolRoll: { diceCount: 5, diceSides: 8, faces: [2, 4, 1, 5, 3], modifier: 0, total: 15 },
    targets: [
      { combatantId: 'g1', effect: { outcome: SpellEffectOutcome.AFFECTED } },
      { combatantId: 'cap', effect: { outcome: SpellEffectOutcome.NOT_AFFECTED } },
    ],
  });

  function render(pool: string) {
    const fixture = TestBed.createComponent(CastResult);
    fixture.componentRef.setInput('rows', castRows(cast, new Map(), labels));
    fixture.componentRef.setInput('sentence', effectSentence(cast, labels, () => true));
    fixture.componentRef.setInput('pool', pool);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('says who fell asleep and who was not affected, in a live sentence, and the caster\'s own roll', () => {
    const el = render('5d8 (2, 4, 1, 5, 3) = 15');
    expect(el.querySelector('[aria-live]')).not.toBeNull();
    expect(el.querySelector('.sentence')!.textContent).toBe('O Goblin 1 adormeceu. O Capitão Goblin não foi afetado.');
    expect(el.querySelector('.pool__cap')!.textContent).toBe('Sua rolagem');
    expect(el.querySelector('.pool__roll')!.textContent).toBe('5d8 (2, 4, 1, 5, 3) = 15');
    // The icon's name, then the word: the colour carries nothing.
    const pills = [...el.querySelectorAll('.pill')].map((p) => [p.querySelector('mat-icon')!.textContent!.trim(), p.textContent!.replace(p.querySelector('mat-icon')!.textContent!, '').trim()]);
    expect(pills).toEqual([
      ['bedtime', 'Adormeceu'],
      ['block', 'Não foi afetado'],
    ]);
  });

  it('never shows an enemy\'s hit points or why someone was not affected', () => {
    const el = render('5d8 (2, 4, 1, 5, 3) = 15');
    expect(el.textContent).not.toMatch(/PV|restam|restantes|mais que/);
  });

  it('has no roll block when the caster has none to show', () => {
    expect(render('').querySelector('.pool')).toBeNull();
  });
});
