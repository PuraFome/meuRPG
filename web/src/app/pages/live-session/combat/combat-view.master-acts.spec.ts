// The master acts for a player character on its turn (C8-01): "Agir por", the band, the character's economy and the end of its turn.
import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { provideRouter } from '@angular/router';
import { create } from '@bufbuild/protobuf';
import { TurnOptionsSchema } from '../../../../gen/meurpg/rules/v1/rules_pb';
import {
  DeathSaveVisibility,
  DiceMode,
  DicePreference,
} from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  type Combatant,
  CombatantKind,
  CombatantSide,
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
import { nextTurn } from '../../../../test-settle';
import { anyCallCombatClient } from '../../../core/combat/combat-client-testing';

// With --coverage test-setup.ts only hooks the first file of a worker, so the stubbed globals this file leaves on would
// reach the next file (a real setTimeout never fires on a fake clock).
afterEach(() => {
  vi.unstubAllGlobals();
});

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();
const pensantus = (over: Partial<Combatant> = {}) =>
  combatant({
    id: 'p',
    label: 'Pensantus',
    kind: CombatantKind.PLAYER,
    side: CombatantSide.PARTY,
    characterId: 'pensantus-c',
    placed: false,
    ...over,
  });
const goblin = combatant({ id: 'g', label: 'Goblin', placed: false });
const enc = (turn: string) =>
  encounter({
    mode: EncounterMode.THEATRE,
    mapId: '',
    gridColumns: 0,
    gridRows: 0,
    currentCombatantId: turn,
    combatants: [pensantus(), goblin],
  });

async function settle(fixture: { detectChanges: () => void; whenStable: () => Promise<unknown> }) {
  await fixture.whenStable();
  await nextTurn();
  await fixture.whenStable();
  fixture.detectChanges();
}

async function mount(first: ReturnType<typeof enc>) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: /min-width: 1024px/.test(query),
    media: query,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    addListener: () => undefined,
    removeListener: () => undefined,
  }));
  const turnsEnded: string[] = [];
  const asked: string[] = [];
  const api = anyCallCombatClient((name) => {
    switch (name) {
      case 'turnOptions':
        return async (_c: string, _e: string, who: string) => {
          asked.push(who);
          return {
            options: create(TurnOptionsSchema, {}),
            attackTargets: [],
            spellTargets: [],
            pendingDamages: [],
          };
        };
      case 'log':
        return async () => ({ rounds: [], undoableEventId: '' });
      case 'endTurn':
        return async (_c: string, _e: string, who: string) => {
          turnsEnded.push(who);
          return enc('g');
        };
      default:
        return async () => ({});
    }
  });
  const state = new CombatState();
  state.apply(first);
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
  await settle(fixture);
  return { fixture, state, el: fixture.nativeElement as HTMLElement, turnsEnded, asked };
}

const button = (el: HTMLElement, text: string) =>
  [...el.querySelectorAll('button')].find((b) => plain(b.textContent).includes(text)) as
    HTMLButtonElement | undefined;

describe('the master acts for a player character', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('shows the character\'s card with "Agir por Pensantus" on its turn', async () => {
    const { el } = await mount(enc('p'));
    expect(plain(el.querySelector('[data-testid="act-as-card"] h2')?.textContent)).toBe(
      'Vez do Pensantus',
    );
    expect(button(el, 'Agir por Pensantus')).toBeDefined();
    expect(el.querySelector('[data-testid="acting-band"]')).toBeNull();
  });

  it('has no "Agir por" on an NPC\'s turn', async () => {
    const { el } = await mount(enc('g'));
    expect(button(el, 'Agir por')).toBeUndefined();
  });

  it('acting: the band, "Voltar ao mestre" and one end-of-turn button named for the character', async () => {
    const { fixture, el } = await mount(enc('p'));
    button(el, 'Agir por Pensantus')!.click();
    await settle(fixture);
    expect(plain(el.querySelector('[data-testid="acting-band"]')?.textContent)).toContain(
      'Você está agindo por Pensantus',
    );
    expect(button(el, 'Voltar ao mestre')).toBeDefined();
    expect(button(el, 'Próximo turno')).toBeUndefined();
    expect(el.querySelectorAll('button').length).toBeGreaterThan(0);
    expect(
      [...el.querySelectorAll('button')].filter((b) =>
        plain(b.textContent).includes('Encerrar o turno de Pensantus'),
      ),
    ).toHaveLength(1);
    expect(plain(el.querySelector('[data-testid="acting-chip"]')?.textContent)).toContain(
      'Você age por Pensantus',
    );
  });

  it("ends the character's own turn with its id", async () => {
    const { fixture, el, turnsEnded } = await mount(enc('p'));
    button(el, 'Agir por Pensantus')!.click();
    await settle(fixture);
    button(el, 'Encerrar o turno de Pensantus')!.click();
    await settle(fixture);
    expect(turnsEnded).toEqual(['p']);
  });

  it('"Voltar ao mestre" gives the rail back without ending the turn', async () => {
    const { fixture, el, turnsEnded } = await mount(enc('p'));
    button(el, 'Agir por Pensantus')!.click();
    await settle(fixture);
    button(el, 'Voltar ao mestre')!.click();
    await settle(fixture);
    expect(el.querySelector('[data-testid="acting-band"]')).toBeNull();
    expect(button(el, 'Agir por Pensantus')).toBeDefined();
    expect(turnsEnded).toEqual([]);
  });

  it('stops acting when the turn passes', async () => {
    const { fixture, el, state } = await mount(enc('p'));
    button(el, 'Agir por Pensantus')!.click();
    await settle(fixture);
    state.apply(enc('g'));
    await settle(fixture);
    expect(el.querySelector('[data-testid="acting-band"]')).toBeNull();
    state.apply(enc('p'));
    await settle(fixture);
    expect(el.querySelector('[data-testid="acting-band"]')).toBeNull();
    expect(button(el, 'Agir por Pensantus')).toBeDefined();
  });

  it("asks for the character's own turn options while acting", async () => {
    const { asked } = await mount(enc('p'));
    expect(asked).toContain('p');
  });
});
