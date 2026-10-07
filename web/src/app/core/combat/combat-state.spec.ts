import { CombatantKind, EncounterStatus } from '../../../gen/meurpg/play/v1/combat_pb';
import { CombatState } from './combat-state';
import { combatant, encounter } from './combat-testing';
import { turnBanner } from './combat-view';
import { acts, turnMembers } from './joint-turn';

describe('CombatState', () => {
  const two = [
    combatant({ id: 'a', label: 'A', col: 1, row: 1 }),
    combatant({ id: 'b', label: 'B' }),
  ];

  it('lets a copy with a lower revision replace the one on screen (a fog player counts what they saw)', () => {
    const state = new CombatState();
    state.apply(encounter({ revision: 40, round: 2 }));
    state.apply(encounter({ revision: 3, round: 3 }));
    expect(state.encounter()?.round).toBe(3);
  });

  it('drops a read that started before a later read was applied', () => {
    const state = new CombatState();
    const first = state.beginRead();
    const second = state.beginRead();
    expect(state.applyRead(second, encounter({ round: 4 }))).toBe(true);
    expect(state.applyRead(first, encounter({ round: 3 }))).toBe(false);
    expect(state.encounter()?.round).toBe(4);
  });

  it('applies a read that started later even when an earlier one answers after it', () => {
    const state = new CombatState();
    const first = state.beginRead();
    const second = state.beginRead();
    expect(state.applyRead(first, encounter({ round: 3 }))).toBe(true);
    expect(state.applyRead(second, encounter({ round: 4 }))).toBe(true);
    expect(state.encounter()?.round).toBe(4);
  });

  it('drops a read that started before an answer to a call was applied', () => {
    const state = new CombatState();
    state.apply(encounter({ round: 2 }));
    const ticket = state.beginRead();
    state.apply(encounter({ round: 3 }));
    expect(state.applyRead(ticket, encounter({ round: 2 }))).toBe(false);
    expect(state.encounter()?.round).toBe(3);
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
    expect(
      state.applyTurn({ encounterId: 'enc', round: 1, currentCombatantId: 'a', masterTurn: false }),
    ).toBe(false);
    state.apply(encounter({ combatants: two }));
    expect(
      state.applyTurn({
        encounterId: 'other',
        round: 1,
        currentCombatantId: 'a',
        masterTurn: false,
      }),
    ).toBe(false);
    expect(state.applyMove({ encounterId: 'enc', combatantId: 'ghost', col: 1, row: 1 })).toBe(
      false,
    );
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

  describe('turn_changed on a joint turn', () => {
    const P = CombatantKind.PLAYER;
    const brisa = combatant({ id: 'b', label: 'Brisa', kind: P, initiative: 19 });
    const toren = combatant({ id: 't', label: 'Toren', kind: P, initiative: 19 });
    const pens = combatant({ id: 'p', label: 'Pensantus', kind: P, initiative: 14, mine: true });

    function onJointTurn(): CombatState {
      const state = new CombatState();
      state.apply(
        encounter({
          combatants: [brisa, toren, pens],
          currentCombatantId: 'b',
          turnGroupIds: ['b', 't'],
          revision: 5,
        }),
      );
      return state;
    }

    it('names the new current combatant as the turn, not the group that ended', () => {
      const state = onJointTurn();
      state.applyTurn({ encounterId: 'enc', round: 2, currentCombatantId: 'p', masterTurn: false });
      expect(turnMembers(state.encounter()!).map((c) => c.id)).toEqual(['p']);
    });

    it('lets the next player act, and the banner names them, before any reload', () => {
      const state = onJointTurn();
      state.applyTurn({ encounterId: 'enc', round: 2, currentCombatantId: 'p', masterTurn: false });
      const e = state.encounter()!;
      expect(acts(e, pens)).toBe(true);
      expect(acts(e, brisa)).toBe(false);
      expect(turnBanner(e).title).toBe('Sua vez, Pensantus');
    });
  });
});
