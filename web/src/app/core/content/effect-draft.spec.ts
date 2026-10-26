import { create } from '@bufbuild/protobuf';

import { TableEffectSchema } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { conditionMenu, menu, rangedAttackMenu, resistanceMenu } from './content-testing-menu';
import {
  draftToEffect,
  effectToDraft,
  emptyEffect,
  missingRequired,
  rangeFeet,
  unreadableRange,
  rangeMeters,
  splitTags,
} from './effect-draft';

describe("an effect from the server's menu (ADR-0018, section 4)", () => {
  const m = menu();

  it('sends a maneuver die with where it goes and what it spends, and asks for both', () => {
    const d = {
      ...emptyEffect('superiority_die'),
      applies: 'damage',
      resource: 'dados_de_golpe',
      value: '8',
      textPt: 'O alvo faz um teste.',
    };
    expect(draftToEffect(d, m)).toEqual({
      type: 'superiority_die',
      applies: 'damage',
      resource: 'dados_de_golpe',
      value: '8',
      textPt: 'O alvo faz um teste.',
    });
    expect(missingRequired({ ...d, applies: '' }, m)).toContain('applies');
  });

  it('knows the fields of each type from the menu, in its order, and the names of its lists', () => {
    expect(m.fieldsOf('proficiency').map((f) => f.name)).toEqual([
      'proficiency',
      'level',
      'only_proficient',
      'when',
      'text_pt',
    ]);
    expect(m.typeOf('sense')?.namePt).toBe('Sentido');
    expect(m.nameOf('senses', 'darkvision')).toBe('Visão no escuro');
    expect(m.nameOf('senses', 'x')).toBe('x');
    expect(m.maxEffects).toBe(4);
  });

  it('sends only the fields of the chosen type', () => {
    // A draft that still holds what an old type had: nothing of it goes.
    const d = {
      ...emptyEffect('proficiency'),
      proficiency: 'skill:perception',
      level: '',
      target: 'ac',
      mode: 'add',
      value: '2',
      rangeM: '18',
      sense: 'darkvision',
      count: 3,
      from: ['skill:arcana'],
      textPt: '  ',
    };
    expect(draftToEffect(d, m)).toEqual({ type: 'proficiency', proficiency: 'skill:perception' });
  });

  it('sends a modifier with its formula, its condition and its tags', () => {
    const d = {
      ...emptyEffect('modifier'),
      target: 'speed.walk',
      mode: 'add',
      value: ' 5 ',
      when: 'level() >= 5',
      tags: 'against:magic, about:items ,',
      proficiency: 'skill:perception',
    };
    expect(draftToEffect(d, m)).toEqual({
      type: 'modifier',
      target: 'speed.walk',
      mode: 'add',
      value: '5',
      when: 'level() >= 5',
      tags: ['against:magic', 'about:items'],
    });
  });

  it('sends a sense with its range in feet, a choice with its list, a note with the text and the granted spells', () => {
    expect(
      draftToEffect({ ...emptyEffect('sense'), sense: 'darkvision', rangeM: '18' }, m),
    ).toEqual({ type: 'sense', sense: 'darkvision', rangeFt: 60 });
    expect(
      draftToEffect(
        {
          ...emptyEffect('choice'),
          choice: 'skill',
          count: 2,
          from: ['skill:arcana'],
          level: 'half',
        },
        m,
      ),
    ).toEqual({ type: 'choice', choice: 'skill', count: 2, from: ['skill:arcana'] });
    expect(
      draftToEffect({ ...emptyEffect('note'), textPt: 'Cai devagar.', spells: ['spell:light'] }, m),
    ).toEqual({ type: 'note', textPt: 'Cai devagar.', spells: ['spell:light'] });
  });

  it('reads a stored effect back and sends it unchanged', () => {
    const stored = create(TableEffectSchema, {
      type: 'modifier',
      target: 'ac',
      mode: 'add',
      value: '1',
      tags: ['against:magic'],
    });
    const d = effectToDraft(stored);
    expect(d.tags).toBe('against:magic');
    expect(draftToEffect(d, m)).toEqual({
      type: 'modifier',
      target: 'ac',
      mode: 'add',
      value: '1',
      tags: ['against:magic'],
    });
  });

  it('keeps a stored range as the metres it shows, and sends what was typed once, in feet', () => {
    const d = effectToDraft(
      create(TableEffectSchema, { type: 'sense', sense: 'darkvision', rangeFt: 60 }),
    );
    expect(d.rangeM).toBe('18');
    expect(draftToEffect(d, m)).toMatchObject({ rangeFt: 60 });
    // 4 m is not a whole number of feet at the table's rate: it is rounded once, on sending.
    expect(draftToEffect({ ...d, rangeM: '4' }, m)).toMatchObject({ rangeFt: 13 });
  });

  it('offers what a choice may list, by the kind of choice', () => {
    expect(m.fromOptions('skill').map((o) => o.key)).toEqual(['skill:arcana', 'skill:perception']);
    expect(m.fromOptions('language').map((o) => o.namePt)).toEqual(['Comum', 'Primordial']);
    expect(m.fromOptions('feature').map((o) => o.namePt)).toEqual(['Estilo de luta']);
    m.classNamePt = (k) => (k === 'class:wizard' ? 'Mago' : k);
    expect(m.fromOptions('spell').map((o) => o.namePt)).toEqual(['Mago', 'class:cleric']);
    expect(m.fromOptions('')).toEqual([]);
  });

  it('says which required field is still empty', () => {
    expect(missingRequired(emptyEffect('modifier'), m)).toEqual(['target', 'mode', 'value']);
    expect(
      missingRequired({ ...emptyEffect('sense'), sense: 'darkvision', rangeM: '18' }, m),
    ).toEqual([]);
  });

  it('reads metres as feet in steps the server checks, and back', () => {
    expect(rangeFeet('18')).toBe(60);
    expect(rangeFeet('4,5')).toBe(15);
    expect(rangeFeet('')).toBe(0);
    expect(rangeFeet('abc')).toBe(0);
    expect(rangeMeters(60)).toBe('18');
    expect(rangeMeters(15)).toBe('4,5');
    expect(rangeMeters(0)).toBe('');
    expect(splitTags(' a, ,b ')).toEqual(['a', 'b']);
  });

  it('tells a range field it cannot read from an empty one', () => {
    for (const text of ['18 m', '18m', '1.000,5', 'abc', '-3']) {
      expect(unreadableRange(text), text).toBe(true);
    }
    for (const text of ['', '  ', '0', '18', '4,5', '4.5']) {
      expect(unreadableRange(text), text).toBe(false);
    }
  });
});

