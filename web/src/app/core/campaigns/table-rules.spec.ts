import { TestBed } from '@angular/core/testing';
import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import {
  CriticalRule,
  DeathSaveVisibility,
  DiceMode,
  HiddenAreaHitRule,
  HitPointsRule,
  TableStyle,
  TableStylePresetSchema,
  XpModeChangeBlockedSchema,
} from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import {
  type RulesDraft,
  TableRulesClient,
  applyPreset,
  changeCount,
  draftFromRules,
  draftProblem,
  styleOf,
  xpModeBlocked,
} from './table-rules';

const presets = [
  create(TableStylePresetSchema, {
    style: TableStyle.TUDO_NO_APP,
    diceMode: DiceMode.APP,
    combatStartsWithMap: true,
    fogOnNewMaps: true,
  }),
  create(TableStylePresetSchema, {
    style: TableStyle.MESA_FISICA,
    diceMode: DiceMode.PHYSICAL,
    combatStartsWithMap: false,
    fogOnNewMaps: false,
  }),
  create(TableStylePresetSchema, {
    style: TableStyle.TEATRO_DA_MENTE,
    diceMode: DiceMode.PLAYERS_CHOOSE,
    combatStartsWithMap: false,
    fogOnNewMaps: false,
  }),
];

const base: RulesDraft = {
  diceMode: DiceMode.PLAYERS_CHOOSE,
  combatStartsWithMap: true,
  fogOnNewMaps: true,
  hitPoints: HitPointsRule.PLAYER_CHOOSES,
  standardArray: true,
  pointBuy: true,
  rolled4d6: true,
  typed: false,
  critical: CriticalRule.DOUBLED_DICE,
  deathSaves: DeathSaveVisibility.VISIBLE_TO_ALL,
  hiddenAreaHits: HiddenAreaHitRule.REVEAL,
  houseRules: [],
};

describe('table rules', () => {
  it('works out the style from the three settings the presets own, and is "Personalizado" when none matches', () => {
    expect(styleOf(base, presets)).toBe(TableStyle.PERSONALIZADO);
    expect(styleOf({ ...base, diceMode: DiceMode.APP }, presets)).toBe(TableStyle.TUDO_NO_APP);
    expect(
      styleOf(
        { ...base, diceMode: DiceMode.PHYSICAL, combatStartsWithMap: false, fogOnNewMaps: false },
        presets,
      ),
    ).toBe(TableStyle.MESA_FISICA);
  });

  it('applies a preset by copying its three values and nothing else', () => {
    const next = applyPreset(
      { ...base, hitPoints: HitPointsRule.AVERAGE, critical: CriticalRule.MAX_PLUS_ROLL },
      presets[1],
    );
    expect(next.diceMode).toBe(DiceMode.PHYSICAL);
    expect(next.combatStartsWithMap).toBe(false);
    expect(next.fogOnNewMaps).toBe(false);
    expect(next.hitPoints).toBe(HitPointsRule.AVERAGE);
    expect(next.critical).toBe(CriticalRule.MAX_PLUS_ROLL);
  });

  it('counts the choices that differ from what is saved, a house rule each', () => {
    expect(changeCount(base, base)).toBe(0);
    expect(changeCount({ ...base, fogOnNewMaps: false, combatStartsWithMap: false }, base)).toBe(2);
    expect(changeCount({ ...base, houseRules: ['a', 'b'] }, { ...base, houseRules: ['a'] })).toBe(
      1,
    );
    expect(changeCount({ ...base, houseRules: ['x'] }, { ...base, houseRules: ['a'] })).toBe(1);
  });

  it('refuses no method at all, and an empty house rule', () => {
    expect(draftProblem(base)).toBe('');
    expect(
      draftProblem({
        ...base,
        standardArray: false,
        pointBuy: false,
        rolled4d6: false,
        typed: false,
      }),
    ).toBe('Marque pelo menos um jeito de fazer as habilidades.');
    expect(draftProblem({ ...base, houseRules: ['ok', '  '] })).toBe(
      'Escreva o lembrete ou remova a linha vazia.',
    );
  });

  it('reads the defaults of a campaign that never saved its rules', () => {
    const d = draftFromRules(undefined);
    expect(d.hitPoints).toBe(HitPointsRule.PLAYER_CHOOSES);
    expect(d.critical).toBe(CriticalRule.DOUBLED_DICE);
    expect(d.combatStartsWithMap).toBe(true);
    expect(d.fogOnNewMaps).toBe(false);
    expect(d.houseRules).toEqual([]);
    // Hidden creatures an area hits appear unless the table chose otherwise.
    expect(d.hiddenAreaHits).toBe(HiddenAreaHitRule.REVEAL);
  });

  it('keeps the rule on hidden creatures an area hits, and counts its change', () => {
    expect(draftFromRules({ hiddenAreaHits: HiddenAreaHitRule.ASK } as never).hiddenAreaHits).toBe(
      HiddenAreaHitRule.ASK,
    );
    expect(changeCount({ ...base, hiddenAreaHits: HiddenAreaHitRule.KEEP_HIDDEN }, base)).toBe(1);
  });

  it('reads how much XP stood in the way from the typed detail, never from the message', () => {
    const detail = create(XpModeChangeBlockedSchema, { awards: 3, totalXp: 2716n });
    const blocked = new ConnectError('qualquer coisa', Code.FailedPrecondition, undefined, [
      { desc: XpModeChangeBlockedSchema, value: detail },
    ]);
    expect(xpModeBlocked(blocked)).toEqual({ awards: 3, totalXp: 2716 });
    expect(xpModeBlocked(new ConnectError('x', Code.FailedPrecondition))).toBeNull();
    expect(xpModeBlocked(new ConnectError('x', Code.Internal))).toBeNull();
  });

  it('sends the rule on hidden creatures with every save (the server requires it)', async () => {
    TestBed.configureTestingModule({
      providers: [TableRulesClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
    });
    const client = TestBed.inject(TableRulesClient);
    const sent: { rules: Record<string, unknown> }[] = [];
    (client as unknown as { client: unknown }).client = {
      setTableRules: (req: { rules: Record<string, unknown> }) => {
        sent.push(req);
        return Promise.resolve({ rules: req.rules, style: TableStyle.PERSONALIZADO });
      },
    };
    const res = await client.set('c', { ...base, hiddenAreaHits: HiddenAreaHitRule.ASK });
    expect(sent[0].rules['hiddenAreaHits']).toBe(HiddenAreaHitRule.ASK);
    expect(res.saved.hiddenAreaHits).toBe(HiddenAreaHitRule.ASK);
  });
});
