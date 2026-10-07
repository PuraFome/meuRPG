import { TestBed } from '@angular/core/testing';

import { CombatantKind, CombatantState } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { OrderColumn } from './order-column';

describe('OrderColumn', () => {
  it("numbers the order, marks the turn and says a word for each, never an NPC's numbers", () => {
    const fixture = TestBed.createComponent(OrderColumn);
    fixture.componentRef.setInput(
      'encounter',
      encounter({
        currentCombatantId: 'pen',
        combatants: [
          combatant({ id: 'brisa', label: 'Brisa', kind: CombatantKind.PLAYER }),
          combatant({ id: 'cap', label: 'Capitão Goblin', state: CombatantState.HURT }),
          combatant({ id: 'pen', label: 'Pensantus', kind: CombatantKind.PLAYER, mine: true }),
          combatant({
            id: 'g1',
            label: 'Goblin 1',
            defeated: true,
            state: CombatantState.DEFEATED,
          }),
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
    expect(
      (fixture.nativeElement as HTMLElement).querySelector('.row--turn')?.textContent,
    ).toContain('Pensantus');
  });

  it('says a creature of another player with only its owner and a state word, and the concentration to everyone', () => {
    const fixture = TestBed.createComponent(OrderColumn);
    fixture.componentRef.setInput(
      'encounter',
      encounter({
        currentCombatantId: 'tor',
        combatants: [
          combatant({
            id: 'sal',
            label: 'Sálvia',
            kind: CombatantKind.PLAYER,
            characterId: 'sc',
            concentrationSpell: 'spell:conjure-animals',
            concentrationSpellNamePt: 'Conjurar Animais',
          }),
          combatant({
            id: 'w1',
            label: 'Lobo atroz 1',
            kind: CombatantKind.CREATURE,
            characterId: '',
            ownerCharacterId: 'sc',
            state: CombatantState.HURT,
          }),
          combatant({ id: 'tor', label: 'Toren', kind: CombatantKind.PLAYER, mine: true }),
        ],
      }),
    );
    fixture.detectChanges();
    const rows = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('.row'), (r) =>
      r.textContent?.replace(/\s+/g, ' ').trim(),
    );
    expect(rows[0]).toContain('Concentração');
    // No numbers of its hit points, armor class or actions: the owner and a word.
    expect(rows[1]).toMatch(/Lobo atroz 1 .*da Sálvia · Ferido/);
    expect(rows[1]).not.toMatch(/PV|CA|\d+ de \d+/);
  });

  it("says another player's druid is a wolf with the state word, never the beast's pool (RN-20)", () => {
    const fixture = TestBed.createComponent(OrderColumn);
    fixture.componentRef.setInput(
      'encounter',
      encounter({
        currentCombatantId: 'tor',
        combatants: [
          combatant({
            id: 'sal',
            label: 'Sálvia',
            kind: CombatantKind.PLAYER,
            wildShapeBeastKey: 'monster:wolf',
            wildShapeBeastNamePt: 'Lobo',
          }),
          combatant({ id: 'tor', label: 'Toren', kind: CombatantKind.PLAYER, mine: true }),
        ],
      }),
    );
    fixture.detectChanges();
    const rows = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('.row'), (r) =>
      r.textContent?.replace(/\s+/g, ' ').trim(),
    );
    expect(rows[0]).toContain('Na forma de Lobo');
    expect(rows[0]).toContain('Jogador');
    expect(rows[0]).not.toMatch(/\d+ de \d+/);
  });
});