describe('the effects only a feat has (MR-025)', () => {
  const m = menu();

  it('leaves "ability_increase" out of every editor but the feat\'s', () => {
    expect(m.typesFor(false).map((t) => t.type)).not.toContain('ability_increase');
    expect(m.typesFor(true).map((t) => t.type)).toContain('ability_increase');
  });

  it('sends how many, from which abilities and how much, and asks for all three', () => {
    const d = {
      ...emptyEffect('ability_increase'),
      count: 2,
      from: ['strength', 'dexterity'],
      value: '1',
      target: 'ac',
    };
    expect(draftToEffect(d, m)).toEqual({
      type: 'ability_increase',
      count: 2,
      from: ['strength', 'dexterity'],
      value: '1',
    });
    expect(missingRequired(emptyEffect('ability_increase'), m)).toEqual(['count', 'value']);
  });

  it("offers the table's feats to a choice of feats, and none by itself", () => {
    expect(m.fromOptions('feat')).toEqual([]);
    m.featOptions = [{ key: 'feat:mestre@mesa', namePt: 'Mestre', hintPt: '' }];
    expect(m.fromOptions('feat').map((o) => o.namePt)).toEqual(['Mestre']);
  });
});

describe("ranged_attack: a table feat's switches and spell range multiplier", () => {
  const m = rangedAttackMenu();

  it('sends the switches that are on and the multiplier that is set, and asks for which attacks', () => {
    const spell = {
      ...emptyEffect('ranged_attack'),
      applies: 'spell',
      ignoreCover: true,
      rangeMultiplier: 2,
    };
    expect(draftToEffect(spell, m)).toEqual({
      type: 'ranged_attack',
      applies: 'spell',
      ignoreCover: true,
      rangeMultiplier: 2,
    });
    const weapon = {
      ...emptyEffect('ranged_attack'),
      applies: 'weapon',
      noLongRangeDisadvantage: true,
    };
    expect(draftToEffect(weapon, m)).toEqual({
      type: 'ranged_attack',
      applies: 'weapon',
      noLongRangeDisadvantage: true,
    });
    expect(missingRequired({ ...spell, applies: '' }, m)).toEqual(['applies']);
  });

  it('reads a stored one back and sends it unchanged', () => {
    const stored = create(TableEffectSchema, {
      type: 'ranged_attack',
      applies: 'weapon',
      ignoreCover: true,
      noLongRangeDisadvantage: true,
    });
    expect(draftToEffect(effectToDraft(stored), m)).toEqual({
      type: 'ranged_attack',
      applies: 'weapon',
      ignoreCover: true,
      noLongRangeDisadvantage: true,
    });
  });

  it('never sends a switch on another type', () => {
    const d = {
      ...emptyEffect('modifier'),
      target: 'ac',
      mode: 'add',
      value: '1',
      ignoreCover: true,
      rangeMultiplier: 2,
    };
    const sent = draftToEffect(d, m);
    expect(sent).not.toHaveProperty('ignoreCover');
    expect(sent).not.toHaveProperty('rangeMultiplier');
  });
});

