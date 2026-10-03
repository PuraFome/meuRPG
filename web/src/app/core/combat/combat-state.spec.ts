import { EncounterStatus } from '../../../gen/meurpg/play/v1/combat_pb';
import { CombatState } from './combat-state';
import { combatant, encounter } from './combat-testing';

describe('CombatState', () => {
  const two = [combatant({ id: 'a', label: 'A', col: 1, row: 1 }), combatant({ id: 'b', label: 'B' })];

  it('keeps the newer of two copies of the same combat', () => {
    const state = new CombatState();
    state.apply(encounter({ revision: 5, round: 3 }));
    state.apply(encounter({ revision: 4, round: 2 }));
    expect(state.encounter()?.round).toBe(3);
    state.apply(encounter({ revision: 6, round: 4 }));
    expect(state.encounter()?.round).toBe(4);
  });

  it('takes another combat whatever its revision', () => {
    const state = new CombatState();
    state.apply(encounter({ id: 'old', revision: 9 }));
    state.apply(encounter({ id: 'new', revision: 1 }));
    expect(state.encounter()?.id).toBe('new');
  });

  it('applies turn_changed in place', () => {
    const state = new CombatState();
    state.apply(encounter({ combatants: two, currentCombatantId: 'a' }));
    expect(
      state.applyTurn({ encounterId: 'enc', round: 3, currentCombatantId: '', masterTurn: true }),
    ).toBe(true);
    expect(state.encounter()).toMatchObject({ round: 3, currentCombatantId: '', masterTurn: true });
  });

  it('asks to read again when the turn or the move is about something unknown', () => {
    const state = new CombatState();
    expect(state.applyTurn({ encounterId: 'enc', round: 1, currentCombatantId: 'a', masterTurn: false })).toBe(false);
    state.apply(encounter({ combatants: two }));
    expect(state.applyTurn({ encounterId: 'other', round: 1, currentCombatantId: 'a', masterTurn: false })).toBe(false);
    expect(state.applyMove({ encounterId: 'enc', combatantId: 'ghost', col: 1, row: 1 })).toBe(false);
  });

  it('applies combatant_moved in place and marks the combatant placed', () => {
    const state = new CombatState();
    state.apply(encounter({ combatants: [combatant({ id: 'a', label: 'A', placed: false })] }));
    expect(state.applyMove({ encounterId: 'enc', combatantId: 'a', col: 7, row: 8 })).toBe(true);
    expect(state.encounter()?.combatants[0]).toMatchObject({ placed: true, col: 7, row: 8 });
  });

  it('hides an ended combat once the person left it, and shows a new one', () => {
    const state = new CombatState();
    state.apply(encounter({ status: EncounterStatus.ENDED }));
    expect(state.shown()).not.toBeNull();
    state.dismissEnded();
    expect(state.shown()).toBeNull();
    state.apply(encounter({ id: 'next', status: EncounterStatus.SETUP }));
    expect(state.shown()?.id).toBe('next');
  });
});
