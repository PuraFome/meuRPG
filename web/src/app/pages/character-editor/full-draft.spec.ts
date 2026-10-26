import { FormBuilder } from '@angular/forms';

import {
  type DraftParts,
  buildFullValue,
  choicesAfterAnswer,
  classProblemsOf,
  invalidFullFieldsOf,
  rollProblemsOf,
  serverDraftNeeded,
} from './full-draft';
import { createFullForm, fullFormValueOf } from './full-form';

const fb = new FormBuilder();

function parts(over: Partial<DraftParts> = {}): DraftParts {
  const form = createFullForm(fb);
  form.patchValue({
    name: 'Tharn',
    race: 'race:gnome',
    className: 'class:wizard',
    background: 'background:acolyte',
  });
  return {
    form: form.getRawValue(),
    customBackgroundSkills: new Set(),
    customBackgroundProficiencies: [],
    extraClasses: [],
    chosenSkills: new Set(['skill:arcana']),
    expertiseSkills: new Set(),
    hitPointsRolls: [4, 5, 6],
    rollsNeeded: 2,
    isCaster: true,
    cantrips: ['spell:fire-bolt'],
    spellsKnown: [],
    spellsPrepared: [],
    featureChoiceKeys: ['feature:style'],
    featureChoiceText: { 'choice:a#0': 'text' },
    feats: null,
    ...over,
  };
}

describe('buildFullValue', () => {
  it('carries the form, the picks and as many rolls as the levels need', () => {
    const full = buildFullValue(parts());
    expect(full).toMatchObject({
      name: 'Tharn',
      race: 'race:gnome',
      className: 'class:wizard',
      skillProficiencies: ['skill:arcana'],
      hitPointsRolls: [4, 5],
      isCaster: true,
      cantrips: ['spell:fire-bolt'],
      featureChoiceKeys: ['feature:style'],
      featureChoiceText: { 'choice:a#0': 'text' },
      weapons: [],
    });
    expect(full.featKeys).toBeUndefined();
    expect(full.customBackgroundSkills).toBeNull();
  });

  it('leaves a blank extra class out', () => {
    const full = buildFullValue(
      parts({
        extraClasses: [
          { classKey: '', level: 1, subclassKey: '', customSubclassName: '' },
          { classKey: 'class:fighter', level: 2, subclassKey: '', customSubclassName: '' },
        ],
      }),
    );
    expect(full.extraClasses.map((c) => c.classKey)).toEqual(['class:fighter']);
  });

  it('sends the "Outro" background\'s own fields only for it, the skills only when exactly two', () => {
    const form = createFullForm(fb);
    form.patchValue({
      background: 'custom',
      customBackgroundFeatureName: 'Nome',
      customBackgroundFeatureText: 'Texto',
      customBackgroundEquipment: 'Corda',
    });
    const custom = parts({
      form: form.getRawValue(),
      customBackgroundSkills: new Set(['a', 'b']),
      customBackgroundProficiencies: ['p'],
    });
    expect(buildFullValue(custom)).toMatchObject({
      customBackgroundSkills: ['a', 'b'],
      customBackgroundProficiencies: ['p'],
      customBackgroundFeatureName: 'Nome',
      customBackgroundFeatureText: 'Texto',
      customBackgroundEquipment: 'Corda',
    });
    expect(
      buildFullValue({ ...custom, customBackgroundSkills: new Set(['a']) }).customBackgroundSkills,
    ).toBeNull();
    const plain = parts({
      customBackgroundSkills: new Set(['a', 'b']),
      customBackgroundProficiencies: ['p'],
    });
    expect(buildFullValue(plain)).toMatchObject({
      customBackgroundSkills: null,
      customBackgroundProficiencies: [],
      customBackgroundFeatureName: '',
      customBackgroundEquipment: '',
    });
  });

  it("sends the feats only when the master's draft has them, as copies", () => {
    const slots = { 'feat:alert': 'str' };
    const full = buildFullValue(parts({ feats: { keys: ['feat:alert'], slots } }));
    expect(full.featKeys).toEqual(['feat:alert']);
    expect(full.featSlots).toEqual(slots);
    expect(full.featSlots).not.toBe(slots);
  });
});

describe('fullFormValueOf', () => {
  it('is what the form gives back for a saved sheet', () => {
    const form = createFullForm(fb);
    const saved = buildFullValue(parts());
    form.setValue(fullFormValueOf(saved));
    expect(buildFullValue(parts({ form: form.getRawValue() }))).toEqual(saved);
  });
});

