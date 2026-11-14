import { create, equals } from '@bufbuild/protobuf';

import {
  type TableEffect,
  TableEffectSchema,
  TableFeatSchema,
  TableFeatureSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { menu } from './content-testing-menu';
import { draftToFeat, featToDraft } from './feat-draft';
import { draftToFeature, featureToDraft } from './feature-draft';

const power = (): TableEffect =>
  create(TableEffectSchema, {
    type: 'power_attack',
    attackPenalty: 5,
    damageBonus: 10,
    resource: 'resource:fury',
    weapon: 'melee',
    weaponProperties: ['weapon-property:heavy', 'weapon-property:two-handed'],
    onlyProficient: true,
    textPt: 'Troca precisão por força.',
  });

describe('an effect of a type the editor does not offer is kept as stored', () => {
  it('a feat: changing only the name sends the power_attack effect proto-equal to the loaded one', () => {
    const draft = featToDraft(
      create(TableFeatSchema, { namePt: 'Golpe', descPt: ['Texto.'], effects: [power()] }),
    );
    draft.feature = { ...draft.feature, name: 'Golpe forte' };
    const sent = draftToFeat(draft, menu());
    expect(sent.namePt).toBe('Golpe forte');
    expect(sent.effects).toHaveLength(1);
    expect(equals(TableEffectSchema, create(TableEffectSchema, sent.effects?.[0]), power())).toBe(
      true,
    );
  });

  it('a feature: changing only the name sends the power_attack effect proto-equal to the loaded one', () => {
    const draft = featureToDraft(
      create(TableFeatureSchema, { namePt: 'Fúria', descPt: ['Texto.'], effects: [power()] }),
    );
    draft.name = 'Fúria maior';
    const sent = draftToFeature(draft, menu());
    expect(sent.namePt).toBe('Fúria maior');
    expect(sent.effects).toHaveLength(1);
    expect(equals(TableEffectSchema, create(TableEffectSchema, sent.effects?.[0]), power())).toBe(
      true,
    );
  });
});
