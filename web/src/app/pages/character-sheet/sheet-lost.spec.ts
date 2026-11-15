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
