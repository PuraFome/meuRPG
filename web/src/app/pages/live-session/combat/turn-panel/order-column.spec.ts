import { TestBed } from '@angular/core/testing';

import { CombatantKind, CombatantState } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { OrderColumn } from './order-column';

describe('OrderColumn', () => {
  it('numbers the order, marks the turn and says a word for each, never an NPC\'s numbers', () => {
    const fixture = TestBed.createComponent(OrderColumn);
    fixture.componentRef.setInput(
      'encounter',
      encounter({
        currentCombatantId: 'pen',
        combatants: [
          combatant({ id: 'brisa', label: 'Brisa', kind: CombatantKind.PLAYER }),
          combatant({ id: 'cap', label: 'Capitão Goblin', state: CombatantState.HURT }),
          combatant({ id: 'pen', label: 'Pensantus', kind: CombatantKind.PLAYER, mine: true }),
          combatant({ id: 'g1', label: 'Goblin 1', defeated: true, state: CombatantState.DEFEATED }),
        ],
      }),
    );
    fixture.detectChanges();
    const rows = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('.row'), (r) =>
      r.textContent?.replace(/\s+/g, ' ').trim(),
    );
    expect(rows).toHaveLength(4);
    expect(rows[0]).toMatch(/^1 .*Brisa Jogador$/);
    expect(rows[1]).toMatch(/Capitão Goblin Ferido$/);
    expect(rows[2]).toMatch(/Pensantus VezVocê$/);
    expect(rows[3]).toMatch(/^4.*Goblin 1 .*Derrotado$/);
    expect((fixture.nativeElement as HTMLElement).querySelector('.row--turn')?.textContent).toContain('Pensantus');
  });
});
