import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { provideRouter } from '@angular/router';

import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CombatantKind,
  CombatantSide,
  EncounterBlockedReason,
  EncounterBlockedSchema,
  EncounterMode,
} from '../../../../gen/meurpg/play/v1/combat_pb';
import { TableRulesClient } from '../../../core/campaigns/table-rules';
import { CombatClient } from '../../../core/combat/combat-client';
import { CombatState } from '../../../core/combat/combat-state';
import { combatant, encounter } from '../../../core/combat/combat-testing';
import { CreaturesClient } from '../../../core/creatures/creatures-client';
import { MapState } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
import { RosterClient } from '../../../core/maps/roster-client';
import { SpellCatalog } from '../../../core/combat/spell-catalog';
import { CombatView } from './combat-view';
import { anyCallCombatClient } from '../../../core/combat/combat-client-testing';

// The master's drag on the map of a running combat: "Movimento forçado" is for one drag.

interface Drag {
  forced: boolean;
}
interface Internals {
  moves: {
    forcedMove: { (): boolean; set(v: boolean): void };
    forcedNote: () => string;
    drop(d: { id: string; col: number; row: number }): Promise<void>;
    overAsk: () => { label: string; missing: string } | null;
    moveAnyway(): Promise<void>;
    dropOverAsk(): void;
  };
  error: () => string;
}

function setup(move: (args: unknown[]) => Promise<void>) {
  TestBed.resetTestingModule();
  const sent: Drag[] = [];
  const state = new CombatState();
  const e = encounter({
    mode: EncounterMode.GRID,
    currentCombatantId: 'p',
    combatants: [
      combatant({
        id: 'p',
        label: 'Pensantus',
        kind: CombatantKind.PLAYER,
        side: CombatantSide.PARTY,
        placed: true,
        col: 1,
        row: 1,
      }),
      combatant({
        id: 'b',
        label: 'Brisa',
        kind: CombatantKind.PLAYER,
        side: CombatantSide.PARTY,
        placed: true,
        col: 2,
        row: 2,
      }),
    ],
  });
  state.apply(e);
  const api = anyCallCombatClient((name) => {
    if (name === 'move') {
      return async (...a: unknown[]) => {
        sent.push({ forced: a[7] === true });
        await move(a);
        return { encounter: e, stoppedEarly: false, lockedDoor: false, provoked: false };
      };
    }
    if (name === 'log') {
      return async () => ({ rounds: [], undoableEventId: '' });
    }
    return async () => ({});
  });
  TestBed.configureTestingModule({
    providers: [
      provideRouter([]),
      { provide: CombatClient, useValue: api },
      { provide: RosterClient, useValue: { list: async () => [] } },
      { provide: MapsClient, useValue: { layers: async () => ({}) } },
      { provide: SpellCatalog, useValue: { details: async () => null } },
      {
        provide: CreaturesClient,
        useValue: {
          list: async () => [],
          statBlock: async () => null,
          summonOptions: async () => [],
        },
      },
      { provide: TableRulesClient, useValue: { get: async () => ({ saved: {} }) } },
      {
        provide: MatDialog,
        useValue: { open: () => ({ afterClosed: () => ({ subscribe: () => undefined }) }) },
      },
      {
        provide: MatBottomSheet,
        useValue: { open: () => ({ afterDismissed: () => ({ subscribe: () => undefined }) }) },
      },
    ],
  });
  const fixture = TestBed.createComponent(CombatView);
  fixture.componentRef.setInput('campaignId', 'c');
  fixture.componentRef.setInput('isMaster', true);
  fixture.componentRef.setInput('state', state);
  fixture.componentRef.setInput('mapState', new MapState(async () => ({}) as never));
  fixture.componentRef.setInput('diceMode', DiceMode.PLAYERS_CHOOSE);
  fixture.componentRef.setInput('dicePreference', DicePreference.APP);
  fixture.detectChanges();
  return { view: fixture.componentInstance as unknown as Internals, sent, fixture };
}

