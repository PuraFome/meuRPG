import { type MessageInitShape, create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import {
  CastingTimeUnit,
  SpellDurationKind,
  SpellDurationUnit,
  SpellRangeKind,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  TableContentKind,
  TableEffectAudience,
  TableEffectModifierKind,
  type TableEffectModifierSchema,
  TableHitConditionDuration,
  TableHitTrigger,
  TableSpellSchema,
  type TableSpell,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { readEntry } from './content-read';
import { catalog, entry } from './content-testing';
import { readHit, visibilityText } from './hit-effect-read';
import { spellToDraft } from './spell-draft';

const cat = catalog([]);
const nameOf = (k: string) => (k === 'damage-type:cold' ? 'frio' : cat.nameOf(k));
const flat = (s: string) => s.replace(/ /g, ' ');

type Mod = MessageInitShape<typeof TableEffectModifierSchema>;

function spell(mod: Mod | null, extra: object = {}, effect: object = {}): TableSpell {
  return create(TableSpellSchema, {
    namePt: 'Geada no Punho',
    level: 2,
    castingTime: { unit: CastingTimeUnit.BONUS_ACTION, amount: 1 },
    range: { kind: SpellRangeKind.SELF },
    duration: { kind: SpellDurationKind.TIMED, amount: 1, unit: SpellDurationUnit.MINUTE },
    concentration: false,
    target: { kind: 4 },
    ...(mod
      ? {
          effect: {
            visibility: TableEffectAudience.OWNER,
            modifiers: [
              {
                kind: TableEffectModifierKind.WEAPON_HIT_DAMAGE,
                trigger: TableHitTrigger.NEXT_MELEE_HIT,
                die: 8,
                dice: 2,
                maxDice: 5,
                damageType: 'damage-type:cold',
                ...mod,
              },
            ],
            ...effect,
          },
        }
      : {}),
    ...extra,
  });
}

const hitOf = (s: TableSpell) => readHit(spellToDraft(s), nameOf);

describe('what a player reads of the next-hit effect (#577)', () => {
  it('reads the spell of the board: where, chips, save sentence and audience', () => {
    const h = hitOf(
      spell({
        perSlotDice: 1,
        save: 'str',
        onFailConditions: ['condition:restrained'],
        conditionDuration: TableHitConditionDuration.UNTIL_END_OF_NEXT_TURN,
      }),
    )!;
    expect(flat(h.where)).toBe('Em quem conjura: o próximo golpe corpo a corpo, por até 1 minuto.');
    expect(h.damage).toBe('+2d8 frio');
    expect(h.more).toBe('+1d8 por nível acima do 2º, até 5d8');
    expect(h.save.map((p) => p.text).join('')).toBe(
      'O alvo faz um teste de resistência de Força; se falhar, fica Impedido até o fim do próximo turno dele.',
    );
    expect(h.save.filter((p) => p.strong).map((p) => p.text)).toEqual(['Impedido']);
    expect(h.visibility).toBe('Só quem conjura e você');
  });

  it.each([
    [TableHitTrigger.NEXT_HIT, 'o próximo golpe com arma', 'No próximo golpe com arma:'],
    [
      TableHitTrigger.NEXT_MELEE_HIT,
      'o próximo golpe corpo a corpo',
      'No próximo golpe corpo a corpo:',
    ],
    [TableHitTrigger.EACH_HIT, 'cada golpe com arma', 'Em cada golpe com arma:'],
    [TableHitTrigger.MARKED_TARGET, 'cada golpe no alvo marcado', 'Em cada golpe no alvo marcado:'],
  ])('says the trigger %s', (trigger, where, lead) => {
    const h = hitOf(spell({ trigger }))!;
    expect(h.where).toContain(`Em quem conjura: ${where},`);
    expect(h.lead).toBe(lead);
  });

  it.each([
    ['condition:prone', 'Derrubado'],
    ['condition:restrained', 'Impedido'],
    ['condition:blinded', 'Cego'],
  ])('names %s as "%s", never a key, and "Caído" is not a condition', (key, name) => {
    const h = hitOf(spell({ save: 'con', onFailConditions: [key] }))!;
    const text = h.save.map((p) => p.text).join('');
    expect(text).toContain(`fica ${name} até`);
    expect(text).not.toMatch(/condition:|Caído/);
  });

  it('says each duration of the condition', () => {
    const turn = hitOf(
      spell({
        save: 'con',
        onFailConditions: ['condition:blinded'],
        conditionDuration: TableHitConditionDuration.UNTIL_END_OF_NEXT_TURN,
      }),
    )!;
    const end = hitOf(
      spell({
        save: 'con',
        onFailConditions: ['condition:blinded'],
        conditionDuration: TableHitConditionDuration.UNTIL_SPELL_END,
      }),
    )!;
    expect(turn.save.at(-1)?.text).toBe(' até o fim do próximo turno dele.');
    expect(end.save.at(-1)?.text).toBe(' até o fim da magia.');
  });

  it('lists several conditions, and reads a test with no condition as the test alone', () => {
    const many = hitOf(
      spell({
        save: 'wis',
        onFailConditions: ['condition:blinded', 'condition:prone', 'condition:deafened'],
        conditionDuration: TableHitConditionDuration.UNTIL_SPELL_END,
      }),
    )!;
    expect(many.save.map((p) => p.text).join('')).toBe(
      'O alvo faz um teste de resistência de Sabedoria; se falhar, fica Cego, Derrubado e Surdo até o fim da magia.',
    );
    expect(
      hitOf(spell({ save: 'dex' }))!
        .save.map((p) => p.text)
        .join(''),
    ).toBe('O alvo faz um teste de resistência de Destreza.');
  });

  it('has no save sentence without a test', () => {
    expect(hitOf(spell({}))!.save).toEqual([]);
  });

  it("says the weapon's own type when none is set", () => {
    expect(hitOf(spell({ damageType: '' }))!.damage).toBe('+2d8 do tipo da arma');
  });

  it('says the per-level dice with the ordinal of the spell and the cap, and nothing without per-level dice', () => {
    expect(hitOf(spell({ perSlotDice: 2, maxDice: 6 }, { level: 1 }))!.more).toBe(
      '+2d8 por nível acima do 1º, até 6d8',
    );
    expect(hitOf(spell({ perSlotDice: 1, maxDice: 8 }, { level: 3 }))!.more).toBe(
      '+1d8 por nível acima do 3º, até 8d8',
    );
    expect(hitOf(spell({ perSlotDice: 0, maxDice: 5 }))!.more).toBe('');
  });

  it('says the duration of the spell, with the plural, and a spell with no end', () => {
    const d = (extra: object) => hitOf(spell({}, extra))!.where;
    expect(
      d({
        duration: { kind: SpellDurationKind.TIMED, amount: 10, unit: SpellDurationUnit.MINUTE },
      }),
    ).toContain('por até 10 minutos.');
    expect(
      d({
        duration: { kind: SpellDurationKind.TIMED, amount: 1, unit: SpellDurationUnit.HOUR },
        concentration: true,
      }),
    ).toContain('por até 1 hora.');
    expect(
      d({ duration: { kind: SpellDurationKind.UNTIL_DISPELLED }, concentration: true }),
    ).toContain('por até a magia acabar.');
  });

  it('says the audience the server sends: all, owner, and the owner when unset', () => {
    expect(hitOf(spell({}, {}, { visibility: TableEffectAudience.PUBLIC }))!.visibility).toBe(
      'Todos na mesa',
    );
    expect(visibilityText(TableEffectAudience.OWNER)).toBe('Só quem conjura e você');
    expect(visibilityText(TableEffectAudience.UNSPECIFIED)).toBe('Só quem conjura e você');
  });

  it("is not there without the modifier, or on a spell that does not last, and ignores the effect's other modifiers", () => {
    expect(hitOf(spell(null))).toBeNull();
    expect(hitOf(spell({}, { duration: { kind: SpellDurationKind.INSTANTANEOUS } }))).toBeNull();
    const other = spell(null, {
      effect: { modifiers: [{ kind: TableEffectModifierKind.AC_BONUS, value: 2 }] },
    });
    expect(hitOf(other)).toBeNull();
  });

  it('reaches the read view of the spell as its `hit`, beside the rows and the text', () => {
    const e = entry(TableContentKind.SPELL, 'Geada no Punho', {
      body: { case: 'tableSpell', value: spell({ perSlotDice: 1 }, { descPt: ['O frio sobe.'] }) },
    });
    const read = readEntry(e, nameOf);
    expect(read.hit?.damage).toBe('+2d8 frio');
    expect(read.text).toEqual(['O frio sobe.']);
    expect(read.rows.map((r) => r.label)).not.toContain('Efeito no próximo golpe');
  });
});
