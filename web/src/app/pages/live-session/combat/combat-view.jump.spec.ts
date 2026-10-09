import { Code, ConnectError } from '@connectrpc/connect';
import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { provideRouter } from '@angular/router';
import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CombatantKind,
  CombatantSide,
  EncounterMode,
} from '../../../../gen/meurpg/play/v1/combat_pb';
import { TableRulesClient } from '../../../core/campaigns/table-rules';
import { CombatClient } from '../../../core/combat/combat-client';
import { CONNECT_TRANSPORT } from '../../../core/connect/transport';
import { CombatState } from '../../../core/combat/combat-state';
import { combatant, encounter } from '../../../core/combat/combat-testing';
import { CreaturesClient } from '../../../core/creatures/creatures-client';
import { MapState } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
import { RosterClient } from '../../../core/maps/roster-client';
import { SpellCatalog } from '../../../core/combat/spell-catalog';
import { CombatView } from './combat-view';

/** A combat view of Toren (the player's own character) over a transport that answers `answer(method, input)`. */
async function mountToren(answer: (method: string, input: unknown) => unknown) {
  TestBed.resetTestingModule();
  const toren = combatant({
    id: 't',
    label: 'Toren',
    kind: CombatantKind.PLAYER,
    side: CombatantSide.PARTY,
    characterId: 'toren-c',
    mine: true,
  });
  const state = new CombatState();
  const enc = encounter({
    mode: EncounterMode.THEATRE,
    mapId: '',
    gridColumns: 0,
    gridRows: 0,
    currentCombatantId: 't',
    combatants: [toren],
  });
  state.apply(enc);
  const transport = {
    unary: async (
      method: { name: string },
      _s: unknown,
      _t: unknown,
      _h: unknown,
      input: unknown,
    ) => {
      const message =
        answer(method.name, input) ?? (method.name === 'GetEncounter' ? { encounter: enc } : {});
      return {
        stream: false,
        service: {},
        method,
        header: new Headers(),
        trailer: new Headers(),
        message,
      };
    },
  };
  TestBed.configureTestingModule({
    providers: [
      provideRouter([]),
      { provide: CONNECT_TRANSPORT, useValue: transport },
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
  expect(TestBed.inject(CombatClient)).toBeTruthy();
  const fixture = TestBed.createComponent(CombatView);
  fixture.componentRef.setInput('campaignId', 'c');
  fixture.componentRef.setInput('isMaster', false);
  fixture.componentRef.setInput('state', state);
  fixture.componentRef.setInput('mapState', new MapState(async () => ({}) as never));
  fixture.componentRef.setInput('diceMode', DiceMode.PLAYERS_CHOOSE);
  fixture.componentRef.setInput('dicePreference', DicePreference.APP);
  fixture.detectChanges();
  await fixture.whenStable();
  return {
    enc,
    view: fixture.componentInstance as unknown as { confirmJump(r: unknown): Promise<void> },
  };
}

// A jump costs movement: a retry after a lost answer must be the same request to the server, not a second charge.
describe('CombatView: a jump', () => {
  it('after a lost answer (Unavailable) the second tap of the same high jump sends the same key', async () => {
    const keys: string[] = [];
    const { enc, view } = await mountToren((method, input) => {
      if (method !== 'MoveCombatant') {
        return undefined;
      }
      keys.push((input as { idempotencyKey: string }).idempotencyKey);
      if (keys.length === 1) {
        throw new ConnectError('lost answer', Code.Unavailable);
      }
      return { encounter: enc };
    });
    await view.confirmJump({ kind: 'high', heightDft: 18 });
    await view.confirmJump({ kind: 'high', heightDft: 18 });

    expect(keys).toHaveLength(2);
    expect(keys[1]).toBe(keys[0]);
  });

  it('"Saltar e desengajar" takes the Disengage action first and then jumps; a plain jump takes none', async () => {
    const calls: string[] = [];
    const { enc, view } = await mountToren((method, input) => {
      if (method === 'TakeAction') {
        calls.push(`action ${(input as { actionKey: string }).actionKey}`);
        return { encounter: enc };
      }
      if (method === 'MoveCombatant') {
        calls.push('move');
        return { encounter: enc };
      }
      return undefined;
    });
    await view.confirmJump({ kind: 'long', square: { col: 9, row: 3 }, disengage: true });
    expect(calls).toEqual(['action standard:disengage', 'move']);
    calls.length = 0;
    await view.confirmJump({ kind: 'long', square: { col: 9, row: 3 } });
    expect(calls).toEqual(['move']);
  });

  it('does not jump when the Disengage action was refused: the choice was never spent', async () => {
    const calls: string[] = [];
    const { view } = await mountToren((method) => {
      calls.push(method);
      if (method === 'TakeAction') {
        throw new ConnectError('already used', Code.FailedPrecondition);
      }
      return undefined;
    });
    await view.confirmJump({ kind: 'long', square: { col: 9, row: 3 }, disengage: true });
    expect(calls).not.toContain('MoveCombatant');
  });
});