describe('CombatView: the master\'s "Movimento forçado"', () => {
  it('a plain drag sends forced as false', async () => {
    const { view, sent } = setup(async () => undefined);
    await view.moves.drop({ id: 'p', col: 5, row: 5 });
    expect(sent).toEqual([{ forced: false }]);
    expect(view.moves.forcedNote()).toBe('');
  });

  it('a drag with the box on sends forced, says it in words and switches the box off', async () => {
    const { view, sent } = setup(async () => undefined);
    view.moves.forcedMove.set(true);
    await view.moves.drop({ id: 'p', col: 5, row: 5 });
    expect(sent).toEqual([{ forced: true }]);
    expect(view.moves.forcedMove()).toBe(false);
    expect(view.moves.forcedNote()).toBe(
      'Pensantus foi movido à força. Ninguém recebeu oferta de ataque de oportunidade.',
    );
    await view.moves.drop({ id: 'p', col: 6, row: 6 });
    expect(sent[1]).toEqual({ forced: false });
    expect(view.moves.forcedNote()).toBe('');
  });

  it("agrees with the name's gender", async () => {
    const { view } = setup(async () => undefined);
    view.moves.forcedMove.set(true);
    await view.moves.drop({ id: 'b', col: 5, row: 5 });
    expect(view.moves.forcedNote()).toContain('Brisa foi movida à força.');
  });

  it('a refusal keeps the box on for the next try and says nothing of success', async () => {
    const { view } = setup(async () => {
      throw new Error('refused');
    });
    view.moves.forcedMove.set(true);
    await view.moves.drop({ id: 'p', col: 5, row: 5 });
    expect(view.moves.forcedMove()).toBe(true);
    expect(view.moves.forcedNote()).toBe('');
    expect(view.error()).not.toBe('');
  });
});

describe("CombatView: an NPC's drag past the movement it has left (B4)", () => {
  const tooFar = (missingDft: number) =>
    new ConnectError('too far', Code.FailedPrecondition, undefined, [
      {
        desc: EncounterBlockedSchema,
        value: create(EncounterBlockedSchema, {
          reason: EncounterBlockedReason.TOO_FAR,
          missingFt: Math.ceil(missingDft / 10),
          missingDft,
        }),
      },
    ]);

  it('asks "Mover mesmo assim" instead of an error, and the token goes back', async () => {
    const { view } = setup(async () => {
      throw tooFar(150);
    });
    await view.moves.drop({ id: 'p', col: 5, row: 5 });
    expect(view.error()).toBe('');
    const ask = view.moves.overAsk();
    expect(ask?.label).toBe('Pensantus');
    expect(ask?.missing.replace(/\u00a0/g, ' ')).toBe('4,5 m');
  });

  it('"Mover mesmo assim" sends the same drag forced, once, and the question goes', async () => {
    let first = true;
    const { view, sent } = setup(async () => {
      if (first) {
        first = false;
        throw tooFar(150);
      }
    });
    await view.moves.drop({ id: 'p', col: 5, row: 5 });
    await view.moves.moveAnyway();
    expect(sent).toEqual([{ forced: false }, { forced: true }]);
    expect(view.moves.overAsk()).toBeNull();
    expect(view.moves.forcedNote()).toContain('Pensantus foi movido à força');
  });

  it('"Cancelar" drops the question and sends nothing', async () => {
    const { view, sent } = setup(async () => {
      throw tooFar(150);
    });
    await view.moves.drop({ id: 'p', col: 5, row: 5 });
    view.moves.dropOverAsk();
    expect(view.moves.overAsk()).toBeNull();
    expect(sent).toHaveLength(1);
  });

  it('a refusal for another reason is still an error, not a question', async () => {
    const { view } = setup(async () => {
      throw new Error('refused');
    });
    await view.moves.drop({ id: 'p', col: 5, row: 5 });
    expect(view.moves.overAsk()).toBeNull();
    expect(view.error()).not.toBe('');
  });
});
