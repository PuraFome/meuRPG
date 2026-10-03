import { AbilityKey, CharacterBlockedReason, CharacterKind } from '../../core/characters/characters.types';
import { DamageTypeKey } from '../../core/characters/character-labels';

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
  /** An enemy's or boss's challenge rating ("ND"): "0", "1/8", "1/4", "1/2",
   * "1" to "30", or `''` for none. A player's is always `''` and 0. */
  challengeRating: string;
  /** The XP an enemy or boss gives when defeated ("XP ao derrotar"). */
  xpValue: number;
  alignment: AlignmentKey;
  /** Locks with the rest of the sheet, unlike the story fields (A3). */
  customFeaturesText: string;
}

/** One attack of a minion or story NPC (`BasicAttack`). */
export interface BasicAttackFormValue {
  name: string;
  attackBonus: number;
  damageDiceCount: number;
  damageDiceSides: number;
  damageBonus: number;
  damageType: DamageTypeKey;
  /** `BasicAttack.range_ft`, kept as saved: the form has no field for it. */
  rangeFt: number;
}

/** The form value for a minion or story NPC (`BasicSheet`) — the "single
 * short form" the plan asks for. */
export interface BasicCharacterFormValue {
  name: string;
  hitPointsMax: number;
  armorClass: number;
  speedFt: number;
  initiativeBonus: number;
  attacks: BasicAttackFormValue[];
  /** `BasicSheet.damage` and `attack_bonus`, the deprecated fields of a
   * sheet saved before Etapa 6 whose damage could not become an attack. The
   * form shows the text as a note and sends both back unchanged. */
  legacyDamage: string;
  legacyAttackBonus: number;
  description: string;
  /** The challenge rating ("ND") and the XP the minion gives when defeated,
   * as `FullSheet` has them (E7-11). */
  challengeRating: string;
  xpValue: number;
}

/** One row of the SRD's "Experience Points by Challenge Rating" table
 * (`rules.v1.ChallengeRating`): "1/4" gives 50 XP. */
export interface ChallengeRatingVm {
  readonly rating: string;
  readonly xp: number;
}

export interface SubraceOptionVm {
  readonly key: string;
  readonly namePt: string;
  /** The subrace's Constitution increase, added to the race's: the HP
   * preview needs the final score. The other abilities are not read here. */
  readonly constitutionBonus: number;
}

export interface RaceOptionVm {
  readonly key: string;
  readonly namePt: string;
  /** The race's Constitution increase (see `SubraceOptionVm`). */
  readonly constitutionBonus: number;
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
  /** Faces of the hit die: 6, 8, 10 or 12. */
  readonly hitDie: number;
  readonly isCaster: boolean;
  readonly preparation: SpellPreparation | null;
  readonly subclasses: readonly SubclassOptionVm[];
  /** The class level at which the subclass is chosen (`subclass_level`). */
  readonly subclassLevel: number;
  /** The class level at which spellcasting starts (`first_level`), or 0 for
   * a class that never casts. */
  readonly spellcastingFirstLevel: number;
  /** The highest spell circle at each class level, index 0 = level 1 (from
   * the server, `max_spell_level_by_level`). Empty for a non-caster. */
  readonly maxSpellLevelByLevel: readonly number[];
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
  /** 0 is a cantrip, 1-9 a leveled spell — this decides whether a spell
   * belongs in "Truques" or in "Magias conhecidas"/"preparadas". Which
   * circles a character level reaches comes from the server
   * (`ClassOptionVm.maxSpellLevelByLevel`); the browser only filters by it. */
  readonly level: number;
  /** Content keys of the classes whose spell list has this spell. */
  readonly classKeys: readonly string[];
}

export type CastingTimeUnitKey = '' | 'action' | 'bonus_action' | 'reaction' | 'minute' | 'hour';
export type RangeKindKey = '' | 'self' | 'touch' | 'ranged' | 'sight' | 'unlimited' | 'special';
export type DurationKindKey = '' | 'instantaneous' | 'timed' | 'until_dispelled' | 'special';
export type DurationUnitKey = '' | 'round' | 'minute' | 'hour' | 'day';

/**
 * What the "?" next to a spell shows (`rules.v1.SpellDetails`, trimmed to
 * the four fields of the sheet and the SRD text). Values are in the SRD's
 * units and language; `spell-details-format.ts` writes them in Portuguese.
 * Whatever the structure can't carry stays in each `raw`.
 */
export interface SpellDetailsVm {
  readonly key: string;
  readonly namePt: string;
  /** The SRD's own (English) name. */
  readonly nameEn: string;
  /** 0 is a cantrip. */
  readonly level: number;
  readonly schoolNamePt: string;
  readonly ritual: boolean;
  readonly concentration: boolean;
  readonly castingTime: {
    readonly amount: number;
    readonly unit: CastingTimeUnitKey;
    /** For a reaction, when it is cast, in English. */
    readonly trigger: string;
    readonly raw: string;
  };
  readonly range: {
    readonly kind: RangeKindKey;
    readonly distanceFt: number;
    readonly raw: string;
  };
  readonly components: {
    readonly verbal: boolean;
    readonly somatic: boolean;
    readonly material: boolean;
    /** In English. */
    readonly materialText: string;
  };
  readonly duration: {
    readonly kind: DurationKindKey;
    readonly amount: number;
    readonly unit: DurationUnitKey;
    readonly upTo: boolean;
    readonly concentration: boolean;
    readonly raw: string;
  };
  /** The SRD description, in English, one paragraph per entry. */
  readonly description: readonly string[];
  /** "At Higher Levels", in English. Empty when the spell has none. */
  readonly higherLevel: readonly string[];
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
  /** The ND to XP table, in order (0, 1/8, 1/4, 1/2, 1 to 30), for the NPC's
   * "Nível de desafio (ND)" picker. */
  readonly challengeRatings: readonly ChallengeRatingVm[];
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
  /** Why the caller may not save this sheet now (`Character.can_edit` is
   * false), or null when they may. The editor shows the reason instead of
   * a form whose save the server would refuse (RN-01, RN-03). */
  readonly blocked: CharacterBlockedReason | null;
  /** The player's sheet is locked (a session started, RN-01) or the
   * character is dead. The master may still edit it; the XP is then read-only
   * here, because only awards change it (MR-016). */
  readonly sheetLocked: boolean;
}

/**
 * The port `CharacterEditor` depends on. Phase 2 provides a concrete
 * implementation wrapping `rules.v1.ContentService` and
 * `characters.v1.CharacterService` (see this file's top comment).
 */
export abstract class CharacterEditorSource {
  abstract loadCatalog(campaignId: string): Promise<RulesCatalogVm>;
  /** One spell in full, for the "?" next to its name. */
  abstract loadSpellDetails(campaignId: string, spellKey: string): Promise<SpellDetailsVm>;
  abstract loadCharacterForEdit(campaignId: string, characterId: string): Promise<CharacterForEdit>;
  abstract createCharacter(input: CreateCharacterInput): Promise<{ characterId: string }>;
  abstract updateCharacter(input: UpdateCharacterInput): Promise<{ revision: number }>;
}
