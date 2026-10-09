import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import {
  CombatantEffectSchema,
  CombatantStateKind,
} from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { OrderList } from './order-list';

describe('OrderList: states', () => {
  it('shows the state chips of a row, with who marked the target', () => {
    const fixture = TestBed.createComponent(OrderList);
    fixture.componentRef.setInput(
      'encounter',
      encounter({
        currentCombatantId: 'g',
        combatants: [
          combatant({
            id: 'g',
            label: 'Goblin',
            states: [
              create(CombatantEffectSchema, {
                id: 'm',
                kind: CombatantStateKind.HUNTERS_MARK_TARGET,
                labelPt: 'Marcado',
                effectPt: 'Dano extra de quem marcou.',
                sourceLabel: 'Brisa',
              }),
              create(CombatantEffectSchema, {
                id: 'd',
                kind: CombatantStateKind.DODGING,
                labelPt: 'Esquivando',
                effectPt: 'Ataques contra ele têm desvantagem.',
              }),
            ],
          }),
        ],
      }),
    );
    fixture.detectChanges();
    const chips = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll('.row button.tag__btn'),
      (b) => b.textContent?.trim(),
    );
    expect(chips).toEqual(['Marcado por Brisa', 'Esquivando']);
  });
});
