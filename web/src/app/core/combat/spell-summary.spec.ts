import { SpellHitPointEffectKind, SpellRangeKind } from '../../../gen/meurpg/rules/v1/rules_pb';
import { spellSummary } from './spell-summary';

const plain = (s: string) => s.replace(/ /g, ' ');

describe('spellSummary (the line under a spell in the list, E8-02)', () => {
  it('says the reach and the pool of Sono from its own details', () => {
    const sleep = {
      spell: { level: 1 },
      range: { kind: SpellRangeKind.RANGED, distanceFt: 90 },
      damage: [],
      healBySlotLevel: {},
      hitPointEffect: {
        kind: SpellHitPointEffectKind.POOL,
        poolDiceCount: 5,
        poolDiceSides: 8,
        poolDicePerLevel: 2,
      },
    } as never;
    expect(plain(spellSummary(sleep))).toBe('Alcance 27 m · 5d8 PV de criaturas');
  });

  it('says touch and self, and the damage with its type', () => {
    const burning = {
      spell: { level: 1 },
      range: { kind: SpellRangeKind.SELF },
      damage: [{ damageTypePt: 'fogo', bySlotLevel: { 1: '3d6' }, byCharacterLevel: {} }],
      healBySlotLevel: {},
    } as never;
    expect(plain(spellSummary(burning))).toBe('Pessoal · 3d6 de fogo');
    expect(
      plain(
        spellSummary({
          spell: { level: 1 },
          range: { kind: SpellRangeKind.TOUCH },
          damage: [],
          healBySlotLevel: {},
        } as never),
      ),
    ).toBe('Toque');
  });

  it('is empty until the details are read', () => {
    expect(spellSummary(null)).toBe('');
    expect(spellSummary(undefined)).toBe('');
  });
});