describe('resistance: the damage types it covers, its kind and when it holds', () => {
  const m = resistanceMenu();

  it('asks for nothing alone: the server says which of the types or the exception is missing', () => {
    expect(missingRequired(emptyEffect('resistance'), m)).toEqual([]);
  });

  it('sends the types, the kind and the condition, and only what is set', () => {
    const d = {
      ...emptyEffect('resistance'),
      damageTypes: ['damage-type:cold', 'nonmagical_bps'],
      kind: 'immune',
      while: 'rage',
    };
    expect(draftToEffect(d, m)).toEqual({
      type: 'resistance',
      damageTypes: ['damage-type:cold', 'nonmagical_bps'],
      kind: 'immune',
      while: 'rage',
    });
    expect(draftToEffect({ ...emptyEffect('resistance'), damageTypes: ['all'] }, m)).toEqual({
      type: 'resistance',
      damageTypes: ['all'],
    });
  });

  it('still sends the type it leaves out, as before', () => {
    const d = { ...emptyEffect('resistance'), except: 'damage-type:psychic', while: 'rage' };
    expect(draftToEffect(d, m)).toEqual({
      type: 'resistance',
      except: 'damage-type:psychic',
      while: 'rage',
    });
  });

  it('reads a stored one back and sends it unchanged', () => {
    const stored = create(TableEffectSchema, {
      type: 'resistance',
      damageTypes: ['damage-type:poison'],
      kind: 'vulnerable',
    });
    expect(draftToEffect(effectToDraft(stored), m)).toEqual({
      type: 'resistance',
      damageTypes: ['damage-type:poison'],
      kind: 'vulnerable',
    });
  });
});

describe('condition immunity and the roll mode against a condition', () => {
  const m = conditionMenu();

  it('asks for the conditions and sends them', () => {
    expect(missingRequired(emptyEffect('condition_immunity'), m)).toEqual(['conditions']);
    const d = { ...emptyEffect('condition_immunity'), conditions: ['condition:poisoned'] };
    expect(draftToEffect(d, m)).toEqual({
      type: 'condition_immunity',
      conditions: ['condition:poisoned'],
    });
  });

  it('sends the condition of a roll mode only when one is picked', () => {
    const base = { ...emptyEffect('roll_mode'), roll: 'advantage', targets: ['save.wis'] };
    expect(draftToEffect(base, m)).toEqual({
      type: 'roll_mode',
      roll: 'advantage',
      targets: ['save.wis'],
    });
    expect(draftToEffect({ ...base, againstCondition: 'condition:frightened' }, m)).toEqual({
      type: 'roll_mode',
      roll: 'advantage',
      targets: ['save.wis'],
      againstCondition: 'condition:frightened',
    });
  });

  it('reads a stored one back and sends it unchanged', () => {
    const stored = create(TableEffectSchema, {
      type: 'roll_mode',
      roll: 'advantage',
      targets: ['save.wis'],
      againstCondition: 'condition:frightened',
    });
    expect(draftToEffect(effectToDraft(stored), m)).toEqual({
      type: 'roll_mode',
      roll: 'advantage',
      targets: ['save.wis'],
      againstCondition: 'condition:frightened',
    });
  });
});
