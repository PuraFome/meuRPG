import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import {
  TableAuraAffects,
  TableEffectApplies,
  TableEffectAudience,
  TableEffectModifierKind as K,
  TableEffectModifierSchema,
  TableEffectRollKind,
  TableResistanceKind,
  TableSpellEffectSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import {
  type EffectRow,
  type SpellEffectDraft,
  draftToEffect,
  effectToDraft,
  emptyEffect,
  emptyRow,
  modifierIndexOf,
  readWhole,
  rowOfModifier,
  rowToModifiers,
} from './spell-effect-draft';
import { effectAdvice, effectUnreadable } from './spell-effect-checks';
import { auraTag, effectReadItem, rowTags } from './spell-effect-tags';

const name = (k: string) =>
  ({ 'damage-type:fire': 'Fogo', 'condition:frightened': 'Amedrontado' })[k] ?? k;
const row = (p: Partial<EffectRow>): EffectRow => ({ ...emptyRow('roll'), ...p });
const on = (p: Partial<SpellEffectDraft>): SpellEffectDraft => ({
  ...emptyEffect(),
  on: true,
  ...p,
});

describe('the stored effect as a draft and back', () => {
  it('has no effect while the block is off, and sends none', () => {
    expect(draftToEffect(emptyEffect())).toBeUndefined();
    expect(effectToDraft(undefined).on).toBe(false);
  });

  it('reads every kind the menu offers into one row and writes the same modifiers back', () => {
    const mods = [
      create(TableEffectModifierSchema, { kind: K.AC_BONUS, value: 2 }),
      create(TableEffectModifierSchema, {
        kind: K.ROLL_DIE,
        die: 4,
        sign: 1,
        appliesTo: [TableEffectRollKind.ATTACK, TableEffectRollKind.SAVE],
      }),
      create(TableEffectModifierSchema, {
        kind: K.ROLL_BONUS,
        value: -1,
        appliesTo: [TableEffectRollKind.CHECK],
      }),
      create(TableEffectModifierSchema, { kind: K.CHECK_BONUS, value: 3, skill: 'skill:stealth' }),
      create(TableEffectModifierSchema, { kind: K.TEMP_HP, value: 10 }),
      create(TableEffectModifierSchema, {
        kind: K.RESISTANCE,
        resistanceKind: TableResistanceKind.IMMUNE,
        damageTypes: ['damage-type:fire'],
      }),
      create(TableEffectModifierSchema, { kind: K.SPEED_ADD, value: 10 }),
      create(TableEffectModifierSchema, { kind: K.SPEED_MULTIPLIER, value: 50 }),
    ];
    const e = create(TableSpellEffectSchema, {
      applies: TableEffectApplies.FAILED_SAVE,
      conditions: ['condition:frightened'],
      modifiers: mods,
      playerLabelPt: 'Treme',
      visibility: TableEffectAudience.PUBLIC,
    });
    const d = effectToDraft(e);
    expect(d.rows.map((r) => r.kind)).toEqual([
      'ac_bonus',
      'roll',
      'roll',
      'check_bonus',
      'temp_hp',
      'resistance',
      'speed',
      'speed',
    ]);
    expect(d.rows[5].resistance).toBe('immune');
    expect(d.rows[6].speedM).toBe('3');
    const back = draftToEffect(d)!;
    expect(back.modifiers.map((m) => [m.kind, m.value, m.die, m.skill])).toEqual(
      mods.map((m) => [m.kind, m.value, m.die, m.skill]),
    );
    expect(back.applies).toBe(TableEffectApplies.FAILED_SAVE);
    expect(back.playerLabelPt).toBe('Treme');
    expect(back.conditions).toEqual(['condition:frightened']);
  });

  it('keeps a modifier the menu does not offer as it came', () => {
    const odd = create(TableEffectModifierSchema, { kind: K.ATTACK_DISADVANTAGE });
    const raw = create(TableEffectModifierSchema, { kind: 9 as K });
    const d = effectToDraft(create(TableSpellEffectSchema, { modifiers: [raw, odd] }));
    expect(d.rows[0].kind).toBe('unknown');
    expect(draftToEffect(d)!.modifiers[0].kind).toBe(9);
  });

  it('keeps a modifier the block does not edit (the next-hit weapon damage of #577) in place and whole while its neighbours change', () => {
    const next = create(TableEffectModifierSchema, {
      kind: 77 as K,
      value: 4,
      die: 6,
      appliesTo: [3],
    });
    const d = effectToDraft(
      create(TableSpellEffectSchema, {
        modifiers: [
          create(TableEffectModifierSchema, { kind: K.AC_BONUS, value: 1 }),
          next,
          create(TableEffectModifierSchema, { kind: K.TEMP_HP, value: 5 }),
        ],
      }),
    );
    const edited = {
      ...d,
      rows: d.rows.map((r) => (r.kind === 'ac_bonus' ? { ...r, value: '2' } : r)),
    };
    const back = draftToEffect(edited)!.modifiers;
    expect(back.map((m) => m.kind)).toEqual([K.AC_BONUS, 77, K.TEMP_HP]);
    expect(back[0].value).toBe(2);
    expect(back[1]).toMatchObject({ value: 4, die: 6, appliesTo: [3] });
  });

  it('writes one advantage row as a modifier for each roll, and reads them back as one row', () => {
    const r = emptyRow('advantage');
    const rows = {
      ...r,
      advantage: 'dis' as const,
      rolls: ['attack', 'save', 'check'] as const,
      abilities: ['dex'],
    };
    const mods = rowToModifiers(rows as EffectRow);
    expect(mods.map((m) => m.kind)).toEqual([
      K.ATTACK_DISADVANTAGE,
      K.SAVE_DISADVANTAGE,
      K.CHECK_DISADVANTAGE,
    ]);
    expect(mods[0].abilities).toEqual([]);
    expect(mods[1].abilities).toEqual(['dex']);
    const back = effectToDraft(create(TableSpellEffectSchema, { modifiers: mods })).rows;
    expect(back).toHaveLength(1);
    expect(back[0]).toMatchObject({
      kind: 'advantage',
      advantage: 'dis',
      rolls: ['attack', 'save', 'check'],
      abilities: ['dex'],
    });
  });

  it('keeps two advantage rows apart when their abilities differ', () => {
    const mods = [
      create(TableEffectModifierSchema, { kind: K.SAVE_ADVANTAGE, abilities: ['dex'] }),
      create(TableEffectModifierSchema, { kind: K.CHECK_ADVANTAGE, abilities: ['wis'] }),
    ];
    expect(effectToDraft(create(TableSpellEffectSchema, { modifiers: mods })).rows).toHaveLength(2);
  });

  it('sends the aura in feet and no condition, and reads the radius back in metres', () => {
    const d = on({
      aura: { radiusM: '4,5', affects: 'enemies' },
      conditions: ['condition:frightened'],
      rows: [row({ rolls: ['attack'], value: '1' })],
    });
    const e = draftToEffect(d)!;
    expect(e.aura?.radiusFt).toBe(15);
    expect(e.aura?.affects).toBe(TableAuraAffects.ENEMIES);
    expect(e.conditions).toEqual([]);
    expect(effectToDraft(e).aura).toEqual({ radiusM: '4,5', affects: 'enemies' });
  });

  it('sends metres as feet in steps of 5', () => {
    const r = { ...emptyRow('speed'), speedM: '3' };
    expect(rowToModifiers(r)[0].value).toBe(10);
  });
});

describe('what the form asks before the save', () => {
  it('says which number fields are no whole number, on the modifier the server will refuse', () => {
    const d = on({
      rows: [
        { ...emptyRow('ac_bonus'), value: '2' },
        { ...emptyRow('temp_hp'), value: '1x' },
      ],
      aura: { radiusM: 'abc', affects: 'self' },
    });
    expect(effectUnreadable(d)).toEqual([
      'table_spell.effect.modifiers[1].value',
      'table_spell.effect.aura.radius_ft',
    ]);
    expect(readWhole('+3')).toBe(3);
    expect(readWhole('')).toBeNaN();
  });

  it('refuses an empty effect, and AC, speed and temporary hit points inside an aura', () => {
    expect(effectAdvice(on({})).map((a) => a.path)).toEqual(['table_spell.effect']);
    const rows = [
      { ...emptyRow('ac_bonus'), value: '1' },
      { ...emptyRow('roll'), value: '1' },
    ];
    const adv = effectAdvice(on({ rows, aura: { radiusM: '9', affects: 'allies' } }));
    expect(adv.map((a) => a.path)).toEqual(['table_spell.effect.modifiers[0].kind']);
    expect(effectAdvice(emptyEffect())).toEqual([]);
    const none = { ...emptyRow('advantage'), rolls: [] as never[] };
    expect(effectAdvice(on({ rows: [none] })).map((a) => a.path)).toContain(
      'table_spell.effect.rows[0].applies_to',
    );
    const next = { ...emptyRow('temp_hp'), value: '5' };
    expect(
      effectAdvice(on({ rows: [none, next] })).filter((a) => a.path.includes('modifiers[0]')),
    ).toEqual([]);
    const noAbility = {
      ...emptyRow('advantage'),
      rolls: ['attack', 'save'] as const,
      abilities: [],
    };
    expect(effectAdvice(on({ rows: [noAbility as never] })).map((a) => a.path)).toContain(
      'table_spell.effect.modifiers[1].abilities',
    );
  });

  it('maps the index of a sent modifier back to its row', () => {
    const adv = { ...emptyRow('advantage'), rolls: ['attack', 'save'] as const };
    const rows = [
      { ...emptyRow('temp_hp'), value: '1' },
      adv as EffectRow,
      { ...emptyRow('ac_bonus'), value: '1' },
    ];
    expect(modifierIndexOf(rows, 2)).toBe(3);
    expect(rowOfModifier(rows, 2)).toBe(1);
    expect(rowOfModifier(rows, 3)).toBe(2);
    expect(rowOfModifier(rows, 9)).toBe(-1);
  });
});

describe('the labels, as the server writes them for the card', () => {
  // The same cases as rules.EffectTagsPT's test: a change on either side has to change the other.
  const cases: [EffectRow, string[]][] = [
    [
      { ...emptyRow('roll'), rollMode: 'die', die: '4', sign: 1, rolls: ['attack', 'save'] },
      ['+1d4 em ataques e resistências'],
    ],
    [{ ...emptyRow('roll'), value: '-2', rolls: ['check'] }, ['-2 em testes']],
    [{ ...emptyRow('ac_bonus'), value: '2' }, ['CA +2']],
    [{ ...emptyRow('temp_hp'), value: '10' }, ['PV temporários: 10']],
    [
      { ...emptyRow('check_bonus'), value: '3', skill: 'skill:stealth' },
      ['+3 em testes de perícia'],
    ],
    [
      { ...emptyRow('resistance'), resistance: 'resist', damageTypes: ['damage-type:fire'] },
      ['Resistência a fogo'],
    ],
    [
      { ...emptyRow('resistance'), resistance: 'vulnerable', damageTypes: ['all'] },
      ['Vulnerabilidade a todo dano'],
    ],
    [
      { ...emptyRow('resistance'), resistance: 'immune', damageTypes: ['nonmagical_bps'] },
      ['Imunidade a dano de armas não mágicas'],
    ],
    [{ ...emptyRow('speed'), speedMode: 'add', speedM: '3' }, ['Deslocamento +3 m']],
    [{ ...emptyRow('speed'), speedMode: 'add', speedM: '4,5' }, ['Deslocamento +4,5 m']],
    [{ ...emptyRow('speed'), speedMode: 'multiply', percent: '50' }, ['Deslocamento 50%']],
    [
      { ...emptyRow('advantage'), advantage: 'dis', rolls: ['save'], abilities: ['dex', 'wis'] },
      ['Desvantagem em resistências (DES e SAB)'],
    ],
    [
      {
        ...emptyRow('advantage'),
        advantage: 'adv',
        rolls: ['attack', 'check'],
        abilities: ['str'],
      },
      ['Vantagem em ataques', 'Vantagem em testes (FOR)'],
    ],
  ];
  it.each(cases)('writes %#', (r, tags) => {
    expect(rowTags(r, name)).toEqual(tags);
  });

  it('writes the aura and the line a reader sees', () => {
    expect(auraTag({ radiusM: '9', affects: 'allies' })).toBe('Aura de 9 m: aliados');
    const item = effectReadItem(
      create(TableSpellEffectSchema, {
        aura: { radiusFt: 30, affects: TableAuraAffects.ALLIES },
        modifiers: [
          create(TableEffectModifierSchema, {
            kind: K.ROLL_BONUS,
            value: 1,
            appliesTo: [TableEffectRollKind.ATTACK],
          }),
        ],
      }),
      name,
    );
    expect(item).toEqual({
      title: 'Em volta de quem conjura',
      text: 'Aura de 9 m: aliados, +1 em ataques, enquanto durar.',
    });
    expect(effectReadItem(undefined, name)).toBeUndefined();
  });
});
