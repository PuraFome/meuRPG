import type { FullSheet } from '../../../gen/meurpg/characters/v1/characters_pb';
import { create } from '@bufbuild/protobuf';
import { DerivedSheetSchema, FeatureSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import { featuresVm } from './sheet-lost';

describe('featuresVm', () => {
  it('carries the name of what offers an option to its row (board 580-05)', () => {
    const derived = create(DerivedSheetSchema, {
      features: [
        create(FeatureSchema, {
          key: 'feat:truques',
          namePt: 'Truques de lâmina',
          sourcePt: 'Talento',
        }),
        create(FeatureSchema, {
          key: 'feat:truques#asa',
          namePt: 'Asa Partida',
          sourcePt: 'Talento',
          optionOfPt: 'Truques de lâmina',
        }),
      ],
    });
    const rows = featuresVm(
      { featKeys: [], featureChoiceKeys: [] } as unknown as FullSheet,
      derived,
    );
    expect(rows.map((r) => r.optionOf)).toEqual(['', 'Truques de lâmina']);
  });
});

describe('featuresVm, the order of the rows', () => {
  const row = (key: string, namePt: string, optionOfPt = '') =>
    create(FeatureSchema, { key, namePt, sourcePt: 'Guerreiro 1', optionOfPt });
  const names = (features: ReturnType<typeof row>[]) =>
    featuresVm(
      { featKeys: [], featureChoiceKeys: [] } as unknown as FullSheet,
      create(DerivedSheetSchema, { features }),
    ).map((r) => r.name);

  it('puts an option right under the feature that offers it, the options in their own order', () => {
    expect(
      names([
        row('feature:estilo', 'Estilo de Luta'),
        row('feat:truques', 'Truques de lâmina'),
        row('feature:estilo-grandes', 'Estilo de Luta: Armas Grandes', 'Estilo de Luta'),
        row('feat:truques#asa', 'Asa Partida', 'Truques de lâmina'),
        row('feature:estilo-defesa', 'Estilo de Luta: Defesa', 'Estilo de Luta'),
        row('feature:fúria', 'Fúria'),
      ]),
    ).toEqual([
      'Estilo de Luta',
      'Estilo de Luta: Armas Grandes',
      'Estilo de Luta: Defesa',
      'Truques de lâmina',
      'Asa Partida',
      'Fúria',
    ]);
  });

  it('keeps an option whose feature is not on the sheet where it is', () => {
    expect(names([row('a', 'A'), row('o', 'Opção', 'Sumiu'), row('b', 'B')])).toEqual([
      'A',
      'Opção',
      'B',
    ]);
  });
});
