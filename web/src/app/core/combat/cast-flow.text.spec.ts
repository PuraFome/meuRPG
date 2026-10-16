import { create } from '@bufbuild/protobuf';

import {
  AttackOutcome,
  CombatantState,
  type PendingDamage,
  PendingDamageStatus,
  SaveOutcome,
  SpellCastSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  Ability,
  ActionEconomy,
  SpellHitPointEffectKind,
  SpellRangeKind,
  SpellSaveSuccess,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { castRows, castSubtitle, slotRows, type SpellKind } from './cast-flow';

const plain = (s: string) => s.replace(/ /g, ' ');

describe('the subtitle of the cast sheet, part by part', () => {
  const fire = {
    damageTypePt: 'fogo',
    damageTypeKey: 'fire',
    bySlotLevel: { 1: '3d6', 2: '5d6' },
    byCharacterLevel: {},
  };
  const spell = (over: object = {}) =>
    ({
      spell: { level: 1 },
      range: { kind: SpellRangeKind.RANGED, distanceFt: 120 },
      damage: [fire],
      healBySlotLevel: {},
      ...over,
    }) as never;
  const sub = (
    economy: ActionEconomy,
    kind: SpellKind,
    details: never | null,
    slot = 1,
    darts = 0,
    shape = '',
  ) => plain(castSubtitle(economy, kind, details, slot, darts, '', '', shape));

  it('says the economy and the reach of the spell', () => {
    expect(sub(ActionEconomy.ACTION, 'plain', spell())).toBe('Ação · alcance 36 m');
    expect(sub(ActionEconomy.BONUS_ACTION, 'plain', spell())).toBe('Ação bônus · alcance 36 m');
    expect(
      sub(ActionEconomy.REACTION, 'plain', spell({ range: { kind: SpellRangeKind.TOUCH } })),
    ).toBe('Reação · toque');
    expect(
      sub(
        ActionEconomy.ACTION,
        'plain',
        spell({ range: { kind: SpellRangeKind.SELF } }),
        1,
        0,
        'esfera de 6 m',
      ),
    ).toBe('Ação · pessoal · esfera de 6 m');
    expect(
      sub(
        ActionEconomy.ACTION,
        'plain',
        spell({ range: { kind: SpellRangeKind.RANGED, distanceFt: 0 } }),
      ),
    ).toBe('Ação');
    expect(sub(ActionEconomy.ACTION, 'plain', null)).toBe('Ação');
  });

  it('says what an attack, a save and a heal bring', () => {
    expect(sub(ActionEconomy.ACTION, 'attack', spell())).toBe(
      'Ação · alcance 36 m · ataque de magia · 3d6 de fogo',
    );
    expect(sub(ActionEconomy.ACTION, 'attack', spell({ damage: [] }))).toBe(
      'Ação · alcance 36 m · ataque de magia',
    );
    const save = {
      save: { ability: Ability.DEXTERITY, onSuccess: SpellSaveSuccess.HALF },
    };
    expect(sub(ActionEconomy.ACTION, 'save', spell(save), 2)).toBe(
      'Ação · alcance 36 m · resistência de Destreza · 5d6 de fogo, metade se resistir',
    );
    expect(
      sub(
        ActionEconomy.ACTION,
        'save',
        spell({ save: { ability: Ability.UNSPECIFIED }, damage: [] }),
      ),
    ).toBe('Ação · alcance 36 m · resistência');
    expect(sub(ActionEconomy.ACTION, 'heal', spell({ damage: [] }))).toBe(
      'Ação · alcance 36 m · cura',
    );
  });

  it('says the darts, with three as the default, and the pool', () => {
    expect(
      sub(
        ActionEconomy.ACTION,
        'darts',
        spell({ damage: [{ ...fire, damageTypePt: 'energia' }] }),
        1,
        4,
      ),
    ).toBe('Ação · alcance 36 m · 4 dardos de 1d4 + 1 de energia, sempre acertam');
    expect(sub(ActionEconomy.ACTION, 'darts', spell({ damage: [] }))).toBe(
      'Ação · alcance 36 m · 3 dardos de 1d4 + 1 de energia, sempre acertam',
    );
    const pool = {
      hitPointEffect: {
        kind: SpellHitPointEffectKind.POOL,
        poolDiceCount: 5,
        poolDiceSides: 8,
        poolDicePerLevel: 2,
      },
    };
    expect(sub(ActionEconomy.ACTION, 'pool', spell(pool), 2)).toBe(
      'Ação · alcance 36 m · 7d8 de pontos de vida',
    );
    expect(sub(ActionEconomy.ACTION, 'pool', spell())).toBe('Ação · alcance 36 m · pontos de vida');
    expect(sub(ActionEconomy.ACTION, 'hp', spell())).toBe('Ação · alcance 36 m');
  });
});

describe('the slot rows and the cast rows, branch by branch', () => {
  it('builds the pact row from the usage when the server sends no pact choice, and a lone free slot from the usage', () => {
    const pact = { slotLevel: 2, total: 2, used: 1 };
    const rows = slotRows(1, [], [{ level: 1, total: 3, used: 1 }], pact);
    expect(rows.map((r) => [r.level, r.pact, r.free, r.total, r.used, r.enabled])).toEqual([
      [1, false, 2, 3, 1, false],
      [2, true, 1, 2, 1, false],
    ]);
    expect(slotRows(1, [], [], null)).toEqual([]);
    const onlyPact = slotRows(1, [{ level: 3, free: 2, pact: true } as never], []);
    expect(onlyPact.map((r) => [r.level, r.pact, r.free, r.total, r.enabled, r.count])).toEqual([
      [3, true, 2, null, true, '2 livres'],
    ]);
  });

  const labels = new Map([['t', { label: 'Toren', state: CombatantState.UNHURT }]]);
  const rowOf = (target: object, pending?: Partial<PendingDamage>) => {
    const cast = create(SpellCastSchema, {
      targets: [{ combatantId: 't', pendingDamageId: 'p', ...target }],
    } as never);
    return castRows(cast, new Map(pending ? [['p', pending as PendingDamage]] : []), labels)[0];
  };
  const d20 = (face: number) => ({
    diceCount: 1,
    diceSides: 20,
    faces: [face],
    modifier: 3,
    total: face + 3,
    countedIndex: 0,
  });

  it('writes the outcome of a spell attack: critical, hit, miss, and the natural 1', () => {
    expect(rowOf({ outcome: AttackOutcome.CRITICAL_HIT })).toMatchObject({
      word: 'Crítico',
      tone: 'good',
    });
    expect(rowOf({ outcome: AttackOutcome.HIT })).toMatchObject({ word: 'Acertou', tone: 'good' });
    const miss = rowOf({ outcome: AttackOutcome.MISS, attackRoll: d20(1) });
    expect(miss).toMatchObject({ word: 'Errou', tone: 'bad', natural: '1 natural' });
    expect(miss.d20s?.map((d) => d.caption)).toEqual(['Ataque']);
    expect(rowOf({ outcome: AttackOutcome.MISS, attackRoll: d20(5) }).natural).toBeUndefined();
  });

  it('writes the save when it has no roll: sculpted, failed on its own, or only the DC', () => {
    const save = (over: object) =>
      rowOf({ save: { outcome: SaveOutcome.SAVED, dc: 0, ...over } }).lines;
    expect(save({ sculpted: true })).toEqual(['Resistência: passa sem rolar (Esculpir Magias)']);
    expect(save({ autoFailed: true, dc: 15 })).toEqual(['Resistência: Falha automática, CD 15']);
    expect(save({ autoFailed: true })).toEqual(['Resistência: Falha automática']);
    expect(save({ dc: 13 })).toEqual(['CD 13']);
    expect(save({})).toEqual([]);
    const withRoll = rowOf({ save: { outcome: SaveOutcome.FAILED, dc: 14, roll: d20(9) } });
    expect(plain(withRoll.lines[0])).toBe('Resistência: 1d20 (9) + 3 = 12, CD 14');
    expect(withRoll.d20s?.map((d) => d.caption)).toEqual(['Resistência']);
    expect(withRoll).toMatchObject({ word: 'Falhou', tone: 'good' });
  });

  it('writes the darts and the damage of a target: waiting, without dice, healing, rolled', () => {
    const roll = { diceCount: 1, diceSides: 4, faces: [3], modifier: 1, total: 4, physical: false };
    expect(rowOf({ darts: 1 }).lines).toEqual(['1 dardo']);
    expect(rowOf({ darts: 2 }).lines).toEqual(['2 dardos']);
    expect(
      rowOf({ darts: 1 }, { status: PendingDamageStatus.APPLIED, amount: 4, roll } as never),
    ).toMatchObject({
      lines: ['Dardo 1: 1d4 (3) + 1 = 4'],
      summary: '4 de dano',
    });
    expect(
      rowOf({}, { status: PendingDamageStatus.APPLIED, amount: 7, healing: true, roll } as never)
        .summary,
    ).toBe('7 de cura');
    expect(
      rowOf({}, {
        status: PendingDamageStatus.ROLLED,
        amount: 6,
        damageTypePt: 'frio',
        roll,
      } as never).summary,
    ).toBe('6 de frio. Esperando o mestre aplicar');
    expect(
      rowOf({}, { status: PendingDamageStatus.APPLIED, amount: 6, roll } as never).summary,
    ).toBe('6 de dano');
    expect(
      rowOf({}, { status: PendingDamageStatus.AWAITING_ROLL, amount: 0 } as never),
    ).toMatchObject({
      owed: true,
      summary: '',
    });
    expect(rowOf({}).state).toBe('Ileso');
  });
});
