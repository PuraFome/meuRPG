import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { provideRouter } from '@angular/router';

import { DiceMode, DicePreference } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CharacterKind } from '../../../gen/meurpg/characters/v1/characters_pb';
import { EncounterMode } from '../../../gen/meurpg/play/v1/combat_pb';
import { CampaignsService } from '../../core/campaigns/campaigns.service';
import { TableRulesClient } from '../../core/campaigns/table-rules';
import { CombatClient } from '../../core/combat/combat-client';
import { RosterClient } from '../../core/maps/roster-client';
import { nextTurn } from '../../../test-settle';
import { type CombatMapInfo, StartCombatDialog, type StartCombatData } from './start-combat-dialog';

const gridMap: CombatMapInfo = {
  id: 'map',
  name: 'Estrada',
  image: { url: '/i', width: 100, height: 100 },
  columns: 20,
  rows: 14,
};
const roster = [
  { id: 'p', name: 'Pensantus', kind: CharacterKind.PLAYER, playerUserId: 'u1', raceName: 'Gnomo' },
  { id: 'g', name: 'Goblin', kind: CharacterKind.MINION, playerUserId: '', raceName: '' },
];

/** The members of the dialog the template does not show, read straight from the instance. */
interface Peek {
  blocked(): string | null;
  state(): string;
  name: { set(v: string): void };
  included: { set(v: ReadonlySet<string>): void };
  picked: { set(v: readonly { key: string; namePt: string; count: number }[]): void };
  chooseMode(mode: EncounterMode): void;
  setCount(id: string, count: number): void;
  npcSub(n: { id: string; name: string; kind: CharacterKind }): string;
}

async function setup(data: StartCombatData, settled = true) {
  TestBed.configureTestingModule({
    providers: [
      provideRouter([]),
      { provide: MAT_DIALOG_DATA, useValue: data },
      { provide: MatDialogRef, useValue: { close: () => undefined } },
      { provide: RosterClient, useValue: { list: async () => roster } },
      {
        provide: CampaignsService,
        useValue: {
          listMembers: async () => ({ members: [] }),
          getCampaign: async () => ({
            campaign: { diceMode: DiceMode.PLAYERS_CHOOSE, dicePreference: DicePreference.APP },
          }),
        },
      },
      {
        provide: TableRulesClient,
        useValue: { get: async () => ({ saved: { combatStartsWithMap: true } }) },
      },
      { provide: CombatClient, useValue: {} },
    ],
  });
  const fixture = TestBed.createComponent(StartCombatDialog);
  fixture.detectChanges();
  if (settled) {
    await fixture.whenStable();
    await nextTurn();
    await fixture.whenStable();
    fixture.detectChanges();
  }
  return fixture.componentInstance as unknown as Peek;
}

const start = (map: CombatMapInfo | null): StartCombatData => ({
  campaignId: 'c',
  mode: 'start',
  map,
});

describe('StartCombatDialog: why the main button waits', () => {
  it('waits for the roster while it loads', async () => {
    const d = await setup(start(gridMap), false);
    expect(d.state()).toBe('loading');
    expect(d.blocked()).toBe('Carregando quem pode lutar.');
  });

  it('is free to go with a map, a name and the party', async () => {
    expect((await setup(start(gridMap))).blocked()).toBeNull();
  });

  it('asks for the session map when "Com mapa" is chosen with none', async () => {
    const d = await setup(start(null));
    d.chooseMode(EncounterMode.GRID);
    expect(d.blocked()).toBe('Escolha o mapa atual da sessão primeiro.');
  });

  it('asks for a grid when the map has none and "Com mapa" is chosen', async () => {
    const d = await setup(start({ ...gridMap, columns: 0, rows: 0 }));
    d.chooseMode(EncounterMode.GRID);
    expect(d.blocked()).toBe('O mapa precisa de uma grade.');
  });

  it('asks for a name, within 80 characters', async () => {
    const d = await setup(start(gridMap));
    d.name.set('   ');
    expect(d.blocked()).toBe('Dê um nome ao combate.');
    d.name.set('x'.repeat(81));
    expect(d.blocked()).toBe('Dê um nome ao combate.');
    d.name.set('x'.repeat(80));
    expect(d.blocked()).toBeNull();
  });

  it('asks who of the party enters, unless a monster is picked', async () => {
    const d = await setup(start(gridMap));
    d.included.set(new Set());
    expect(d.blocked()).toBe('Escolha quem do grupo entra.');
    d.picked.set([{ key: 'k', namePt: 'Bandido', count: 1 }]);
    expect(d.blocked()).toBeNull();
  });

  it('in add mode asks how many NPCs enter, and accepts a picked monster', async () => {
    const d = await setup({
      campaignId: 'c',
      mode: 'add',
      map: null,
      encounterId: 'e',
      existing: 2,
    });
    expect(d.blocked()).toBe('Escolha quantos NPCs entram.');
    d.picked.set([{ key: 'k', namePt: 'Bandido', count: 1 }]);
    expect(d.blocked()).toBeNull();
  });

  it('refuses more than 40 combatants, counting the ones already there', async () => {
    const d = await setup({
      campaignId: 'c',
      mode: 'add',
      map: null,
      encounterId: 'e',
      existing: 10,
    });
    d.setCount('g', 30);
    expect(d.blocked()).toBeNull();
    d.setCount('g', 31);
    expect(d.blocked()).toBe('Um combate tem no máximo 40 combatentes.');
  });

  it('says what an NPC becomes: "Goblin 1 e 2", "Goblin 1, 2 e 3"', async () => {
    const d = await setup(start(gridMap));
    const goblin = { id: 'g', name: 'Goblin', kind: CharacterKind.MINION };
    expect(d.npcSub(goblin)).not.toContain('vira');
    d.setCount('g', 1);
    expect(d.npcSub(goblin)).not.toContain('vira');
    d.setCount('g', 2);
    expect(d.npcSub(goblin)).toContain('vira Goblin 1 e 2');
    d.setCount('g', 3);
    expect(d.npcSub(goblin)).toContain('vira Goblin 1, 2 e 3');
  });
});
