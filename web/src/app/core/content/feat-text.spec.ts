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
  'race:elf': 'Elfo',
  'trait:darkvision': 'Visão no Escuro',
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

  it('writes the races to choose from, the size and the feature to lack', () => {
    const p = create(FeatPrerequisiteSchema, {
      races: ['race:elf', 'race:dwarf'],
      size: 'Small',
      lacksFeatureKey: 'trait:darkvision',
      level: 2,
    });
    expect(prerequisiteParts(p, nameOf)).toEqual([
      'ser Elfo ou Anão',
      'ter tamanho pequeno',
      'não ter Visão no Escuro',
      'nível 2 ou mais',
    ]);
    // The single race_key is read together with the list, each race once.
    expect(
      prerequisiteParts(
        create(FeatPrerequisiteSchema, { raceKey: 'race:elf', races: ['race:elf', 'race:dwarf'] }),
        nameOf,
      ),
    ).toEqual(['ser Elfo ou Anão']);
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
    expect(unmet({ kind: FeatUnmetKind.RACE, keys: ['race:elf', 'race:dwarf'] })).toBe(
      'Precisa ser Elfo ou Anão.',
    );
    expect(unmet({ kind: FeatUnmetKind.SIZE, key: 'Medium' })).toBe('Precisa ter tamanho médio.');
    expect(unmet({ kind: FeatUnmetKind.LACKS_FEATURE, key: 'trait:darkvision' })).toBe(
      'Não pode ter Visão no Escuro.',
    );
    expect(unmet({ kind: FeatUnmetKind.LACKS_FEATURE, key: 'x', namePt: 'Dom do Mar' })).toBe(
      'Não pode ter Dom do Mar.',
    );
    // What the players cannot see is blanked by the server: no empty name is written.
    expect(unmet({ kind: FeatUnmetKind.LACKS_FEATURE })).toBe('Não atende ao pré-requisito.');
    expect(unmet({ kind: FeatUnmetKind.RACE })).toBe('Não atende ao pré-requisito.');
    expect(prerequisiteParts(create(FeatPrerequisiteSchema, { size: 'Large' }), nameOf)).toEqual([
      'ter tamanho grande',
    ]);
    expect(unmet({ kind: FeatUnmetKind.RACE, keys: ['race:elf', 'race:dwarf', 'race:elf'] })).toBe(
      'Precisa ser Elfo, Anão ou Elfo.',
    );
    expect(unmet({ kind: FeatUnmetKind.LEVEL, value: 8 })).toBe('Precisa do nível 8 ou mais.');
  });
});
