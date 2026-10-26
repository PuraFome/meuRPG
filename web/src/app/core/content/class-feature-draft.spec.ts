import { create } from '@bufbuild/protobuf';

import { TableClassFeatureSchema } from '../../../gen/meurpg/rules/v1/table_content_pb';
import {
  classFeaturePaths,
  classFeatureToDraft,
  draftToClassFeature,
  emptyClassFeature,
} from './class-feature-draft';
import { menu } from './content-testing-menu';

const stored = () =>
  create(TableClassFeatureSchema, {
    namePt: 'Rastro do Corvo',
    descPt: ['Primeiro.', 'Segundo.'],
    classKey: 'class:ranger',
    level: 1,
    replacesFeatureKey: 'feature:favored-enemy-1-type',
    effects: [{ type: 'note', textPt: 'Segue o rastro de um corvo.' }],
  });

describe('the class feature form (MR-025)', () => {
  it('turns a stored feature into a draft and back to the same body', () => {
    const draft = classFeatureToDraft(stored());
    expect(draft.feature.text).toBe('Primeiro.\n\nSegundo.');
    expect(draft.level).toBe('1');
    expect(draftToClassFeature(draft, menu())).toEqual({
      namePt: 'Rastro do Corvo',
      descPt: ['Primeiro.', 'Segundo.'],
      classKey: 'class:ranger',
      level: 1,
      replacesFeatureKey: 'feature:favored-enemy-1-type',
      effects: [{ type: 'note', textPt: 'Segue o rastro de um corvo.' }],
    });
  });

  it('sends an empty level as 0 and a feature that replaces nothing as empty', () => {
    const init = draftToClassFeature(emptyClassFeature(), menu());
    expect(init.level).toBe(0);
    expect(init.replacesFeatureKey).toBe('');
    expect(init.effects).toEqual([]);
  });

  it('lists the inputs the editor draws, with the fields of each effect type', () => {
    const paths = classFeaturePaths(classFeatureToDraft(stored()), menu());
    expect(paths).toContain('table_class_feature.class_key');
    expect(paths).toContain('table_class_feature.level');
    expect(paths).toContain('table_class_feature.replaces_feature_key');
    expect(paths).toContain('table_class_feature.effects[0].text_pt');
  });
});
