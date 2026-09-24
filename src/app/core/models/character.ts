/** The six ability score keys used across the sheet (Portuguese abbreviations). */
export type AttributeKey = 'for' | 'des' | 'con' | 'int' | 'sab' | 'car';

/**
 * A single skill row. The total bonus is derived from the governing ability,
 * the character's proficiency bonus and the expertise flag; `bonusOverride`
 * lets the player replace that computed value (house rules, magic items).
 */
export interface SkillEntry {
  name: string;
  ability: AttributeKey;
  proficient: boolean;
  expertise: boolean;
  /** Manual total override; `null`/`undefined` means "use the computed bonus". */
  bonusOverride?: number | null;
}

export interface AttackEntry {
  name: string;
  attackBonus: string;
  damage: string;
  damageType: string;
}

export interface SpellEntry {
  name: string;
  /** 0 = cantrip, 1-9 = spell level. */
  level: number;
  prepared: boolean;
}

export interface SpellSlot {
  level: number;
  total: number;
  used: number;
}

/** Coin purse, using the standard 5e denominations. */
export interface Currency {
  /** Cobre */
  pc: number;
  /** Prata */
  pp: number;
  /** Electrum */
  pe: number;
  /** Ouro */
  po: number;
  /** Platina */
  pl: number;
}

export interface Personality {
  traits: string;
  ideals: string;
  bonds: string;
  flaws: string;
}

export interface Appearance {
  age: string;
  height: string;
  weight: string;
  eyes: string;
  skin: string;
  hair: string;
}

export interface DeathSaves {
  successes: number;
  failures: number;
}

/** Fields whose value can be either auto-computed or manually overridden. */
export type DerivedField =
  | 'proficiencyBonus'
  | 'initiative'
  | 'passivePerception'
  | 'spellSaveDc'
  | 'spellAttackBonus';

/**
 * Full D&D 5e character sheet data.
 *
 * Every field added after the original release is optional so characters saved
 * before the expansion keep loading without migration.
 */
export interface DndSheet {
  // ── Identidade ────────────────────────────────────────
  race: string;
  class: string;
  level: number;
  background: string;
  alignment: string;
  xp: number;
  /** Name of the person playing the character (optional). */
  playerName?: string;
  /** Heroic inspiration points (optional). */
  inspiration?: number;

  // ── Combate ───────────────────────────────────────────
  hpMax: number;
  hpCurrent: number;
  hpTemp: number;
  armorClass: number;
  initiative: number;
  speed: number;
  hitDice: string;
  /** Total hit dice available (usually equals level). */
  hitDiceTotal?: number;
  /** Hit dice already spent during rests. */
  hitDiceSpent?: number;
  proficiencyBonus: number;
  deathSaves?: DeathSaves;
  passivePerception?: number;
  conditions?: string[];

  // ── Proficiências e idiomas ───────────────────────────
  proficiencies: string[];
  languages: string[];
  /** Ability -> whether the character is proficient in that saving throw. */
  savingThrows?: Partial<Record<AttributeKey, boolean>>;
  tools?: string[];

  // ── Ataques e conjuração ──────────────────────────────
  attacks?: AttackEntry[];
  spellcastingAbility?: AttributeKey | '';
  spellSaveDc?: number;
  spellAttackBonus?: number;
  spells?: SpellEntry[];
  spellSlots?: SpellSlot[];

  // ── Equipamento ───────────────────────────────────────
  currency?: Currency;

  // ── Interpretação ─────────────────────────────────────
  personality?: Personality;
  appearance?: Appearance;
  allies?: string;

  // ── Traços e características ──────────────────────────
  features: string[];

  /** Marks which derived fields the user chose to edit manually. */
  overrides?: Partial<Record<DerivedField, boolean>>;
}

export interface Character {
  id: string;
  name: string;
  description: string;
  type: 'npc' | 'player' | 'boss' | 'minion';
  imageUrl?: string;
  history?: string;
  masterNotes?: string;
  attributes: Record<string, number>;
  skills: string[];
  inventory: string[];
  quotes: string[];
  /** Full D&D sheet for player/boss types; absent for NPC. */
  sheet?: DndSheet;
  /** Simplified combat stats for minions; absent for other types. */
  minion?: { hp: number; attack: number };
  createdAt: Date;
  updatedAt: Date;
}