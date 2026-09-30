import { AbilityKey, CharacterKind } from '../../core/characters/characters.types';

/**
 * The view-model and port `CharacterEditor` needs. Phase 2 maps
 * `rules.v1.ContentService.ListContent`'s response onto `RulesCatalogVm`,
 * and `characters.v1.CharacterService`'s `Create/UpdateCharacter` onto
 * `CharacterEditorSource` — nothing below imports from `../../../gen/...`,
 * so that mapping is the only thing phase 2 adds.
 */

/** How the editor was opened. `'create'` covers both a player creating
 * their own character and a master creating an NPC — `kind` (below) tells
 * them apart; `'edit'` loads the existing sheet first. */
export type CharacterEditorMode = 'create' | 'edit';

export interface AbilityScoresInput {
  str: number;
  dex: number;
  con: number;
  int: number;
  wis: number;
  cha: number;
}

/** `Alignment` (characters.proto), as a string key instead of the wire
 * enum so this file stays gen-free. `''` is `ALIGNMENT_UNSPECIFIED` — not
 * chosen. Labels are the proto's own comments, word for word. */
export type AlignmentKey =
  | ''
  | 'lawful_good'
  | 'neutral_good'
  | 'chaotic_good'
  | 'lawful_neutral'
  | 'neutral'
  | 'chaotic_neutral'
  | 'lawful_evil'
  | 'neutral_evil'
  | 'chaotic_evil';

export const ALIGNMENT_LABELS: Record<AlignmentKey, string> = {
  '': 'Não escolhido',
  lawful_good: 'Leal e bom',
  neutral_good: 'Neutro e bom',
  chaotic_good: 'Caótico e bom',
  lawful_neutral: 'Leal e neutro',
  neutral: 'Neutro',
  chaotic_neutral: 'Caótico e neutro',
  lawful_evil: 'Leal e mau',
  neutral_evil: 'Neutro e mau',
  chaotic_evil: 'Caótico e mau',
};

/** How a character's maximum hit points grow after the first level, which
 * always takes the hit die's maximum (`HitPointsMethod`, characters.proto). */
export type HitPointsMethod = 'average' | 'rolled';

/**
 * The form value for a player, enemy or boss (`FullSheet`). Every field
 * that is a content key (`race`, `subrace`, `className`, `subclassName`,
 * `background`, skill/expertise keys, `armor`, `weapons`, `cantrips`,
 * `spellsKnown`, `spellsPrepared`) is picked from the loaded
 * `RulesCatalogVm`, never typed — a typed key almost never matches a real
 * one, and `CreateCharacter`/`UpdateCharacter` reject it with
 * `invalid_argument` (integrator fix: the editor must never make a person
 * type a content key). Only genuinely free text stays free text: other
 * equipment items, languages, tool proficiencies, and custom features —
 * `FullSheet.equipment`/`languages`/`tool_proficiencies` are themselves
 * free text or simple repeated strings on the wire, so a line-per-item
 * textarea maps onto them directly. Equipment quantity round-trips through
 * a "(xN)" suffix on that same line (`CharacterEditorSourceLive`'s
 * `itemsFromLines`/`lineFromItem`) rather than a separate structured
 * field — simple enough to be lossless without a dedicated add/remove list
 * UI (integrator review, phase 2b).
 *
 * **Left out on purpose (integrator review, phase 2 and 2b):**
 * `FullSheet.feature_choice_keys` (a fighting style, a dragon ancestry, an
 * eldritch invocation...) has no field here. `rules.v1.Content` does not
 * list, for a given feature or trait, which content keys are valid choices
 * for it — there is nowhere in the catalog to build a "choose one" select
 * from. Sending it empty on create is safe: an unmade choice a feature
 * needs shows up as a `DerivedSheet.issue`, exactly like any other
 * incomplete choice the rules would flag (ADR-0008, "the app is an
 * assistant, not a judge"). On an edit, `CharacterEditorSourceLive` starts
 * from the loaded `FullSheet` and only overwrites what this form actually
 * has a field for, so a value already set here (by a future level-up flow,
 * say) survives a save through this editor even though this editor cannot
 * set it. Revisit once `ContentService` exposes those option lists.
 */
export interface CharacterFormValue {
  name: string;
  race: string;
  subrace: string;
  className: string;
  subclassName: string;
  customSubclassName: string;
  level: number;
  /** `'custom'` selects the "Outro (personalizado)" option — see
   * `customBackgroundName` and `customBackgroundSkills`. */
  background: string;
  customBackgroundName: string;
  customBackgroundSkills: [string, string] | null;
  skillProficiencies: string[];
  /** A subset of `skillProficiencies`: expertise doubles the proficiency
   * bonus (Bard, Rogue). */
  expertiseSkillKeys: string[];
  abilities: AbilityScoresInput;
  /** Manual bonuses on top of race/subrace (an ability score improvement,
   * a race's chosen +1s, a magic item): each -10 to +10. */
  extraAbilityBonuses: AbilityScoresInput;
  hitPointsMethod: HitPointsMethod;
  /** Only meaningful with `hitPointsMethod === 'rolled'`: one roll per
   * level after the first (`level - 1` entries, since the MVP editor's one
   * class means "every level of the first class" is every level). */
  hitPointsRolls: number[];
  isCaster: boolean;
  /** Content keys, picked from `RulesCatalogVm.spells` filtered to
   * `level === 0` and the chosen class's list (`ClassOptionVm.key` in
   * `Spell.classKeys`). */
  cantrips: string[];
  /** Content keys, `RulesCatalogVm.spells` filtered to `level >= 1` and the
   * chosen class's list. Shown when the class's preparation is "known" or
   * "spellbook". */
  spellsKnown: string[];
  /** Same filter as `spellsKnown`. Shown when the class's preparation is
   * "prepared" or "spellbook". */
  spellsPrepared: string[];
  /** A content key from `RulesCatalogVm.armor`, or `''` for "Sem armadura". */
  armor: string;
  shield: boolean;
  /** Content keys from `RulesCatalogVm.weapons`. */
  weapons: string[];
  equipmentText: string;
  languagesText: string;
  toolProficienciesText: string;
  experiencePoints: number;
  alignment: AlignmentKey;
  /** Locks with the rest of the sheet, unlike the story fields (A3). */
  customFeaturesText: string;
}