describe('classProblemsOf', () => {
  const extra = (classKey: string, level: number) => ({
    classKey,
    level,
    subclassKey: '',
    customSubclassName: '',
  });

  it('names the blocks that lack a class or a valid level, and the total', () => {
    expect(
      classProblemsOf(
        [extra('', 1), extra('class:a', 0), extra('class:b', 25), extra('class:c', 1.5)],
        'x',
      ),
    ).toEqual([
      'Classe 2',
      'Nível da classe 3',
      'Nível da classe 4',
      'Nível da classe 5',
      'o nível total vai até 20',
    ]);
  });

  it('is empty for blocks that are fine', () => {
    expect(classProblemsOf([extra('class:a', 3)], '')).toEqual([]);
  });
});

describe('rollProblemsOf', () => {
  it('names the levels whose roll is missing or does not fit the die of that level', () => {
    expect(rollProblemsOf(true, 8, 3, [6, 10, 8], [4, 11])).toEqual([
      'Dado de vida do nível 3',
      'Dado de vida do nível 4',
    ]);
  });

  it('rolls against the first die where a level has none', () => {
    expect(rollProblemsOf(true, 8, 2, [], [8, 9])).toEqual(['Dado de vida do nível 3']);
  });

  it('asks nothing of the average, or before a class is chosen', () => {
    expect(rollProblemsOf(false, 8, 2, [], [])).toEqual([]);
    expect(rollProblemsOf(true, 0, 2, [], [])).toEqual([]);
  });
});

describe('invalidFullFieldsOf', () => {
  it("adds the class and roll problems to the form's invalid fields, on their steps", () => {
    const form = createFullForm(fb);
    const fields = invalidFullFieldsOf(form, ['Classe 2'], ['Dado de vida do nível 2'], {
      incomplete: false,
      problem: '',
    });
    expect(fields.slice(-2)).toEqual([
      { path: 'classes', label: 'Classe 2', step: 'basico' },
      { path: 'hitPointsRolls', label: 'Dado de vida do nível 2', step: 'atributos' },
    ]);
    expect(fields.length).toBeGreaterThan(2);
  });

  it('adds the unplaced results while the placing is unfinished, in the words of its problem', () => {
    const form = createFullForm(fb);
    const base = invalidFullFieldsOf(form, [], [], { incomplete: false, problem: '' });
    const plain = invalidFullFieldsOf(form, [], [], { incomplete: true, problem: '' });
    const worded = invalidFullFieldsOf(form, [], [], {
      incomplete: true,
      problem: 'role as habilidades',
    });
    expect(plain.length).toBe(base.length + 1);
    expect(plain.at(-1)?.step).toBe('atributos');
    expect(worded.at(-1)).toMatchObject({ label: 'role as habilidades', step: 'atributos' });
  });
});

describe('serverDraftNeeded', () => {
  it('asks while a box that needs the server is on, and only on an open page', () => {
    const off = { ready: true, rolledBox: false, caster: false, feats: false };
    expect(serverDraftNeeded(off)).toBe(false);
    expect(serverDraftNeeded({ ...off, rolledBox: true })).toBe(true);
    expect(serverDraftNeeded({ ...off, caster: true })).toBe(true);
    expect(serverDraftNeeded({ ...off, feats: true })).toBe(true);
    expect(serverDraftNeeded({ ready: false, rolledBox: true, caster: true, feats: true })).toBe(
      false,
    );
  });
});

describe('choicesAfterAnswer', () => {
  const answer = (notOffered: string[], keys: string[]) => ({
    notOffered,
    groups: [{ choices: keys.map((key) => ({ key })) }],
  });

  it('drops the picks that are not offered and the texts of a choice that is gone', () => {
    const next = choicesAfterAnswer(
      answer(['feature:gone'], ['choice:keep']),
      ['feature:gone', 'feature:keep'],
      { 'choice:keep#0': 'a', 'choice:gone#1': 'b' },
    );
    expect(next).toEqual({ keys: ['feature:keep'], texts: { 'choice:keep#0': 'a' } });
  });

  it('changes nothing when everything is still offered', () => {
    expect(
      choicesAfterAnswer(answer([], ['choice:keep']), ['feature:keep'], { 'choice:keep#0': 'a' }),
    ).toEqual({
      keys: null,
      texts: null,
    });
  });

  it('returns no keys when the answer lists a key the draft does not hold (the draft is not asked again)', () => {
    const next = choicesAfterAnswer(
      answer(['race:x#bonus-2=ability:str'], ['choice:keep']),
      [],
      {},
    );
    expect(next.keys).toBeNull();
    const other = choicesAfterAnswer(
      answer(['feature:gone'], ['choice:keep']),
      ['feature:keep'],
      {},
    );
    expect(other.keys).toBeNull();
  });
});
