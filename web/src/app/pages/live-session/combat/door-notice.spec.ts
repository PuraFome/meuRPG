import { Injector, computed, runInInjectionContext, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { type CombatLogRound, CombatLogKind } from '../../../../gen/meurpg/play/v1/combat_pb';
import { noticeOpenedDoors } from './door-notice';

const door = (id: string, actorId: string) => ({ id, kind: CombatLogKind.DOOR_OPENED, actorId });
const round = (...entries: object[]) => [{ round: 1, entries }] as unknown as CombatLogRound[];

describe('noticeOpenedDoors', () => {
  function setup() {
    const rounds = signal<readonly CombatLogRound[]>([]);
    const loaded = signal(false);
    const own = signal<string | undefined>('pen');
    const say = vi.fn();
    const combatant = computed(() => (own() ? { id: own()! } : undefined));
    runInInjectionContext(TestBed.inject(Injector), () =>
      noticeOpenedDoors({ rounds, loaded }, combatant, say),
    );
    const tick = () => TestBed.tick();
    return { rounds, loaded, own, say, tick };
  }

  it('does not say what the first read of the log already held', () => {
    const { rounds, loaded, say, tick } = setup();
    rounds.set(round(door('d0', 'pen')));
    loaded.set(true);
    tick();
    expect(say).not.toHaveBeenCalled();
  });

  it("says once when a later read has a door the player's own combatant opened", () => {
    const { rounds, loaded, say, tick } = setup();
    loaded.set(true);
    tick();
    rounds.set(round(door('d1', 'pen')));
    tick();
    expect(say).toHaveBeenCalledTimes(1);
    rounds.set(round(door('d1', 'pen')));
    tick();
    expect(say).toHaveBeenCalledTimes(1);
  });

  it('says nothing of a door somebody else opened, or for a master with no combatant', () => {
    const { rounds, loaded, own, say, tick } = setup();
    loaded.set(true);
    tick();
    rounds.set(round(door('d1', 'bri')));
    tick();
    expect(say).not.toHaveBeenCalled();
    own.set(undefined);
    rounds.set(round(door('d1', 'bri'), door('d2', 'pen')));
    tick();
    expect(say).not.toHaveBeenCalled();
  });
});
