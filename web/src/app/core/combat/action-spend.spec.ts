import { create } from '@bufbuild/protobuf';

import { ActionOptionSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  CombatantKind,
  CombatantSchema,
  CombatantState,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  allyRows,
  asksHowMany,
  costDetail,
  givesResult,
  healDice,
  notEnoughUses,
  opensSheet,
  spendPreview,
  spendRange,
} from './action-spend';

/** An invented table feature: "Fôlego da Mesa" spends 1 to 3 points of "Reserva de Teste" and heals 1d6 per point. */
function option(
  over: {
    cost?: number;
    costMax?: number;
    usesLeft?: number;
    resourceKey?: string;
    heal?: { count: number; sides: number; bonus?: number };
    healPerPoint?: { count: number; sides: number };
    tempHp?: number;
    tempHpPerPoint?: number;
    enabled?: boolean;
  } = {},
) {
  return create(ActionOptionSchema, {
    action: {
      key: 'feature:folego-da-mesa',
      namePt: 'Fôlego da Mesa',
      resourceKey: over.resourceKey ?? 'reserva_de_teste',
      cost: over.cost ?? 0,
      costMax: over.costMax ?? 0,
      heal: over.heal,
      healPerPoint: over.healPerPoint,
      tempHp: over.tempHp ?? 0,
      tempHpPerPoint: over.tempHpPerPoint ?? 0,
    },
    usesLeft: over.usesLeft ?? 5,
    enabled: over.enabled ?? true,
  });
}

describe('the range a variable-cost action asks "quantos?" with', () => {
  it('is max(cost, 1) to the smaller of cost_max and what is left', () => {
    expect(spendRange(option({ costMax: 3, usesLeft: 5 }))).toEqual({ min: 1, max: 3 });
    expect(spendRange(option({ cost: 2, costMax: 4, usesLeft: 3 }))).toEqual({ min: 2, max: 3 });
    expect(spendRange(option({ cost: 2, costMax: 4, usesLeft: 9 }))).toEqual({ min: 2, max: 4 });
  });

  it('asks only when cost_max is above max(cost, 1); a fixed cost, or a cost_max equal to the cost, asks nothing', () => {
    expect(asksHowMany(option({ costMax: 3 }))).toBe(true);
    expect(asksHowMany(option({ cost: 2, costMax: 2 }))).toBe(false);
    expect(asksHowMany(option({ costMax: 1 }))).toBe(false);
    expect(asksHowMany(option({ cost: 2 }))).toBe(false);
    expect(asksHowMany(option())).toBe(false);
    expect(spendRange(option({ cost: 2 }))).toBeNull();
  });

  it('is empty (null) when fewer are left than the least the action costs', () => {
    expect(spendRange(option({ cost: 2, costMax: 4, usesLeft: 1 }))).toBeNull();
    expect(notEnoughUses(option({ cost: 2, costMax: 4, usesLeft: 1 }))).toBe(true);
    expect(notEnoughUses(option({ cost: 2, usesLeft: 2 }))).toBe(false);
    expect(notEnoughUses(option({ cost: 2, usesLeft: 0 }))).toBe(true);
  });

  it('is never "not enough" for an action with no resource', () => {
    expect(notEnoughUses(option({ resourceKey: '', cost: 2, usesLeft: 0 }))).toBe(false);
  });
});

