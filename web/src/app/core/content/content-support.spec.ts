import { create } from '@bufbuild/protobuf';

import {
  TableBackgroundSchema,
  TableClassSchema,
  TableContentKind,
  TableEntrySchema,
  TableFeatSchema,
  TableRaceSchema,
  TableSpellSchema,
  TableSubclassSchema,
  TableSubraceSchema,
  type TableEntry,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { entrySupport } from './content-kinds';
import { catalog } from './content-testing';

const cat = catalog();
const nameOf = (k: string) => cat.nameOf(k);
const dressed = (kind: TableContentKind, body: TableEntry['body']): TableEntry =>
  create(TableEntrySchema, { key: 'x@mesa', kind, namePt: 'X', body });
const klass = (init: object) =>
  dressed(TableContentKind.CLASS, {
    case: 'tableClass',
    value: create(TableClassSchema, init as never),
  });
const subclass = (init: object) =>
  dressed(TableContentKind.SUBCLASS, {
    case: 'tableSubclass',
    value: create(TableSubclassSchema, init as never),
  });
const race = (init: object) =>
  dressed(TableContentKind.RACE, {
    case: 'tableRace',
    value: create(TableRaceSchema, init as never),
  });

const CASES: Record<string, () => string> = {
  'a class that does not cast': () => entrySupport(klass({ hitDie: 8 }), nameOf),
  'a class that casts, subclass at 2': () =>
    entrySupport(klass({ hitDie: 6, subclassLevel: 2, casting: { kind: 'half' } }), nameOf),
  'a class with a casting kind the app does not know': () =>
    entrySupport(klass({ hitDie: 6, casting: { kind: 'zz' } }), nameOf),
  'a subclass at its own level': () =>
    entrySupport(subclass({ classKey: 'class:wizard', level: 3 }), nameOf),
  'a subclass at the level of the catalog': () =>
    entrySupport(subclass({ classKey: 'class:wizard' }), nameOf, () => 2),
  'a subclass with no level anywhere': () =>
    entrySupport(subclass({ classKey: 'class:wizard' }), nameOf),
  'a race with darkvision': () =>
    entrySupport(race({ size: 'Medium', speedFt: 30, darkvisionFt: 60 }), nameOf),
  'a race of an odd size': () => entrySupport(race({ size: 'Odd', speedFt: 25 }), nameOf),
  'a subrace': () =>
    entrySupport(
      dressed(TableContentKind.SUBRACE, {
        case: 'tableSubrace',
        value: create(TableSubraceSchema, { raceKey: 'race:human' }),
      }),
      nameOf,
    ),
  'a background with one skill': () =>
    entrySupport(
      dressed(TableContentKind.BACKGROUND, {
        case: 'tableBackground',
        value: create(TableBackgroundSchema, { skills: ['skill:arcana'] }),
      }),
      nameOf,
    ),
  'a background with two skills': () =>
    entrySupport(
      dressed(TableContentKind.BACKGROUND, {
        case: 'tableBackground',
        value: create(TableBackgroundSchema, { skills: ['skill:arcana', 'skill:history'] }),
      }),
      nameOf,
    ),
  'a cantrip': () =>
    entrySupport(
      dressed(TableContentKind.SPELL, {
        case: 'tableSpell',
        value: create(TableSpellSchema, { level: 0 }),
      }),
      nameOf,
    ),
  'a leveled spell': () =>
    entrySupport(
      dressed(TableContentKind.SPELL, {
        case: 'tableSpell',
        value: create(TableSpellSchema, { level: 3 }),
      }),
      nameOf,
    ),
  'a feat with no prerequisite': () =>
    entrySupport(
      dressed(TableContentKind.FEAT, {
        case: 'tableFeat',
        value: create(TableFeatSchema, {}),
      }),
      nameOf,
    ),
  'a feat with a prerequisite': () =>
    entrySupport(
      dressed(TableContentKind.FEAT, {
        case: 'tableFeat',
        value: create(TableFeatSchema, { prerequisite: { minimums: { strength: 13 } } } as never),
      }),
      nameOf,
    ),
  'an entry with no body': () =>
    entrySupport(dressed(TableContentKind.CLASS, { case: undefined } as never), nameOf),
};

const EXPECTED: Record<string, string> = {
  'a class that does not cast': 'd8 · sem conjuração · subclasse no nível 3',
  'a class that casts, subclass at 2': 'd6 · conjuração de metade · subclasse no nível 2',
  'a class with a casting kind the app does not know': 'd6 · conjuração zz · subclasse no nível 3',
  'a subclass at its own level': 'Subclasse de Mago · nível 3',
  'a subclass at the level of the catalog': 'Subclasse de Mago · nível 2',
  'a subclass with no level anywhere': 'Subclasse de Mago',
  'a race with darkvision': 'Médio · 9 m · visão no escuro 18 m',
  'a race of an odd size': 'Odd · 7,5 m',
  'a subrace': 'Sub-raça de Humano',
  'a background with one skill': '1 perícia',
  'a background with two skills': '2 perícias',
  'a cantrip': 'Truque',
  'a leveled spell': '3º nível',
  'a feat with no prerequisite': 'Sem pré-requisito',
  'a feat with a prerequisite': 'Força 13',
  'an entry with no body': '',
};

describe('the one-line support of a row, by kind', () => {
  it.each(Object.keys(CASES))('%s', (name) => {
    const got = CASES[name]().replace(/ /g, ' ');
    expect(got).toBe(EXPECTED[name]);
  });
});
