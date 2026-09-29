import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import {
  Alignment as GenAlignment,
  BasicSheet as GenBasicSheet,
  CharacterKind as GenCharacterKind,
  CharacterService,
  CustomBackground,
  FullSheet as GenFullSheet,
  HitPointsMethod as GenHitPointsMethod,
  Item,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import {
  Ability as GenAbility,
  ContentService,
  SpellPreparation as GenSpellPreparation,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { AbilityKey, CharacterKind } from '../../core/characters/characters.types';
import { CONNECT_TRANSPORT } from '../../core/connect/transport';
import {
  AlignmentKey,
  BasicCharacterFormValue,
  CharacterEditorSource,
  CharacterFormValue,
  CreateCharacterInput,
  HitPointsMethod,
  RulesCatalogVm,
  SpellPreparation,
  SubclassOptionVm,
  SubraceOptionVm,
  UpdateCharacterInput,
} from './character-editor.types';

const KIND_FROM_GEN: Record<GenCharacterKind, CharacterKind> = {
  [GenCharacterKind.UNSPECIFIED]: 'player',
  [GenCharacterKind.PLAYER]: 'player',
  [GenCharacterKind.ENEMY]: 'enemy',
  [GenCharacterKind.BOSS]: 'boss',
  [GenCharacterKind.MINION]: 'minion',
  [GenCharacterKind.STORY]: 'story',
};

const KIND_TO_GEN: Record<CharacterKind, GenCharacterKind> = {
  player: GenCharacterKind.PLAYER,
  enemy: GenCharacterKind.ENEMY,
  boss: GenCharacterKind.BOSS,
  minion: GenCharacterKind.MINION,
  story: GenCharacterKind.STORY,
};

const ABILITY_FROM_GEN: Record<GenAbility, AbilityKey> = {
  [GenAbility.UNSPECIFIED]: 'str',
  [GenAbility.STRENGTH]: 'str',
  [GenAbility.DEXTERITY]: 'dex',
  [GenAbility.CONSTITUTION]: 'con',
  [GenAbility.INTELLIGENCE]: 'int',
  [GenAbility.WISDOM]: 'wis',
  [GenAbility.CHARISMA]: 'cha',
};

const PREPARATION_FROM_GEN: Record<GenSpellPreparation, SpellPreparation | null> = {
  [GenSpellPreparation.UNSPECIFIED]: null,
  [GenSpellPreparation.KNOWN]: 'known',
  [GenSpellPreparation.PREPARED]: 'prepared',
  [GenSpellPreparation.SPELLBOOK]: 'spellbook',
};

const ALIGNMENT_TO_GEN: Record<AlignmentKey, GenAlignment> = {
  '': GenAlignment.UNSPECIFIED,
  lawful_good: GenAlignment.LAWFUL_GOOD,
  neutral_good: GenAlignment.NEUTRAL_GOOD,
  chaotic_good: GenAlignment.CHAOTIC_GOOD,
  lawful_neutral: GenAlignment.LAWFUL_NEUTRAL,
  neutral: GenAlignment.NEUTRAL,
  chaotic_neutral: GenAlignment.CHAOTIC_NEUTRAL,
  lawful_evil: GenAlignment.LAWFUL_EVIL,
  neutral_evil: GenAlignment.NEUTRAL_EVIL,
  chaotic_evil: GenAlignment.CHAOTIC_EVIL,
};

const ALIGNMENT_FROM_GEN: Record<GenAlignment, AlignmentKey> = {
  [GenAlignment.UNSPECIFIED]: '',
  [GenAlignment.LAWFUL_GOOD]: 'lawful_good',
  [GenAlignment.NEUTRAL_GOOD]: 'neutral_good',
  [GenAlignment.CHAOTIC_GOOD]: 'chaotic_good',
  [GenAlignment.LAWFUL_NEUTRAL]: 'lawful_neutral',
  [GenAlignment.NEUTRAL]: 'neutral',
  [GenAlignment.CHAOTIC_NEUTRAL]: 'chaotic_neutral',
  [GenAlignment.LAWFUL_EVIL]: 'lawful_evil',
  [GenAlignment.NEUTRAL_EVIL]: 'neutral_evil',
  [GenAlignment.CHAOTIC_EVIL]: 'chaotic_evil',
};

const HIT_POINTS_METHOD_TO_GEN: Record<HitPointsMethod, GenHitPointsMethod> = {
  average: GenHitPointsMethod.AVERAGE,
  rolled: GenHitPointsMethod.ROLLED,
};

const HIT_POINTS_METHOD_FROM_GEN: Record<GenHitPointsMethod, HitPointsMethod> = {
  [GenHitPointsMethod.UNSPECIFIED]: 'average',
  [GenHitPointsMethod.AVERAGE]: 'average',
  [GenHitPointsMethod.ROLLED]: 'rolled',
};

function linesOf(text: string): string[] {
  return text
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

/** The MVP editor keeps equipment as one line of free text per item — this
 * is the one place that round-trips a line through `Item.quantity` (shown
 * as "Nome (xN)"), so a saved sheet's quantities survive re-opening the
 * editor. */
function itemsFromLines(text: string): Item[] {
  return linesOf(text).map((line) => {
    const match = /^(.*)\s\(x(\d+)\)$/.exec(line);
    return match ? { name: match[1], quantity: Number(match[2]) } : { name: line, quantity: 1 };
  }) as Item[];
}

function lineFromItem(item: { name: string; quantity: number }): string {
  return item.quantity > 1 ? `${item.name} (x${item.quantity})` : item.name;
}

/**
 * Builds the wire `FullSheet` fields this form edits, from the form value
 * alone — used as-is for `CreateCharacter` (nothing to preserve yet) and
 * as the "what changed" half of `mergeFullSheetInit` for `UpdateCharacter`.
 *
 * Exported for `character-editor-source.live.spec.ts`'s round-trip test.
 */
export function toFullSheetInit(v: CharacterFormValue) {
  const firstClass = v.className
    ? [
        {
          classKey: v.className,
          level: v.level,
          subclass: v.subclassName
            ? ({ case: 'subclassKey' as const, value: v.subclassName })
            : v.customSubclassName
              ? ({ case: 'customSubclassName' as const, value: v.customSubclassName })
              : ({ case: undefined, value: undefined }),
        },
      ]
    : [];

  const background =
    v.background === 'custom'
      ? {
          case: 'customBackground' as const,
          value: {
            name: v.customBackgroundName,
            skillKeys: v.customBackgroundSkills ?? [],
          } satisfies Partial<CustomBackground>,
        }
      : { case: 'backgroundKey' as const, value: v.background };

  return {
    baseScores: {
      strength: v.abilities.str,
      dexterity: v.abilities.dex,
      constitution: v.abilities.con,
      intelligence: v.abilities.int,
      wisdom: v.abilities.wis,
      charisma: v.abilities.cha,
    },
    raceKey: v.race,
    subraceKey: v.subrace,
    classes: firstClass,
    background,
    skillProficiencyKeys: v.skillProficiencies,
    expertiseSkillKeys: v.expertiseSkillKeys,
    extraAbilityBonuses: {
      strength: v.extraAbilityBonuses.str,
      dexterity: v.extraAbilityBonuses.dex,
      constitution: v.extraAbilityBonuses.con,
      intelligence: v.extraAbilityBonuses.int,
      wisdom: v.extraAbilityBonuses.wis,
      charisma: v.extraAbilityBonuses.cha,
    },
    hitPoints: {
      method: HIT_POINTS_METHOD_TO_GEN[v.hitPointsMethod],
      rolls: v.hitPointsMethod === 'rolled' ? v.hitPointsRolls : [],
    },
    armorKey: v.armor,
    shield: v.shield,
    weaponKeys: v.weapons,
    cantripKeys: v.cantrips,
    knownSpellKeys: v.spellsKnown,
    preparedSpellKeys: v.spellsPrepared,
    equipment: itemsFromLines(v.equipmentText),
    languages: linesOf(v.languagesText),
    toolProficiencies: linesOf(v.toolProficienciesText),
    experiencePoints: v.experiencePoints,
    alignment: ALIGNMENT_TO_GEN[v.alignment],
    customFeaturesText: v.customFeaturesText,
  };
}

/**
 * The no-data-loss fix (integrator review, phase 2b): starts from the
 * `FullSheet` `loadCharacterForEdit` last read for this character and
 * overwrites only the fields `toFullSheetInit` sets from the form — so
 * `coins`, `feature_choice_keys`, and any class beyond the first
 * (multiclassing; this editor only ever edits one) survive a save
 * unchanged instead of silently disappearing. On `CreateCharacter` there
 * is nothing loaded yet, so `original` is `undefined` and this is exactly
 * `toFullSheetInit(v)`.
 *
 * Exported for `character-editor-source.live.spec.ts`'s round-trip test.
 */
export function mergeFullSheetInit(original: GenFullSheet | undefined, v: CharacterFormValue) {
  if (!original) {
    return toFullSheetInit(v);
  }
  // Drop the message's own `$typeName` brand: the result is a plain
  // MessageInitShape for the RPC call, like `toFullSheetInit`'s own
  // return value, not a re-branded `FullSheet` instance.
  const { $typeName: _typeName, ...rest } = original;
  return { ...rest, ...toFullSheetInit(v) };
}

function toBasicSheetInit(v: BasicCharacterFormValue) {
  return {
    hitPointsMax: v.hitPointsMax,
    armorClass: v.armorClass,
    speedFt: v.speedWalkFt,
    attackBonus: v.attackBonus,
    damage: v.damage,
    description: v.description,
  };
}

/** Exported for `character-editor-source.live.spec.ts`'s round-trip test. */
export function toFormFullSheet(name: string, full: GenFullSheet): CharacterFormValue {
  const firstClass = full.classes[0];
  return {
    name,
    race: full.raceKey,
    subrace: full.subraceKey,
    className: firstClass?.classKey ?? '',
    level: firstClass?.level ?? 1,
    subclassName: firstClass?.subclass.case === 'subclassKey' ? firstClass.subclass.value : '',
    customSubclassName:
      firstClass?.subclass.case === 'customSubclassName' ? firstClass.subclass.value : '',
    background:
      full.background.case === 'customBackground' ? 'custom' : (full.background.value ?? ''),
    customBackgroundName:
      full.background.case === 'customBackground' ? full.background.value.name : '',
    customBackgroundSkills:
      full.background.case === 'customBackground' && full.background.value.skillKeys.length === 2
        ? [full.background.value.skillKeys[0], full.background.value.skillKeys[1]]
        : null,
    skillProficiencies: full.skillProficiencyKeys,
    expertiseSkillKeys: full.expertiseSkillKeys,
    abilities: {
      str: full.baseScores?.strength ?? 10,
      dex: full.baseScores?.dexterity ?? 10,
      con: full.baseScores?.constitution ?? 10,
      int: full.baseScores?.intelligence ?? 10,
      wis: full.baseScores?.wisdom ?? 10,
      cha: full.baseScores?.charisma ?? 10,
    },
    extraAbilityBonuses: {
      str: full.extraAbilityBonuses?.strength ?? 0,
      dex: full.extraAbilityBonuses?.dexterity ?? 0,
      con: full.extraAbilityBonuses?.constitution ?? 0,
      int: full.extraAbilityBonuses?.intelligence ?? 0,
      wis: full.extraAbilityBonuses?.wisdom ?? 0,
      cha: full.extraAbilityBonuses?.charisma ?? 0,
    },
    hitPointsMethod: HIT_POINTS_METHOD_FROM_GEN[full.hitPoints?.method ?? GenHitPointsMethod.AVERAGE],
    hitPointsRolls: full.hitPoints?.rolls ?? [],
    // Recomputed by the component from the catalog once loaded — see
    // `CharacterEditor.isCaster`; not read back from the wire.
    isCaster: false,
    cantrips: full.cantripKeys,
    spellsKnown: full.knownSpellKeys,
    spellsPrepared: full.preparedSpellKeys,
    armor: full.armorKey,
    shield: full.shield,
    weapons: full.weaponKeys,
    equipmentText: full.equipment.map(lineFromItem).join('\n'),
    languagesText: full.languages.join('\n'),
    toolProficienciesText: full.toolProficiencies.join('\n'),
    experiencePoints: full.experiencePoints,
    alignment: ALIGNMENT_FROM_GEN[full.alignment],
    customFeaturesText: full.customFeaturesText,
  };
}

function toFormBasicSheet(name: string, basic: GenBasicSheet): BasicCharacterFormValue {
  return {
    name,
    hitPointsMax: basic.hitPointsMax,
    armorClass: basic.armorClass,
    speedWalkFt: basic.speedFt,
    attackBonus: basic.attackBonus,
    damage: basic.damage,
    description: basic.description,
  };
}

/**
 * `CharacterEditorSource` over the generated `ContentService`
 * (`meurpg.rules.v1`, for the catalog) and `CharacterService`
 * (`meurpg.characters.v1`, for create/read/update) clients, phase 2.
 * Provided at the route level for the three editor routes — see
 * `character-editor.routes.ts` — so these clients stay out of the eager
 * bundle.
 */
@Injectable()
export class CharacterEditorSourceLive implements CharacterEditorSource {
  private readonly characterClient = createClient(CharacterService, inject(CONNECT_TRANSPORT));
  private readonly contentClient = createClient(ContentService, inject(CONNECT_TRANSPORT));

  /** The last `FullSheet` `loadCharacterForEdit` read for a character, kept
   * only so `updateCharacter` can start from it — see
   * `mergeFullSheetInit`'s doc comment. Never read for anything else; a
   * fresh `CharacterEditorSourceLive` per route (see
   * `character-editor.routes.ts`) means this never outlives the page. */
  private readonly loadedFullSheets = new Map<string, GenFullSheet>();

  async loadCatalog(campaignId: string): Promise<RulesCatalogVm> {
    const res = await this.contentClient.listContent({ campaignId });
    const content = res.content!;

    const subracesByRace = new Map<string, SubraceOptionVm[]>();
    for (const sr of content.subraces) {
      const list = subracesByRace.get(sr.raceKey) ?? [];
      list.push({ key: sr.key, namePt: sr.namePt });
      subracesByRace.set(sr.raceKey, list);
    }
    const subclassesByClass = new Map<string, SubclassOptionVm[]>();
    for (const sc of content.subclasses) {
      const list = subclassesByClass.get(sc.classKey) ?? [];
      list.push({ key: sc.key, namePt: sc.namePt });
      subclassesByClass.set(sc.classKey, list);
    }

    return {
      races: content.races.map((r) => ({
        key: r.key,
        namePt: r.namePt,
        subraces: subracesByRace.get(r.key) ?? [],
      })),
      classes: content.classes.map((c) => ({
        key: c.key,
        namePt: c.namePt,
        isCaster: !!c.spellcasting,
        preparation: c.spellcasting ? PREPARATION_FROM_GEN[c.spellcasting.preparation] : null,
        subclasses: subclassesByClass.get(c.key) ?? [],
      })),
      backgrounds: content.backgrounds.map((b) => ({ key: b.key, namePt: b.namePt })),
      skills: content.skills.map((s) => ({
        key: s.key,
        namePt: s.namePt,
        ability: ABILITY_FROM_GEN[s.ability],
      })),
    };
  }

  async loadCharacterForEdit(campaignId: string, characterId: string) {
    const res = await this.characterClient.getCharacter({ campaignId, characterId });
    const character = res.character!;
    const kind = KIND_FROM_GEN[character.kind];
    const sheetCase = character.sheet?.content.case;

    let full = null;
    if (sheetCase === 'full') {
      const loaded = character.sheet!.content.value as GenFullSheet;
      this.loadedFullSheets.set(characterId, loaded);
      full = toFormFullSheet(character.name, loaded);
    }

    return {
      kind,
      revision: character.revision,
      full,
      basic:
        sheetCase === 'basic'
          ? toFormBasicSheet(character.name, character.sheet!.content.value as GenBasicSheet)
          : null,
    };
  }

  async createCharacter(input: CreateCharacterInput): Promise<{ characterId: string }> {
    const res = await this.characterClient.createCharacter({
      campaignId: input.campaignId,
      kind: KIND_TO_GEN[input.kind],
      name: input.full?.name ?? input.basic?.name ?? '',
      sheet: input.full
        ? { content: { case: 'full', value: toFullSheetInit(input.full) } }
        : { content: { case: 'basic', value: toBasicSheetInit(input.basic!) } },
    });
    return { characterId: res.character!.id };
  }

  async updateCharacter(input: UpdateCharacterInput): Promise<{ revision: number }> {
    const res = await this.characterClient.updateCharacter({
      campaignId: input.campaignId,
      characterId: input.characterId,
      revision: input.revision,
      name: input.name,
      sheet: input.full
        ? {
            content: {
              case: 'full',
              value: mergeFullSheetInit(this.loadedFullSheets.get(input.characterId), input.full),
            },
          }
        : { content: { case: 'basic', value: toBasicSheetInit(input.basic!) } },
    });
    // Keep the cache current: the next save (without reloading in between)
    // starts from what the server just confirmed, not the stale pre-edit
    // version.
    const savedCase = res.character?.sheet?.content.case;
    if (savedCase === 'full') {
      this.loadedFullSheets.set(input.characterId, res.character!.sheet!.content.value as GenFullSheet);
    }
    return { revision: res.character!.revision };
  }
}
