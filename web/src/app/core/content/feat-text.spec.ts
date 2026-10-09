import { create } from '@bufbuild/protobuf';

import {
  Ability,
  FeatPrerequisiteSchema,
  FeatUnmetKind,
  FeatUnmetSchema,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { prerequisiteParts, prerequisiteSupport, unmetText } from './feat-text';

const names: Record<string, string> = {
  'proficiency:medium-armor': 'Armadura média',
  'race:dwarf': 'Anão',
};
const nameOf = (key: string) => names[key] ?? key;

describe('what a feat asks, in Portuguese', () => {
  it('writes every condition of a prerequisite, in the sheet order', () => {
    const p = create(FeatPrerequisiteSchema, {
      minimums: { dexterity: 13, strength: 13 },
      anyOf: { intelligence: 13, wisdom: 13 },
      proficiencyKey: 'proficiency:medium-armor',
      spellcasting: true,
      raceKey: 'race:dwarf',
      level: 4,
    });
    expect(prerequisiteParts(p, nameOf)).toEqual([
      'Força 13 e Destreza 13',
      'Inteligência 13 ou Sabedoria 13 (uma delas)',
      'proficiência em Armadura média',
      'poder conjurar ao menos uma magia',
      'ser Anão',
      'nível 4 ou mais',
    ]);
    expect(prerequisiteSupport(p, nameOf)).toContain('Força 13 e Destreza 13, ');
  });

  it('says nothing for a feat that asks nothing', () => {
    expect(prerequisiteParts(undefined, nameOf)).toEqual([]);
    expect(prerequisiteSupport(create(FeatPrerequisiteSchema), nameOf)).toBe('');
  });

  it("writes what a character lacks from the server's codes", () => {
    const unmet = (init: Parameters<typeof create<typeof FeatUnmetSchema>>[1]) =>
      unmetText(create(FeatUnmetSchema, init), nameOf);
    expect(
      unmet({
        kind: FeatUnmetKind.ABILITY_MINIMUM,
        abilities: [{ ability: Ability.STRENGTH, minimum: 13 }],
      }),
    ).toBe('Precisa de Força 13.');
    expect(
      unmet({
        kind: FeatUnmetKind.ABILITY_ANY_OF,
        abilities: [
          { ability: Ability.INTELLIGENCE, minimum: 13 },
          { ability: Ability.WISDOM, minimum: 13 },
        ],
      }),
    ).toBe('Precisa de Inteligência 13 ou Sabedoria 13 (basta uma).');
    expect(unmet({ kind: FeatUnmetKind.PROFICIENCY, key: 'proficiency:medium-armor' })).toBe(
      'Precisa de proficiência em Armadura média.',
    );
    expect(unmet({ kind: FeatUnmetKind.SPELLCASTING })).toBe(
      'Precisa poder conjurar ao menos uma magia.',
    );
    expect(unmet({ kind: FeatUnmetKind.RACE, key: 'race:dwarf' })).toBe('Precisa ser Anão.');
    expect(unmet({ kind: FeatUnmetKind.LEVEL, value: 8 })).toBe('Precisa do nível 8 ou mais.');
    expect(unmet({ kind: FeatUnmetKind.ABILITY_CAP })).toContain('já estão em 20');
  });
});
