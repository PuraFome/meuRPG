import { create } from '@bufbuild/protobuf';

import { ContentSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import { TableContentKind } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { catalogVm, isTableKey } from './catalog';
import { entry } from './content-testing';

const content = () =>
  create(ContentSchema, {
    abilities: [1, 2, 3, 4, 5, 6].map((a) => ({ ability: a, namePt: 'A' + a })),
    races: [
      { key: 'race:human', namePt: 'Humano' },
      { key: 'race:old', namePt: 'Antigo', archived: true },
    ],
    subraces: [
      { key: 'subrace:x', namePt: 'Sub' },
      { key: 'subrace:y', namePt: 'Velha', archived: true },
    ],
    classes: [
      { key: 'class:wizard', namePt: 'Mago', subclassLevel: 2, spellcasting: { ability: 4 } },
      { key: 'class:old', namePt: 'Velha', subclassLevel: 3, archived: true },
      { key: 'class:fighter', namePt: 'Guerreiro', subclassLevel: 3 },
      { key: 'class:mesa@mesa', namePt: 'Da mesa', subclassLevel: 3 },
      { key: 'class:conj@mesa', namePt: 'Conjuradora', spellcasting: { ability: 4 } },
    ],
    skills: [{ key: 'skill:arcana', namePt: 'Arcanismo' }],
    spells: [
      {
        key: 'spell:light',
        namePt: 'Luz',
        schoolKey: 'school:evocation',
        schoolNamePt: 'Evocação',
      },
      { key: 'spell:b', namePt: 'Bola', schoolKey: 'school:evocation', schoolNamePt: 'Outro nome' },
      { key: 'spell:c', namePt: 'Cura', schoolKey: '', schoolNamePt: '', archived: true },
      { key: 'spell:d', namePt: 'Dom', schoolKey: 'school:abjuration', schoolNamePt: 'Abjuração' },
    ],
    languages: [{ key: 'language:common', namePt: 'Comum' }],
    proficiencies: [{ key: 'proficiency:shields', namePt: 'escudos' }],
    damageTypes: [{ key: 'damage-type:fire', namePt: 'fogo' }],
    backgrounds: [{ key: 'background:a', namePt: 'Fundo' }],
    subclasses: [{ key: 'subclass:a', namePt: 'Sub da classe' }],
  } as never);

describe('the catalog the editors read', () => {
  const vm = catalogVm(content(), [entry(TableContentKind.RACE, 'Da mesa')]);

  it('knows a table key by its ending', () => {
    expect(isTableKey('race:x@mesa')).toBe(true);
    expect(isTableKey('race:human')).toBe(false);
  });

  it('names every key of the catalog and of the table, and gives back a key it does not know', () => {
    expect(vm.nameOf('race:human')).toBe('Humano');
    expect(vm.nameOf('subrace:y')).toBe('Velha');
    expect(vm.nameOf('subclass:a')).toBe('Sub da classe');
    expect(vm.nameOf('background:a')).toBe('Fundo');
    expect(vm.nameOf('skill:arcana')).toBe('Arcanismo');
    expect(vm.nameOf('spell:c')).toBe('Cura');
    expect(vm.nameOf('language:common')).toBe('Comum');
    expect(vm.nameOf('proficiency:shields')).toBe('escudos');
    expect(vm.nameOf('damage-type:fire')).toBe('fogo');
    expect(vm.nameOf('race:da-mesa@mesa')).toBe('Da mesa');
    expect(vm.nameOf('ability:strength')).toBe('A1');
    expect(vm.nameOf('ability:6')).toBe('A6');
    expect(vm.nameOf('nothing:here')).toBe('nothing:here');
  });

  it('lists the abilities by field, in the order of the sheet', () => {
    expect(vm.abilities.map((a) => [a.field, a.ability, a.name])).toEqual([
      ['strength', 1, 'A1'],
      ['dexterity', 2, 'A2'],
      ['constitution', 3, 'A3'],
      ['intelligence', 4, 'A4'],
      ['wisdom', 5, 'A5'],
      ['charisma', 6, 'A6'],
    ]);
  });

  it('leaves the archived out of the pickers and sorts the labels in Portuguese', () => {
    expect(vm.races).toEqual([{ value: 'race:human', label: 'Humano' }]);
    expect(vm.classes.map((c) => c.label)).toEqual([
      'Conjuradora (da mesa)',
      'Da mesa (da mesa)',
      'Guerreiro',
      'Mago',
    ]);
    expect(vm.raceChoices.map((r) => r.label)).toEqual(['Humano', 'Sub (sub-raça)']);
    expect(vm.spells.map((s) => s.label)).toEqual(['Bola', 'Dom', 'Luz']);
    expect(vm.skills).toEqual([{ value: 'skill:arcana', label: 'Arcanismo' }]);
    expect(vm.proficiencies).toEqual([{ value: 'proficiency:shields', label: 'Escudos' }]);
    expect(vm.damageTypes).toEqual([{ value: 'damage-type:fire', label: 'Fogo' }]);
  });

  it('lists the casting classes with the catalog first and the table last', () => {
    expect(vm.castingClasses).toEqual([
      { key: 'class:wizard', name: 'Mago', table: false },
      { key: 'class:conj@mesa', name: 'Conjuradora', table: true },
      { key: 'class:mesa@mesa', name: 'Da mesa', table: true },
    ]);
  });

  it('names a school once, by the first spell that has it, and not for a spell with none', () => {
    expect(vm.schools).toEqual([
      { value: 'school:abjuration', label: 'Abjuração' },
      { value: 'school:evocation', label: 'Evocação' },
    ]);
  });

  it('says the level a class chooses its subclass at, 0 for a class it does not know', () => {
    expect(vm.subclassLevelOf('class:wizard')).toBe(2);
    expect(vm.subclassLevelOf('class:fighter')).toBe(3);
    expect(vm.subclassLevelOf('class:zzz')).toBe(0);
  });

  it('builds from a catalog with no table entries', () => {
    expect(catalogVm(content()).nameOf('race:da-mesa@mesa')).toBe('race:da-mesa@mesa');
  });
});
