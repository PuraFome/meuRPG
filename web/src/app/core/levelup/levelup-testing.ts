import { create, type MessageInitShape } from '@bufbuild/protobuf';

import {
  LevelUpClassChoiceSchema,
  LevelUpClassUnavailable,
  LevelUpDiceRule,
  LevelUpOptionsSchema,
  LevelUpSpellsKind,
  type LevelUpClassChoice,
  type LevelUpOptions,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import {
  Ability,
  DerivedAbilitySchema,
  DerivedClassSchema,
  DerivedSheetSchema,
  FeatOptionSchema,
  FeatUnmetKind,
  type FeatOption,
  DerivedSkillSchema,
  HitDiceSchema,
  ProficiencyLevel,
  SavingThrowSchema,
  SkillSchema,
  SpellSchema,
  SpellSlotsSchema,
  SpellcastingSchema,
  type DerivedSheet,
  type Skill,
  type Spell,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import type { SheetKeys } from './levelup-flow';

/** Fixtures of the guided level-up's specs: Pensantus (Mago 3, E8-15's numbers) and a fighter. */

/** One way the level can go, as the class step lists it. */
export function classChoice(
  over: MessageInitShape<typeof LevelUpClassChoiceSchema> = {},
): LevelUpClassChoice {
  return create(LevelUpClassChoiceSchema, { available: true, prerequisiteMet: true, ...over });
}

/** Pensantus's step: the Mago he has, then a Guerreiro he may take (Strength or Dexterity) and a Paladino he may not. */
export function pensantusClassChoices(): LevelUpClassChoice[] {
  return [
    classChoice({
      classKey: 'class:wizard',
      namePt: 'Mago',
      fromLevel: 3,
      toLevel: 4,
      subclassNamePt: 'Evocação',
      prerequisites: [{ ability: Ability.INTELLIGENCE, minimum: 13, have: 18, met: true }],
    }),
    classChoice({
      classKey: 'class:fighter',
      namePt: 'Guerreiro',
      isNew: true,
      fromLevel: 0,
      toLevel: 1,
      prerequisiteAnyOf: true,
      prerequisites: [
        { ability: Ability.STRENGTH, minimum: 13, have: 12, met: false },
        { ability: Ability.DEXTERITY, minimum: 13, have: 16, met: true },
      ],
    }),
    classChoice({
      classKey: 'class:paladin',
      namePt: 'Paladino',
      isNew: true,
      fromLevel: 0,
      toLevel: 1,
      available: false,
      prerequisiteMet: false,
      unavailable: LevelUpClassUnavailable.PREREQUISITE,
      prerequisites: [
        { ability: Ability.STRENGTH, minimum: 13, have: 12, met: false },
        { ability: Ability.CHARISMA, minimum: 13, have: 12, met: false },
      ],
    }),
  ];
}

export function wizardOptions(
  over: MessageInitShape<typeof LevelUpOptionsSchema> = {},
): LevelUpOptions {
  return create(LevelUpOptionsSchema, {
    classKey: 'class:wizard',
    classNamePt: 'Mago',
    classChoices: pensantusClassChoices(),
    fromLevel: 3,
    toLevel: 4,
    totalFromLevel: 3,
    totalToLevel: 4,
    hitDie: 6,
    hitPointAverage: 4,
    diceRule: LevelUpDiceRule.PLAYER_CHOOSES,
    abilityScoreImprovement: true,
    cantrips: 1,
    spells: 2,
    spellsKind: LevelUpSpellsKind.SPELLBOOK,
    spellListClassKey: 'class:wizard',
    maxSpellLevel: 2,
    prepares: true,
    preparedMax: 7,
    preparedMaxAfter: 8,
    proficiencyBonusBefore: 2,
    proficiencyBonusAfter: 2,
    spellSlotsBefore: [4, 2],
    spellSlotsAfter: [4, 3],
    newFeatures: [
      {
        key: 'feature:wizard-ability-score-improvement-1',
        namePt: 'Incremento no Valor de Habilidade',
      },
    ],
    ...over,
  });
}

/** Toren, Guerreiro 4 -> 5: Ataque Extra and the proficiency bonus, nothing to choose but the hit points. */
export function fighterOptions(
  over: MessageInitShape<typeof LevelUpOptionsSchema> = {},
): LevelUpOptions {
  return create(LevelUpOptionsSchema, {
    classKey: 'class:fighter',
    classNamePt: 'Guerreiro',
    classChoices: [
      classChoice({ classKey: 'class:fighter', namePt: 'Guerreiro', fromLevel: 4, toLevel: 5 }),
    ],
    fromLevel: 4,
    toLevel: 5,
    totalFromLevel: 4,
    totalToLevel: 5,
    hitDie: 10,
    hitPointAverage: 6,
    diceRule: LevelUpDiceRule.PLAYER_CHOOSES,
    proficiencyBonusBefore: 2,
    proficiencyBonusAfter: 3,
    newFeatures: [{ key: 'feature:extra-attack', namePt: 'Ataque Extra' }],
    ...over,
  });
}

const sp = (
  key: string,
  namePt: string,
  level: number,
  school: string,
  extra: MessageInitShape<typeof SpellSchema> = {},
): Spell =>
  create(SpellSchema, {
    key,
    namePt,
    level,
    schoolNamePt: school,
    classKeys: ['class:wizard'],
    ...extra,
  });

/** A small slice of the wizard's list: enough to pick, search and prepare. */
export const SPELLS: Spell[] = [
  sp('spell:prestidigitation', 'Prestidigitação', 0, 'Transmutação'),
  sp('spell:mage-hand', 'Mãos Mágicas', 0, 'Conjuração'),
  sp('spell:light', 'Luz', 0, 'Evocação'),
  sp('spell:shocking-grasp', 'Toque Chocante', 0, 'Evocação'),
  sp('spell:fire-bolt', 'Raio de Fogo', 0, 'Evocação'),
  sp('spell:magic-missile', 'Mísseis Mágicos', 1, 'Evocação'),
  sp('spell:detect-magic', 'Detectar Magia', 1, 'Adivinhação', { ritual: true }),
  sp('spell:thunderwave', 'Onda Trovejante', 1, 'Evocação'),
  sp('spell:misty-step', 'Passo Nebuloso', 2, 'Conjuração'),
  sp('spell:mirror-image', 'Reflexos', 2, 'Ilusão'),
  sp('spell:invisibility', 'Invisibilidade', 2, 'Ilusão'),
  sp('spell:fireball', 'Bola de Fogo', 3, 'Evocação'),
  sp('spell:cure-wounds', 'Curar Ferimentos', 1, 'Evocação', { classKeys: ['class:cleric'] }),
];

export const SKILLS: Skill[] = [
  create(SkillSchema, { key: 'skill:arcana', namePt: 'Arcanismo', ability: Ability.INTELLIGENCE }),
  create(SkillSchema, { key: 'skill:history', namePt: 'História', ability: Ability.INTELLIGENCE }),
  create(SkillSchema, { key: 'skill:stealth', namePt: 'Furtividade', ability: Ability.DEXTERITY }),
  create(SkillSchema, { key: 'skill:perception', namePt: 'Percepção', ability: Ability.WISDOM }),
];

export const WIZARD_KEYS: SheetKeys = {
  cantrips: ['spell:fire-bolt', 'spell:mage-hand', 'spell:light'],
  known: ['spell:magic-missile', 'spell:detect-magic'],
  prepared: ['spell:magic-missile', 'spell:detect-magic'],
  skills: ['skill:arcana', 'skill:history'],
  expertise: [],
};

const ABILITIES = [
  [Ability.STRENGTH, 'Força', 12, 1],
  [Ability.DEXTERITY, 'Destreza', 16, 3],
  [Ability.CONSTITUTION, 'Constituição', 16, 3],
  [Ability.INTELLIGENCE, 'Inteligência', 18, 4],
  [Ability.WISDOM, 'Sabedoria', 13, 1],
  [Ability.CHARISMA, 'Carisma', 12, 1],
] as const;

/** Pensantus as `Character.derived` (before) or `PreviewLevelUp.after` (with `after`): E8-15's numbers. */
export function pensantus(
  after = false,
  over: MessageInitShape<typeof DerivedSheetSchema> = {},
): DerivedSheet {
  const lvl = after ? 4 : 3;
  return create(DerivedSheetSchema, {
    classes: [create(DerivedClassSchema, { classKey: 'class:wizard', namePt: 'Mago', level: lvl })],
    totalLevel: lvl,
    proficiencyBonus: 2,
    abilities: ABILITIES.map(([ability, namePt, score, modifier]) =>
      create(DerivedAbilitySchema, {
        ability,
        namePt,
        score: after && ability === Ability.INTELLIGENCE ? 20 : score,
        modifier: after && ability === Ability.INTELLIGENCE ? 5 : modifier,
      }),
    ),
    savingThrows: [
      create(SavingThrowSchema, {
        ability: Ability.INTELLIGENCE,
        bonus: after ? 7 : 6,
        proficient: true,
      }),
    ],
    skills: [
      create(DerivedSkillSchema, {
        key: 'skill:arcana',
        namePt: 'Arcanismo',
        ability: Ability.INTELLIGENCE,
        proficiency: ProficiencyLevel.PROFICIENT,
        bonus: after ? 7 : 6,
      }),
      create(DerivedSkillSchema, {
        key: 'skill:history',
        namePt: 'História',
        ability: Ability.INTELLIGENCE,
        proficiency: ProficiencyLevel.PROFICIENT,
        bonus: after ? 7 : 6,
      }),
      create(DerivedSkillSchema, {
        key: 'skill:stealth',
        namePt: 'Furtividade',
        ability: Ability.DEXTERITY,
        proficiency: ProficiencyLevel.NONE,
        bonus: 3,
      }),
    ],
    passivePerception: 11,
    passiveInvestigation: after ? 17 : 16,
    passiveInsight: 11,
    initiative: 3,
    armorClass: 13,
    hitPointsMax: after ? 30 : 23,
    hitDice: [create(HitDiceSchema, { faces: 6, count: lvl })],
    spellcasting: [
      create(SpellcastingSchema, {
        classKey: 'class:wizard',
        classNamePt: 'Mago',
        ability: Ability.INTELLIGENCE,
        saveDc: after ? 15 : 14,
        attackBonus: after ? 7 : 6,
        cantripsKnown: after ? 4 : 3,
        preparedMax: after ? 9 : 7,
      }),
    ],
    spellSlots: [
      create(SpellSlotsSchema, { level: 1, count: 4 }),
      create(SpellSlotsSchema, { level: 2, count: after ? 3 : 2 }),
    ],
    ...over,
  });
}

/** The feats a table that allows them offers at the level: one with no increase, one that raises one of two abilities, and one the character does not qualify for. */
export function featOptions(): FeatOption[] {
  return [
    create(FeatOptionSchema, {
      key: 'feat:grappler',
      namePt: 'Agarrador',
      name: 'Grappler',
      desc: ['You have advantage on attack rolls against a creature you are grappling.'],
      prerequisite: { minimums: { strength: 13 } },
      qualifies: true,
    }),
    create(FeatOptionSchema, {
      key: 'feat:atleta@mesa',
      namePt: 'Atleta',
      desc: ['Você corre e escala melhor.'],
      table: true,
      increase: { count: 1, from: [Ability.STRENGTH, Ability.DEXTERITY], value: 1 },
      qualifies: true,
    }),
    create(FeatOptionSchema, {
      key: 'feat:mestre@mesa',
      namePt: 'Mestre das Armas',
      desc: ['Texto.'],
      table: true,
      prerequisite: { minimums: { strength: 15 }, level: 8 },
      qualifies: false,
      unmet: [
        {
          kind: FeatUnmetKind.ABILITY_MINIMUM,
          abilities: [{ ability: Ability.STRENGTH, minimum: 15 }],
        },
        { kind: FeatUnmetKind.LEVEL, value: 8 },
      ],
    }),
  ];
}
