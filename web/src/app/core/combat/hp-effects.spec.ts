import { create } from '@bufbuild/protobuf';

import {
  DiceRollSchema,
  SpellEffectKind,
  SpellEffectOutcome,
  SpellEffectReason,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { SpellHitPointEffectKind } from '../../../gen/meurpg/rules/v1/rules_pb';
import { effectWords, hpSpellKind, poolDice, poolRollText, reasonWords } from './hp-effects';

describe('the spells that read hit points (E8-03)', () => {
  const sleep = {
    spell: { level: 1 },
    hitPointEffect: {
      kind: SpellHitPointEffectKind.POOL,
      poolDiceCount: 5,
      poolDiceSides: 8,
      poolDicePerLevel: 2,
    },
  } as never;

  it("reads the kind from the spell's own details, and no other spell is one", () => {
    expect(hpSpellKind(sleep)).toBe('pool');
    expect(
      hpSpellKind({ hitPointEffect: { kind: SpellHitPointEffectKind.THRESHOLD } } as never),
    ).toBe('threshold');
    expect(
      hpSpellKind({ hitPointEffect: { kind: SpellHitPointEffectKind.ZERO_HP } } as never),
    ).toBe('zero');
    expect(
      hpSpellKind({ hitPointEffect: { kind: SpellHitPointEffectKind.FLAT_HEAL } } as never),
    ).toBe('heal');
    expect(hpSpellKind({} as never)).toBeNull();
    expect(hpSpellKind(null)).toBeNull();
  });

  it('gives the pool at the slot used: more dice for each circle above the 1st', () => {
    expect(poolDice(sleep, 1)).toEqual({ count: 5, sides: 8 });
    expect(poolDice(sleep, 2)).toEqual({ count: 7, sides: 8 });
    expect(poolDice(sleep, 4)).toEqual({ count: 11, sides: 8 });
    expect(
      poolDice({ hitPointEffect: { kind: SpellHitPointEffectKind.THRESHOLD } } as never, 7),
    ).toBeNull();
    expect(poolDice(null, 1)).toBeNull();
  });

  it('says what happened in a word and an icon, by the condition the spell gives', () => {
    const slept = effectWords(
      SpellEffectKind.POOL,
      'condition:unconscious',
      SpellEffectOutcome.AFFECTED,
      'Goblin 1',
    );
    expect(slept).toMatchObject({
      present: 'adormece',
      past: 'Adormeceu',
      icon: 'bedtime',
      affected: true,
    });
    expect(
      effectWords(SpellEffectKind.POOL, 'condition:blinded', SpellEffectOutcome.AFFECTED).past,
    ).toBe('Ficou cego');
    expect(
      effectWords(SpellEffectKind.THRESHOLD, 'condition:stunned', SpellEffectOutcome.AFFECTED)
        .present,
    ).toBe('fica atordoado');
    // A threshold with no condition kills; a zero-hit-point spell stabilises; a heal heals.
    expect(effectWords(SpellEffectKind.THRESHOLD, '', SpellEffectOutcome.AFFECTED).past).toBe(
      'Morreu',
    );
    expect(effectWords(SpellEffectKind.ZERO_HP, '', SpellEffectOutcome.AFFECTED).present).toBe(
      'está estável',
    );
    expect(
      effectWords(SpellEffectKind.FLAT_HEAL, '', SpellEffectOutcome.AFFECTED, 'Brisa').past,
    ).toBe('Foi curada');
    expect(
      effectWords(SpellEffectKind.FLAT_HEAL, '', SpellEffectOutcome.AFFECTED, 'Toren').past,
    ).toBe('Foi curado');
  });

  it('says "não foi afetado" in the gender of the name, with the block icon', () => {
    const goblin = effectWords(
      SpellEffectKind.POOL,
      'condition:unconscious',
      SpellEffectOutcome.NOT_AFFECTED,
      'Capitão Goblin',
    );
    expect(goblin).toMatchObject({
      past: 'Não foi afetado',
      present: 'não é afetado',
      icon: 'block',
      affected: false,
    });
    expect(
      effectWords(SpellEffectKind.THRESHOLD, '', SpellEffectOutcome.NOT_AFFECTED, 'Brisa').past,
    ).toBe('Não foi afetada');
  });

  it('gives the master the reason a creature was not affected', () => {
    expect(reasonWords(SpellEffectReason.ABOVE_POOL, 27, 8, undefined)).toBe(
      '27 é mais que 8 restantes',
    );
    expect(reasonWords(SpellEffectReason.ABOVE_LIMIT, 27, undefined, 150)).toBe(
      '27 PV, acima do limite de 150',
    );
    expect(reasonWords(SpellEffectReason.SKIPPED, 7, 8, undefined)).toBe(
      'já estava inconsciente ou a 0 PV',
    );
    expect(reasonWords(SpellEffectReason.NOT_AT_ZERO, 12, undefined, undefined)).toBe(
      'não estava a 0 PV',
    );
    expect(reasonWords(SpellEffectReason.UNSPECIFIED, undefined, undefined, undefined)).toBe('');
  });

  it("writes the caster's pool with its dice, or the typed sum of a physical die", () => {
    const rolled = create(DiceRollSchema, {
      diceCount: 5,
      diceSides: 8,
      faces: [2, 4, 1, 5, 3],
      modifier: 0,
      total: 15,
    });
    expect(poolRollText(rolled)).toBe('5d8 (2, 4, 1, 5, 3) = 15');
    const typed = create(DiceRollSchema, {
      diceCount: 5,
      diceSides: 8,
      faces: [],
      modifier: 0,
      total: 20,
      physical: true,
    });
    expect(poolRollText(typed)).toBe('5d8 = 20 · dado físico');
  });
});
