import { FormArray, FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import type {
  AttackEntry,
  AttributeKey,
  Character,
  Currency,
  DeathSaves,
  DerivedField,
  DndSheet,
  SkillEntry,
  SpellEntry,
  SpellSlot,
} from '../../core/models/character';
import {
  abilityModifier,
  passivePerception,
  proficiencyBonusForLevel,
  spellAttackBonus,
  spellSaveDc,
} from './dnd-calculations';
import { ATTRIBUTE_KEYS, DND_SKILLS, findClass, raceSpeed, skillAbility } from './dnd-data';

export interface InventoryFormValue {
  name: string;
  quantity: number;
  weight: number;
  description: string;
}

/** Shape read from the form's `skills` FormArray. */
export type SkillFormValue = SkillEntry;

function parseJson<T>(raw: string, fallback: T): T {
  try {
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

/** Parse the persisted `character.skills` strings into skill rows. */
export function parseSkillRows(raw: string[] | undefined): SkillEntry[] {
  if (!raw?.length) return [];
  return raw.map((item) => {
    const parsed = parseJson<Partial<SkillEntry> & { attribute?: string; bonus?: number }>(item, {
      name: item,
    });
    if (parsed.name && typeof parsed.proficient === 'boolean') {
      return {
        name: parsed.name,
        ability: (parsed.ability ?? 'for') as AttributeKey,
        proficient: parsed.proficient,
        expertise: parsed.expertise ?? false,
        bonusOverride: parsed.bonusOverride ?? null,
      };
    }
    // Legacy shape: { name, bonus, attribute } with a hand-typed total bonus.
    const legacyBonus = typeof parsed.bonus === 'number' ? parsed.bonus : 0;
    return {
      name: parsed.name ?? item,
      ability: (parsed.ability ?? parsed.attribute ?? 'for') as AttributeKey,
      proficient: legacyBonus !== 0,
      expertise: false,
      bonusOverride: null,
    };
  });
}

/** Parse the persisted `character.inventory` strings into inventory rows. */
export function parseInventoryRows(raw: string[] | undefined): InventoryFormValue[] {
  if (!raw?.length) return [];
  return raw.map((item) =>
    parseJson<InventoryFormValue>(item, { name: item, quantity: 1, weight: 0, description: '' }),
  );
}

function parseSpells(sheet: DndSheet | undefined): SpellEntry[] {
  return sheet?.spells ?? [];
}

function parseSlots(sheet: DndSheet | undefined): SpellSlot[] {
  return sheet?.spellSlots ?? [];
}

function parseAttacks(sheet: DndSheet | undefined): AttackEntry[] {
  return sheet?.attacks ?? [];
}

/**
 * Build one skill row per canonical 5e skill, matching any persisted entry by
 * name, then append persisted homebrew skills. This keeps the grid complete and
 * intuitive while preserving custom content.
 */
function buildSkillRows(existing: SkillEntry[]): SkillEntry[] {
  const byName = new Map(existing.map((s) => [s.name.toLowerCase(), s]));
  const rows: SkillEntry[] = DND_SKILLS.map((def) => {
    const saved = byName.get(def.name.toLowerCase());
    if (saved) {
      byName.delete(def.name.toLowerCase());
      return saved;
    }
    return { name: def.name, ability: def.ability, proficient: false, expertise: false, bonusOverride: null };
  });
  return [...rows, ...byName.values()];
}

function skillsToFormArray(fb: FormBuilder, rows: SkillEntry[]): FormArray {
  return fb.array(
    rows.map((row) =>
      fb.group({
        name: [row.name, Validators.required],
        ability: [row.ability],
        proficient: [row.proficient],
        expertise: [row.expertise],
        bonusOverride: [row.bonusOverride ?? null],
      }),
    ),
  );
}

function inventoryToFormArray(fb: FormBuilder, rows: InventoryFormValue[]): FormArray {
  return fb.array(
    rows.map((row) =>
      fb.group({
        name: [row.name, Validators.required],
        quantity: [row.quantity ?? 1],
        weight: [row.weight ?? 0],
        description: [row.description ?? ''],
      }),
    ),
  );
}

function stringArrayToFormArray(fb: FormBuilder, values: string[] | undefined): FormArray {
  return fb.array((values ?? []).map((value) => fb.control(value)));
}

function attacksToFormArray(fb: FormBuilder, rows: AttackEntry[]): FormArray {
  return fb.array(
    rows.map((row) =>
      fb.group({
        name: [row.name],
        attackBonus: [row.attackBonus],
        damage: [row.damage],
        damageType: [row.damageType],
      }),
    ),
  );
}

function spellsToFormArray(fb: FormBuilder, rows: SpellEntry[]): FormArray {
  return fb.array(
    rows.map((row) =>
      fb.group({
        name: [row.name],
        level: [row.level],
        prepared: [row.prepared],
      }),
    ),
  );
}

function slotsToFormArray(fb: FormBuilder, rows: SpellSlot[]): FormArray {
  return fb.array(
    rows.map((row) =>
      fb.group({
        level: [row.level],
        total: [row.total],
        used: [row.used],
      }),
    ),
  );
}

/**
 * Build the complete character form. Identity, attributes, combat, skills,
 * inventory and typing are always present regardless of character type; the
 * shared fields component decides what to render based on the `type` control.
 */
export function createCharacterForm(fb: FormBuilder, character?: Character | null): FormGroup {
  const sheet = character?.sheet;
  const level = sheet?.level ?? 1;

  const form = fb.group({
    type: [character?.type ?? 'player'],
    name: [character?.name ?? '', Validators.required],
    description: [character?.description ?? ''],
    identity: fb.group({
      playerName: [sheet?.playerName ?? ''],
      race: [sheet?.race ?? ''],
      class: [sheet?.class ?? ''],
      level: [level, [Validators.required, Validators.min(1), Validators.max(20)]],
      background: [sheet?.background ?? ''],
      alignment: [sheet?.alignment ?? ''],
      xp: [sheet?.xp ?? 0, [Validators.min(0)]],
      inspiration: [sheet?.inspiration ?? 0, [Validators.min(0)]],
    }),
    attributes: fb.group({
      for: [character?.attributes?.['for'] ?? 10, [Validators.min(1), Validators.max(30)]],
      des: [character?.attributes?.['des'] ?? 10, [Validators.min(1), Validators.max(30)]],
      con: [character?.attributes?.['con'] ?? 10, [Validators.min(1), Validators.max(30)]],
      int: [character?.attributes?.['int'] ?? 10, [Validators.min(1), Validators.max(30)]],
      sab: [character?.attributes?.['sab'] ?? 10, [Validators.min(1), Validators.max(30)]],
      car: [character?.attributes?.['car'] ?? 10, [Validators.min(1), Validators.max(30)]],
    }),
    savingThrows: fb.group({
      for: [sheet?.savingThrows?.['for'] ?? false],
      des: [sheet?.savingThrows?.['des'] ?? false],
      con: [sheet?.savingThrows?.['con'] ?? false],
      int: [sheet?.savingThrows?.['int'] ?? false],
      sab: [sheet?.savingThrows?.['sab'] ?? false],
      car: [sheet?.savingThrows?.['car'] ?? false],
    }),
    combat: fb.group({
      hpMax: [sheet?.hpMax ?? 10, [Validators.required, Validators.min(0)]],
      hpCurrent: [sheet?.hpCurrent ?? 10, [Validators.min(0)]],
      hpTemp: [sheet?.hpTemp ?? 0, [Validators.min(0)]],
      armorClass: [sheet?.armorClass ?? 10, [Validators.min(0)]],
      initiative: [sheet?.initiative ?? 0],
      speed: [sheet?.speed ?? raceSpeed(sheet?.race ?? '')],
      hitDice: [sheet?.hitDice ?? '1d8'],
      hitDiceTotal: [sheet?.hitDiceTotal ?? level, [Validators.min(0)]],
      hitDiceSpent: [sheet?.hitDiceSpent ?? 0, [Validators.min(0)]],
      proficiencyBonus: [sheet?.proficiencyBonus ?? 2, [Validators.min(1)]],
      deathSaveSuccesses: [sheet?.deathSaves?.successes ?? 0, [Validators.min(0), Validators.max(3)]],
      deathSaveFailures: [sheet?.deathSaves?.failures ?? 0, [Validators.min(0), Validators.max(3)]],
      passivePerception: [sheet?.passivePerception ?? 10, [Validators.min(0)]],
      conditions: [sheet?.conditions ?? []],
    }),
    spellcasting: fb.group({
      ability: [sheet?.spellcastingAbility ?? ''],
      saveDc: [sheet?.spellSaveDc ?? 8, [Validators.min(0)]],
      attackBonus: [sheet?.spellAttackBonus ?? 0],
      active: [usesSpellcasting(sheet)],
      spells: spellsToFormArray(fb, parseSpells(sheet)),
      slots: slotsToFormArray(fb, parseSlots(sheet)),
    }),
    attacks: attacksToFormArray(fb, parseAttacks(sheet)),
    skills: skillsToFormArray(fb, buildSkillRows(parseSkillRows(character?.skills))),
    inventory: inventoryToFormArray(fb, parseInventoryRows(character?.inventory)),
    currency: fb.group({
      pc: [sheet?.currency?.pc ?? 0, [Validators.min(0)]],
      pp: [sheet?.currency?.pp ?? 0, [Validators.min(0)]],
      pe: [sheet?.currency?.pe ?? 0, [Validators.min(0)]],
      po: [sheet?.currency?.po ?? 0, [Validators.min(0)]],
      pl: [sheet?.currency?.pl ?? 0, [Validators.min(0)]],
    }),
    proficiencies: stringArrayToFormArray(fb, sheet?.proficiencies),
    tools: stringArrayToFormArray(fb, sheet?.tools),
    languages: stringArrayToFormArray(fb, sheet?.languages),
    features: stringArrayToFormArray(fb, sheet?.features),
    personality: fb.group({
      traits: [sheet?.personality?.traits ?? ''],
      ideals: [sheet?.personality?.ideals ?? ''],
      bonds: [sheet?.personality?.bonds ?? ''],
      flaws: [sheet?.personality?.flaws ?? ''],
    }),
    appearance: fb.group({
      age: [sheet?.appearance?.age ?? ''],
      height: [sheet?.appearance?.height ?? ''],
      weight: [sheet?.appearance?.weight ?? ''],
      eyes: [sheet?.appearance?.eyes ?? ''],
      skin: [sheet?.appearance?.skin ?? ''],
      hair: [sheet?.appearance?.hair ?? ''],
    }),
    allies: [sheet?.allies ?? ''],
    minion: fb.group({
      hp: [character?.minion?.hp ?? 10, [Validators.min(1)]],
      attack: [character?.minion?.attack ?? 3, [Validators.min(1)]],
    }),
    overrides: fb.group({
      proficiencyBonus: [Boolean(sheet?.overrides?.proficiencyBonus)],
      initiative: [Boolean(sheet?.overrides?.initiative)],
      passivePerception: [Boolean(sheet?.overrides?.passivePerception)],
      spellSaveDc: [Boolean(sheet?.overrides?.spellSaveDc)],
      spellAttackBonus: [Boolean(sheet?.overrides?.spellAttackBonus)],
    }),
  });

  if (sheet) preserveStoredDerivedValuesAsOverrides(form, sheet);
  return form;
}

function preserveStoredDerivedValuesAsOverrides(form: FormGroup, sheet: DndSheet): void {
  const attributes = form.get('attributes')?.value as Record<string, number>;
  const level = Number(form.get('identity')?.get('level')?.value) || 1;
  const overrides = form.get('overrides') as FormGroup;
  const flag = (field: DerivedField, stored: number | undefined, computed: number): void => {
    if (stored !== undefined && stored !== computed) {
      overrides.get(field)?.setValue(true, { emitEvent: false });
    }
  };

  const prof = proficiencyBonusForLevel(level);
  flag('proficiencyBonus', sheet.proficiencyBonus, prof);
  flag('initiative', sheet.initiative, abilityModifier(attributes?.['des'] ?? 10));

  const skills = ((form.get('skills') as FormArray).controls ?? []).map(
    (c) => c.value as SkillEntry,
  );
  const perception = skills.find((s) => s.name.toLowerCase() === 'percepção') ?? null;
  flag(
    'passivePerception',
    sheet.passivePerception,
    passivePerception(attributes, prof, perception),
  );

  const ability = sheet.spellcastingAbility;
  if (ability) {
    const mod = abilityModifier(attributes?.[ability] ?? 10);
    flag('spellSaveDc', sheet.spellSaveDc, spellSaveDc(mod, prof));
    flag('spellAttackBonus', sheet.spellAttackBonus, spellAttackBonus(mod, prof));
  }
}

function usesSpellcasting(sheet: DndSheet | undefined): boolean {
  if (!sheet) return false;
  return Boolean(
    sheet.spellcastingAbility ||
      sheet.spells?.length ||
      sheet.spellSlots?.length ||
      false,
  );
}

function nonNegative(value: unknown): number {
  const n = Number(value);
  return Number.isFinite(n) && n >= 0 ? n : 0;
}

function stringArray(value: unknown): string[] {
  return ((value as string[]) ?? []).filter((v) => typeof v === 'string' && v.trim() !== '');
}

/** Read the sheet portion of the form into a `DndSheet`. */
export function buildSheetFromForm(form: FormGroup, type: Character['type']): DndSheet | undefined {
  if (type !== 'player' && type !== 'boss') return undefined;
  const v = form.value as Record<string, unknown>;
  const identity = v['identity'] as Record<string, unknown>;
  const combat = v['combat'] as Record<string, unknown>;
  const spellcasting = v['spellcasting'] as Record<string, unknown>;
  const currency = v['currency'] as Record<string, unknown>;
  const personality = v['personality'] as Record<string, unknown>;
  const appearance = v['appearance'] as Record<string, unknown>;

  const savingThrows = v['savingThrows'] as Partial<Record<AttributeKey, boolean>>;
  const overrides = v['overrides'] as Partial<Record<string, boolean>>;

  const spells = ((spellcasting['spells'] as SpellEntry[]) ?? []).filter((s) => s.name?.trim());
  const spellSlots = ((spellcasting['slots'] as SpellSlot[]) ?? []).filter(
    (s) => nonNegative(s.total) > 0,
  );
  const attacks = ((v['attacks'] as AttackEntry[]) ?? []).filter((a) => a.name?.trim());

  const sheet: DndSheet = {
    race: (identity['race'] as string) ?? '',
    class: (identity['class'] as string) ?? '',
    level: (identity['level'] as number) ?? 1,
    background: (identity['background'] as string) ?? '',
    alignment: (identity['alignment'] as string) ?? '',
    xp: (identity['xp'] as number) ?? 0,
    playerName: (identity['playerName'] as string) ?? '',
    inspiration: nonNegative(identity['inspiration']),
    hpMax: nonNegative(combat['hpMax']),
    hpCurrent: nonNegative(combat['hpCurrent']),
    hpTemp: nonNegative(combat['hpTemp']),
    armorClass: nonNegative(combat['armorClass']),
    initiative: Math.trunc(Number(combat['initiative']) || 0),
    speed: nonNegative(combat['speed']),
    hitDice: (combat['hitDice'] as string) ?? '1d8',
    hitDiceTotal: nonNegative(combat['hitDiceTotal']),
    hitDiceSpent: nonNegative(combat['hitDiceSpent']),
    proficiencyBonus: Math.max(1, Math.trunc(Number(combat['proficiencyBonus']) || 2)),
    deathSaves: {
      successes: nonNegative(combat['deathSaveSuccesses']),
      failures: nonNegative(combat['deathSaveFailures']),
    } satisfies DeathSaves,
    passivePerception: nonNegative(combat['passivePerception']),
    conditions: (combat['conditions'] as string[]) ?? [],
    proficiencies: stringArray(v['proficiencies']),
    languages: stringArray(v['languages']),
    savingThrows,
    tools: stringArray(v['tools']),
    attacks,
    spellcastingAbility: (spellcasting['ability'] as AttributeKey | '') ?? '',
    spellSaveDc: nonNegative(spellcasting['saveDc']),
    spellAttackBonus: Math.trunc(Number(spellcasting['attackBonus']) || 0),
    spells,
    spellSlots,
    currency: {
      pc: nonNegative(currency['pc']),
      pp: nonNegative(currency['pp']),
      pe: nonNegative(currency['pe']),
      po: nonNegative(currency['po']),
      pl: nonNegative(currency['pl']),
    } satisfies Currency,
    personality: {
      traits: (personality['traits'] as string) ?? '',
      ideals: (personality['ideals'] as string) ?? '',
      bonds: (personality['bonds'] as string) ?? '',
      flaws: (personality['flaws'] as string) ?? '',
    },
    appearance: {
      age: (appearance['age'] as string) ?? '',
      height: (appearance['height'] as string) ?? '',
      weight: (appearance['weight'] as string) ?? '',
      eyes: (appearance['eyes'] as string) ?? '',
      skin: (appearance['skin'] as string) ?? '',
      hair: (appearance['hair'] as string) ?? '',
    },
    allies: (v['allies'] as string) ?? '',
    features: stringArray(v['features']),
    overrides: {
      proficiencyBonus: Boolean(overrides['proficiencyBonus']),
      initiative: Boolean(overrides['initiative']),
      passivePerception: Boolean(overrides['passivePerception']),
      spellSaveDc: Boolean(overrides['spellSaveDc']),
      spellAttackBonus: Boolean(overrides['spellAttackBonus']),
    },
  };
  return sheet;
}

/** Everyone counts as proficient when the total is a manual override. */
export function buildSkillsFromForm(form: FormGroup): string[] {
  const rows = (form.value as { skills: SkillEntry[] }).skills ?? [];
  return rows
    .filter((row) => row.name?.trim())
    .map((row) =>
      JSON.stringify({
        name: row.name.trim(),
        ability: row.ability,
        proficient: Boolean(row.proficient),
        expertise: Boolean(row.expertise),
        bonusOverride:
          row.bonusOverride === null || row.bonusOverride === undefined
            ? null
            : Math.trunc(Number(row.bonusOverride)),
      } satisfies SkillEntry),
    );
}

export function buildInventoryFromForm(form: FormGroup): string[] {
  const rows = (form.value as { inventory: InventoryFormValue[] }).inventory ?? [];
  return rows
    .filter((row) => row.name?.trim())
    .map((row) =>
      JSON.stringify({
        name: row.name.trim(),
        quantity: Math.trunc(Number(row.quantity) || 1),
        weight: Number(row.weight) || 0,
        description: row.description ?? '',
      } satisfies InventoryFormValue),
    );
}

export function buildMinionFromForm(form: FormGroup): { hp: number; attack: number } {
  const minion = (form.value as { minion: { hp: number; attack: number } }).minion;
  return {
    hp: Math.max(1, Math.trunc(Number(minion?.hp) || 1)),
    attack: Math.max(1, Math.trunc(Number(minion?.attack) || 1)),
  };
}

export function buildAttributesFromForm(form: FormGroup): Record<string, number> {
  const attributes = (form.value as { attributes: Record<string, number> }).attributes ?? {};
  const result: Record<string, number> = {};
  for (const key of ATTRIBUTE_KEYS) {
    result[key] = Number(attributes[key]) || 0;
  }
  return result;
}

/** Helper for preset logic: the array of attribute keys, in canonical order. */
export const ABILITY_KEY_ORDER: readonly AttributeKey[] = ATTRIBUTE_KEYS;

export { findClass, skillAbility };