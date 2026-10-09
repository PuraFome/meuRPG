import { create } from '@bufbuild/protobuf';

import { TableFeatSchema } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { draftToFeat, emptyFeat, featPaths, featToDraft } from './feat-draft';
import { menu } from './content-testing';

const stored = () =>
  create(TableFeatSchema, {
    namePt: 'Mestre das Cordas',
    descPt: ['Primeiro.', 'Segundo.'],
    prerequisite: {
      minimums: { strength: 13 },
      anyOf: { dexterity: 13, wisdom: 13 },
      proficiencyKey: 'proficiency:medium-armor',
      raceKey: 'race:human',
      level: 4,
    },
    effects: [{ type: 'ability_increase', count: 1, from: ['strength', 'dexterity'], value: '1' }],
  });

describe('the feat form (MR-025)', () => {
  it('turns a stored feat into a draft and back to the same body', () => {
    const draft = featToDraft(stored());
    expect(draft.feature.text).toBe('Primeiro.\n\nSegundo.');
    expect(draft.minimums.strength).toBe('13');
    expect(draft.minimums.dexterity).toBe('');
    expect(draft.level).toBe('4');
    expect(draftToFeat(draft, menu())).toEqual({
      namePt: 'Mestre das Cordas',
      descPt: ['Primeiro.', 'Segundo.'],
      prerequisite: {
        minimums: { strength: 13 },
        anyOf: { dexterity: 13, wisdom: 13 },
        proficiencyKey: 'proficiency:medium-armor',
        spellcasting: false,
        raceKey: 'race:human',
        level: 4,
      },
      effects: [
        { type: 'ability_increase', count: 1, from: ['strength', 'dexterity'], value: '1' },
      ],
    });
  });

  it('sends a prerequisite that asks nothing as empty, and keeps no key on the feat', () => {
    const init = draftToFeat(
      { ...emptyFeat(), feature: { ...emptyFeat().feature, name: '  Livre ' } },
      menu(),
    );
    expect(init.namePt).toBe('Livre');
    expect(init.prerequisite).toEqual({
      proficiencyKey: '',
      spellcasting: false,
      raceKey: '',
      level: 0,
    });
    expect(init.effects).toEqual([]);
  });

  it('lists the inputs the editor draws, with the fields the effect type reads', () => {
    const paths = featPaths(featToDraft(stored()), menu());
    expect(paths).toEqual(
      expect.arrayContaining([
        'table_feat.name_pt',
        'table_feat.prerequisite.minimums.strength',
        'table_feat.prerequisite.level',
        'table_feat.effects[0].type',
        'table_feat.effects[0].from',
        'table_feat.effects[0].value',
      ]),
    );
  });
});
