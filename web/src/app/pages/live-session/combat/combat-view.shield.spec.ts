// One Escudo sheet at a time: a second prompt waits for the first to close.
import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { provideRouter } from '@angular/router';
import { create } from '@bufbuild/protobuf';
import { Subject } from 'rxjs';
import { TurnOptionsSchema } from '../../../../gen/meurpg/rules/v1/rules_pb';
import {
  DeathSaveVisibility,
  DiceMode,
  DicePreference,
} from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CombatantKind, CombatantSide } from '../../../../gen/meurpg/play/v1/combat_pb';
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

const toren = combatant({
  id: 't',
  label: 'Toren',
  kind: CombatantKind.PLAYER,
  side: CombatantSide.PARTY,
  characterId: 'toren-c',
  mine: true,
});
const goblin = combatant({ id: 'g', label: 'Goblin' });
const prompt = (id: string) => ({ pendingDamageId: id, targetId: 't', slots: [], spellNamePt: '' });
const enc = (...ids: string[]) =>
  encounter({
    currentCombatantId: 'g',
    combatants: [toren, goblin],
    reactionPrompts: ids.map(prompt) as never,
  });

async function settle(fixture: { detectChanges: () => void; whenStable: () => Promise<unknown> }) {
  await fixture.whenStable();
  await new Promise((r) => setTimeout(r, 0));
  await fixture.whenStable();
  fixture.detectChanges();
}

describe('Escudo prompts never pile up', () => {
  it('opens the next prompt only after the open sheet closes', async () => {
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: /min-width: 1024px/.test(query),
      media: query,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      addListener: () => undefined,
      removeListener: () => undefined,
    }));
    const api = new Proxy(
      {},
      {
        get: (_t, name: string) =>
          name === 'turnOptions'
            ? async () => ({
                options: create(TurnOptionsSchema, {}),
                attackTargets: [],
                spellTargets: [],
                pendingDamages: [],
              })
            : name === 'log'
              ? async () => ({ rounds: [], undoableEventId: '' })
              : async () => ({}),
      },
    );
    const opened: string[] = [];
    const closers: Subject<boolean>[] = [];
    const state = new CombatState();
    state.apply(enc('p1'));
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
        {
          provide: TableRulesClient,
          useValue: {
            get: async () => ({ saved: { deathSaves: DeathSaveVisibility.VISIBLE_TO_ALL } }),
          },
        },
        {
          provide: MatDialog,
          useValue: {
            open: (_c: unknown, config: { data: { prompt: { pendingDamageId: string } } }) => {
              const closed = new Subject<boolean>();
              opened.push(config.data.prompt.pendingDamageId);
              closers.push(closed);
              return { afterClosed: () => closed };
            },
          },
        },
        { provide: MatBottomSheet, useValue: {} },
      ],
    });
    const fixture = TestBed.createComponent(CombatView);
    fixture.componentRef.setInput('campaignId', 'c');
    fixture.componentRef.setInput('isMaster', false);
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('mapState', new MapState(async () => ({}) as never));
    fixture.componentRef.setInput('diceMode', DiceMode.PLAYERS_CHOOSE);
    fixture.componentRef.setInput('dicePreference', DicePreference.APP);
    fixture.detectChanges();
    await settle(fixture);
    expect(opened).toEqual(['p1']);

    // A second hit arrives while the first sheet is still open: no second sheet.
    state.apply(enc('p1', 'p2'));
    await settle(fixture);
    expect(opened).toEqual(['p1']);

    // The first closes (answered): the waiting prompt opens, once.
    state.apply(enc('p2'));
    closers[0].next(false);
    await settle(fixture);
    expect(opened).toEqual(['p1', 'p2']);
  });
});
