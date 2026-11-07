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
import { offeringName, optionSetLine } from './option-text';
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
const nameOf = catalogVm(create(ContentSchema, {}), [owner]).nameOf;

describe('the feature that offers a table option (MR-025)', () => {
  it('names a feature with its owner, and a key the catalog does not know by its last word', () => {
    expect(offeringName(crow, nameOf)).toBe('Truques de lâmina (Guarda do Corvo)');
    expect(offeringName('feature:class-x-1a--votos-antigos@mesa', nameOf)).toBe('Votos antigos');
  });

  it('says "Opção de <feature> (<owner>)" when the server names it and the set when it does not', () => {
    expect(optionSetLine('table_feature', crow, nameOf)).toBe(
      'Opção de Truques de lâmina (Guarda do Corvo)',
    );
    expect(optionSetLine('table_feature', '', nameOf)).toBe('Opção de característica da mesa');
    expect(entrySupport(option(crow), nameOf)).toContain(
      'Opção de Truques de lâmina (Guarda do Corvo)',
    );
    expect(entrySupport(option(''), nameOf)).toContain('Opção de característica da mesa');
    expect(entrySupport(option(''), nameOf)).not.toContain(crow);
  });

  it('reads "Oferecida por" on the page only when the feature is named', () => {
    const read = (key: string) => {
      const e = option(key);
      return readEntry(e, nameOf, () => 0).rows;
    };
    expect(read(crow)).toContainEqual({
      label: 'Oferecida por',
      value: 'Truques de lâmina (Guarda do Corvo)',
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