/** The form value for a minion or story NPC (`BasicSheet`) — the "single
 * short form" the plan asks for. */
export interface BasicCharacterFormValue {
  name: string;
  hitPointsMax: number;
  armorClass: number;
  speedWalkFt: number;
  attackBonus: number;
  damage: string;
  description: string;
}

export interface SubraceOptionVm {
  readonly key: string;
  readonly namePt: string;
}

export interface RaceOptionVm {
  readonly key: string;
  readonly namePt: string;
  readonly subraces: readonly SubraceOptionVm[];
}

export interface SubclassOptionVm {
  readonly key: string;
  readonly namePt: string;
}

/** How a caster class handles its spell list (integrator amendment,
 * 29/09/2026, from WP-A's `rules.v1` contract): `'known'` (e.g. Sorcerer,
 * Bard — a fixed list of spells known, all always available); `'prepared'`
 * (e.g. Cleric, Druid — chooses which spells to prepare each day from the
 * whole class list, no separate "known" list); `'spellbook'` (Wizard —
 * both a spellbook of known spells and a smaller prepared subset). Only
 * meaningful when `isCaster` is true. */
export type SpellPreparation = 'known' | 'prepared' | 'spellbook';

export interface ClassOptionVm {
  readonly key: string;
  readonly namePt: string;
  readonly isCaster: boolean;
  readonly preparation: SpellPreparation | null;
  readonly subclasses: readonly SubclassOptionVm[];
}

export interface BackgroundOptionVm {
  readonly key: string;
  readonly namePt: string;
}

export interface SkillOptionVm {
  readonly key: string;
  readonly namePt: string;
  readonly ability: AbilityKey;
}

export interface ArmorOptionVm {
  readonly key: string;
  readonly namePt: string;
}

export interface WeaponOptionVm {
  readonly key: string;
  readonly namePt: string;
}

export interface SpellOptionVm {
  readonly key: string;
  readonly namePt: string;
  /** 0 is a cantrip, 1-9 a leveled spell — this alone decides whether a
   * spell belongs in "Truques" or in "Magias conhecidas"/"preparadas"
   * (`rules.proto`'s own distinction; nothing here is about which
   * character level can reach it — the browser never computes that rule,
   * see `CharacterEditorSourceLive`'s `spellsForClass`). */
  readonly level: number;
  /** Content keys of the classes whose spell list has this spell. */
  readonly classKeys: readonly string[];
}

/** `rules.v1.Content`, trimmed to what the editor's dropdowns need
 * (plan §4's `ContentService.ListContent`). */
export interface RulesCatalogVm {
  readonly races: readonly RaceOptionVm[];
  readonly classes: readonly ClassOptionVm[];
  /** SRD 5.1 has a single background, Acólito (ADR-0008) — the editor
   * always adds a fixed "Outro (personalizado)" option after these. */
  readonly backgrounds: readonly BackgroundOptionVm[];
  readonly skills: readonly SkillOptionVm[];
  /** Body armor only — a shield is the separate `shield` checkbox. */
  readonly armor: readonly ArmorOptionVm[];
  readonly weapons: readonly WeaponOptionVm[];
  readonly spells: readonly SpellOptionVm[];
}

export interface CreateCharacterInput {
  readonly campaignId: string;
  readonly kind: CharacterKind;
  readonly full: CharacterFormValue | null;
  readonly basic: BasicCharacterFormValue | null;
}

export interface UpdateCharacterInput {
  readonly campaignId: string;
  readonly characterId: string;
  readonly revision: number;
  readonly name: string;
  readonly full: CharacterFormValue | null;
  readonly basic: BasicCharacterFormValue | null;
}

export interface CharacterForEdit {
  readonly kind: CharacterKind;
  readonly revision: number;
  readonly full: CharacterFormValue | null;
  readonly basic: BasicCharacterFormValue | null;
}

/**
 * The port `CharacterEditor` depends on. Phase 2 provides a concrete
 * implementation wrapping `rules.v1.ContentService` and
 * `characters.v1.CharacterService` (see this file's top comment).
 */
export abstract class CharacterEditorSource {
  abstract loadCatalog(campaignId: string): Promise<RulesCatalogVm>;
  abstract loadCharacterForEdit(campaignId: string, characterId: string): Promise<CharacterForEdit>;
  abstract createCharacter(input: CreateCharacterInput): Promise<{ characterId: string }>;
  abstract updateCharacter(input: UpdateCharacterInput): Promise<{ revision: number }>;
}