describe('what the action card says it will cost and give (display only: the server decides)', () => {
  it('names the cost and the dice for the chosen number: 1d6 per point, three points, 3d6', () => {
    const o = option({
      costMax: 3,
      heal: { count: 1, sides: 6 },
      healPerPoint: { count: 1, sides: 6 },
    });
    expect(spendPreview(o, 1)).toBe('Gasta 1 · cura 1d6');
    expect(spendPreview(o, 3)).toBe('Gasta 3 · cura 3d6');
  });

  it('adds the bonus once, and the temporary hit points of each point', () => {
    const o = option({
      costMax: 3,
      heal: { count: 1, sides: 6, bonus: 2 },
      healPerPoint: { count: 1, sides: 6 },
      tempHp: 2,
      tempHpPerPoint: 2,
    });
    expect(spendPreview(o, 2)).toBe('Gasta 2 · cura 2d6 + 2 · 4 PV temporários');
  });

  it('counts the points above the cost, not above one', () => {
    const o = option({
      cost: 2,
      costMax: 4,
      heal: { count: 2, sides: 4 },
      healPerPoint: { count: 1, sides: 4 },
    });
    expect(spendPreview(o, 2)).toBe('Gasta 2 · cura 2d4');
    expect(spendPreview(o, 4)).toBe('Gasta 4 · cura 4d4');
  });

  it('says only the cost for an action that gives nothing, with the word "uso" in the singular', () => {
    expect(spendPreview(option({ costMax: 3 }), 1)).toBe('Gasta 1 uso');
    expect(spendPreview(option({ costMax: 3 }), 2)).toBe('Gasta 2 usos');
  });

  it("says what is left and the range in the row's detail line", () => {
    expect(costDetail(option({ costMax: 3, usesLeft: 5 }))).toBe('Gasta de 1 a 3 · restam 5');
    expect(costDetail(option({ cost: 2, usesLeft: 5 }))).toBe('Gasta 2 · restam 5');
    expect(costDetail(option({ cost: 2, costMax: 4, usesLeft: 3 }))).toBe(
      'Gasta de 2 a 3 · restam 3',
    );
  });

  it('leaves the detail of a fixed one-use action to the usual line', () => {
    expect(costDetail(option({ usesLeft: 5 }))).toBe('');
    expect(costDetail(option({ cost: 1, usesLeft: 5 }))).toBe('');
  });

  it('reads "faltam usos" for an action the pool cannot pay', () => {
    expect(costDetail(option({ cost: 2, usesLeft: 1 }))).toBe('Faltam usos: gasta 2, restam 1');
  });
});

describe('what makes the card open its sheet', () => {
  it('opens for a choice of points, a heal or temporary hit points, and not for a plain action', () => {
    expect(opensSheet(option({ costMax: 3 }))).toBe(true);
    expect(opensSheet(option({ heal: { count: 1, sides: 8 } }))).toBe(true);
    expect(opensSheet(option({ tempHp: 3 }))).toBe(true);
    expect(opensSheet(option({ cost: 2 }))).toBe(false);
    expect(givesResult(option())).toBe(false);
  });

  it('rolls the heal for the points spent: the dice grow by the dice per point', () => {
    const o = option({
      costMax: 3,
      heal: { count: 1, sides: 6, bonus: 2 },
      healPerPoint: { count: 1, sides: 6 },
    });
    expect(healDice(o, 1)).toEqual({ count: 1, sides: 6, bonus: 2 });
    expect(healDice(o, 3)).toEqual({ count: 3, sides: 6, bonus: 2 });
    expect(healDice(option({ tempHp: 3 }), 1)).toBeNull();
  });
});

describe('who a heal may go to', () => {
  const c = (id: string, label: string, over: object = {}) =>
    create(CombatantSchema, {
      id,
      label,
      kind: CombatantKind.PLAYER,
      state: CombatantState.UNHURT,
      ...over,
    });

  it('lists the other player characters the viewer sees and that are not dead, never the actor', () => {
    const rows = allyRows(
      [
        c('me', 'Toren'),
        c('a', 'Lira', { state: CombatantState.HURT }),
        c('d', 'Morto', { state: CombatantState.DEAD }),
        c('x', 'Goblin', { kind: CombatantKind.NPC }),
        c('h', 'Escondida', { hidden: true }),
      ],
      'me',
    );
    expect(rows).toEqual([{ id: 'a', label: 'Lira', sub: 'Ferido' }]);
  });
});
