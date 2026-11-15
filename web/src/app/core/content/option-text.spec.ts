import { create } from '@bufbuild/protobuf';

import {
  TableContentKind,
  TableOptionSchema,
  TableClassSchema,
  TableSubclassSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { catalogVm } from './catalog';
import { ContentSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import { entry, feature } from './content-testing';
import { entrySupport } from './content-kinds';
import { readEntry } from './content-read';
import { featureKeyName } from './class-feature-text';
import { optionSetLine, optionSupport } from './option-text';
import { OptionPrerequisiteSchema } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { FeatPrerequisiteSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import { prerequisiteSupport } from './feat-text';
import { offeringFeatures } from './offering-features';

const crow = 'feature:class-guardiao-3f9a1c--truques-de-lamina@mesa';
const withOptions = {
  ...feature('Truques de lâmina'),
  key: crow,
  options: [feature('Corte baixo')],
};
const owner = entry(TableContentKind.SUBCLASS, 'Guarda do Corvo', {
  body: {
    case: 'tableSubclass',
    value: create(TableSubclassSchema, {
      namePt: 'Guarda do Corvo',
      classKey: 'class:fighter',
      levels: [{ level: 3, features: [withOptions] }],
    }),
  },
});
const option = (offeredBy: string) =>
  entry(TableContentKind.OPTION, 'Golpe do Corvo', {
    body: {
      case: 'tableOption',
      value: create(TableOptionSchema, { namePt: 'Golpe', set: 'table_feature', offeredBy }),
    },
  });
const cat = catalogVm(create(ContentSchema, {}), [owner]);
const nameOf = cat.nameOf;
const named = catalogVm(
  create(ContentSchema, { classes: [{ key: 'class:fighter', namePt: 'Guerreiro' }] }),
  [owner],
);

describe('the feature that offers a table option (MR-025)', () => {
  it('names a feature with its owner for the offering lines only, and by its own name everywhere else', () => {
    expect(cat.offeredByName(crow)).toBe('Truques de lâmina (Guarda do Corvo)');
    expect(nameOf(crow)).toBe(crow);
    expect(featureKeyName('feature:class-x-1a--votos-antigos@mesa', nameOf)).toBe('Votos antigos');
    expect(cat.offeredByName('feature:class-x-1a--votos-antigos@mesa')).toBe('Votos antigos');
  });

  it('names the feature of an archived owner for the saved options that still name it', () => {
    const archived = { ...owner, archived: true };
    const c = catalogVm(create(ContentSchema, {}), [archived]);
    expect(c.offeredByName(crow)).toBe('Truques de lâmina (Guarda do Corvo)');
    expect(offeringFeatures([archived])).toEqual([]);
    expect(offeringFeatures([archived], true)).toHaveLength(1);
  });

  it('says "Opção de <feature> (<owner>)" when the server names it and the set when it does not', () => {
    expect(optionSetLine('table_feature', crow, cat.offeredByName)).toBe(
      'Opção de Truques de lâmina (Guarda do Corvo)',
    );
    expect(optionSetLine('table_feature', '', cat.offeredByName)).toBe(
      'Opção de característica da mesa',
    );
    expect(entrySupport(option(crow), nameOf, () => 0, cat.offeredByName)).toBe(
      'Opção de Truques de lâmina (Guarda do Corvo)',
    );
    expect(entrySupport(option(''), nameOf)).toBe('Opção de característica da mesa');
    expect(entrySupport(option(''), nameOf)).not.toContain(crow);
  });

  it('reads "Oferecida por" on the page only when the feature is named', () => {
    const read = (key: string) => {
      const e = option(key);
      return readEntry(e, nameOf, () => 0, undefined, named.offeredByRead).rows;
    };
    expect(read(crow)).toContainEqual({
      label: 'Oferecida por',
      value: 'Truques de lâmina (Guarda do Corvo, Guerreiro 3)',
    });
    expect(read('').map((r) => r.label)).not.toContain('Oferecida por');
  });

  it('lists the features of a class by the level of their row and ignores the ones with no options', () => {
    const cls = entry(TableContentKind.CLASS, 'Guardião do Vale', {
      body: {
        case: 'tableClass',
        value: create(TableClassSchema, {
          namePt: 'Guardião do Vale',
          levels: [{ features: [feature('Sem lista')] }, { features: [withOptions] }],
        }),
      },
    });
    const found = offeringFeatures([cls, owner]);
    expect(found.map((f) => [f.owner, f.level, f.ownerKind])).toEqual([
      ['Guardião do Vale', 2, 'class'],
      ['Guarda do Corvo', 3, 'subclass'],
    ]);
  });
});

describe('the support line of an option list row', () => {
  it('names the set and nothing more when the option asks nothing', () => {
    expect(optionSupport('invocation', undefined, (key) => key)).toEqual(['Invocação mística']);
  });
});

describe('the prerequisite of an option is said as the feat says it (board 580-02)', () => {
  const names: Record<string, string> = {
    'class:warlock': 'Bruxo',
    'feature:pact-of-the-blade': 'Pacto da Lâmina',
    'spell:eldritch-blast': 'Rajada Mística',
    'feature:devils-sight': 'Olhar de Brasa',
  };
  const nameOf = (key: string) => names[key] ?? key;
  const asks = create(OptionPrerequisiteSchema, {
    classLevel: { classKey: 'class:warlock', level: 5 },
    requiresPact: 'feature:pact-of-the-blade',
    requiresSpellKey: 'spell:eldritch-blast',
    requiresOptionKeys: ['feature:devils-sight'],
  });
  const line = 'Bruxo 5, Pacto da Lâmina, Rajada Mística e Olhar de Brasa';

  it('lists the class level, the pact, the spell and the options in the order of the board', () => {
    expect(optionSupport('invocation', asks, nameOf)).toEqual(['Invocação mística', `Exige ${line}`]);
  });

  it('is the same words the feat uses for the same conditions', () => {
    expect(
      prerequisiteSupport(
        create(FeatPrerequisiteSchema, {
          classLevel: { classKey: 'class:warlock', level: 5 },
          requiresPact: asks.requiresPact,
          requiresSpellKey: asks.requiresSpellKey,
          requiresOptionKeys: asks.requiresOptionKeys,
        }),
        nameOf,
      ),
    ).toBe(line);
  });
});
