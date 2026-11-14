import { create } from '@bufbuild/protobuf';

import {
  CombatantKind,
  CombatantSide,
  EncounterMode,
  GetTurnOptionsResponseSchema,
} from '../../../../gen/meurpg/play/v1/combat_pb';
import { ActionEconomy, FeatureTargetWho } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { combatant, encounter } from '../../../core/combat/combat-testing';
import { settle } from '../../../../test-settle';
import { PHONE_QUERY } from '../../../shared/map-view/media-query';
import { mountCombatView } from './combat-view-testing';
import { UseFeatureSheet } from './use-feature-sheet/use-feature-sheet';

// A feature whose action declares a use (`via_use_feature`) is taken with `UseFeature`, never `TakeAction`: the player's own use
// with nothing to ask goes straight through, one that asks who or which and any use of the master opens the sheet.

const toren = () =>
  combatant({
    id: 't',
    label: 'Toren',
    kind: CombatantKind.PLAYER,
    side: CombatantSide.PARTY,
    characterId: 'toren-c',
    mine: true,
    placed: false,
  } as never);
const zombie = () => combatant({ id: 'z1', label: 'Zumbi 1', placed: false } as never);

const theatre = (currentCombatantId = 't', mine = true) =>
  encounter({
    mode: EncounterMode.THEATRE,
    mapId: '',
    gridColumns: 0,
    gridRows: 0,
    round: 2,
    currentCombatantId,
    combatants: [{ ...toren(), mine }, zombie()],
  } as never);

const used = (key: string, namePt: string, over: Record<string, unknown> = {}) => ({
  enabled: true,
  viaUseFeature: true,
  action: { key, namePt, economy: ActionEconomy.ACTION },
  ...over,
});

const turns =
  (featureActions: unknown[], featureTargets: unknown[] = []) =>
  (id: string) =>
    id === 't'
      ? create(GetTurnOptionsResponseSchema, {
          options: { featureActions },
          featureTargets,
        } as never)
      : undefined;

async function setup(opts: Parameters<typeof mountCombatView>[0]) {
  const m = mountCombatView(opts);
  await settle(m.fixture, 2);
  return m;
}

describe('CombatView: Usar a feature taken with UseFeature (#743)', () => {
  it('calls UseFeature at once for the own use that asks nothing, never TakeAction, and notes what it gave', async () => {
    const m = await setup({
      enc: theatre(),
      turnOptions: turns([used('feature:symbiotic', 'Entidade simbiótica')]),
      api: {
        useFeature: () => ({ encounter: theatre(), healed: 0, tempHp: 16 }),
      },
    });
    await m.view.useFeature('feature:symbiotic');
    expect(m.opened).toHaveLength(0);
    expect(m.callsOf('takeAction')).toHaveLength(0);
    expect(m.callsOf('useFeature')).toHaveLength(1);
    expect(m.callsOf('useFeature')[0].args).toEqual([
      {
        campaignId: 'c',
        encounterId: 'enc',
        combatantId: 't',
        actionKey: 'feature:symbiotic',
      },
      expect.any(String),
    ]);
    expect(m.view.actionNote()).toBe('Uso de Entidade simbiótica: 16 PV temporários.');
  });

  it("writes the server's refusal when the call fails and notes nothing", async () => {
    const m = await setup({
      enc: theatre(),
      turnOptions: turns([used('feature:symbiotic', 'Entidade simbiótica')]),
      api: {
        useFeature: () => {
          throw new Error('x');
        },
      },
    });
    await m.view.useFeature('feature:symbiotic');
    expect(m.view.actionNote()).toBe('');
    expect(m.view.error()).not.toBe('');
  });

  it('opens the sheet with the option and the creatures the server listed when the use asks who', async () => {
    const m = await setup({
      enc: theatre(),
      matches: [PHONE_QUERY],
      turnOptions: turns(
        [
          used('feature:halo', 'Halo de esporos', {
            targetMin: 1,
            targetMax: 1,
            targetWho: FeatureTargetWho.ENEMY,
            targetRangeFt: 10,
          }),
        ],
        [
          {
            actionKey: 'feature:halo',
            targets: [{ target: { combatantId: 'z1', label: 'Zumbi 1' } }],
          },
        ],
      ),
    });
    await m.view.useFeature('feature:halo');
    expect(m.opened).toHaveLength(1);
    expect(m.opened[0].component).toBe(UseFeatureSheet);
    expect(m.opened[0].config.ariaLabel).toBe('Halo de esporos');
    const data = m.opened[0].config.data;
    expect(data).toMatchObject({
      campaignId: 'c',
      encounterId: 'enc',
      combatantId: 't',
      userLabel: 'Toren',
      master: false,
    });
    expect((data['targets'] as { target: { label: string } }[]).map((t) => t.target.label)).toEqual(
      ['Zumbi 1'],
    );
    expect(m.callsOf('useFeature')).toHaveLength(0);
    expect(m.callsOf('takeAction')).toHaveLength(0);
  });

  it('opens the sheet to confirm for the master acting as the character, even when nothing is asked', async () => {
    const m = await setup({
      enc: theatre('t', false),
      master: true,
      turnOptions: turns([used('feature:symbiotic', 'Entidade simbiótica')]),
    });
    m.view.actFor.set('t');
    await settle(m.fixture, 2);
    await m.view.useFeature('feature:symbiotic');
    expect(m.opened[0].component).toBe(UseFeatureSheet);
    expect(m.opened[0].config.data).toMatchObject({ master: true, userLabel: 'Toren' });
    expect(m.callsOf('useFeature')).toHaveLength(0);
  });

  it('notes what the sheet answered when the use worked, and nothing when it was cancelled', async () => {
    const m = await setup({
      enc: theatre(),
      turnOptions: turns([
        used('feature:fix', 'Corrigir o tipo de dano', {
          choices: [{ key: 'fire', textPt: 'Fogo' }],
        }),
      ]),
    });
    await m.view.useFeature('feature:fix');
    m.opened[0].close();
    expect(m.view.actionNote()).toBe('');
    await m.view.useFeature('feature:fix');
    m.opened[1].close({ healed: 0, tempHp: 0 });
    expect(m.view.actionNote()).toBe('Uso de Corrigir o tipo de dano.');
  });
});
