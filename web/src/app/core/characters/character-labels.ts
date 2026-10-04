import { AbilityKey, CharacterKind, CharacterState } from './characters.types';

/** Fixed pt-BR labels for the six abilities, in official-sheet order
 * (Força through Carisma) — the same order `character-editor` and
 * `character-sheet` render them in, and the exact field labels
 * `character-editor.spec.ts` and the e2e tests select on (plan §5, "Labels
 * to agree on now"). */
export const ABILITY_LABELS: Record<AbilityKey, string> = {
  str: 'Força',
  dex: 'Destreza',
  con: 'Constituição',
  int: 'Inteligência',
  wis: 'Sabedoria',
  cha: 'Carisma',
};

export function abilityLabel(key: AbilityKey): string {
  return ABILITY_LABELS[key];
}

/** The NPC kinds' pt-BR chip label, matching `characters.proto`'s own
 * comments word for word ("inimigo", "boss", "minion", "NPC de história")
 * and the docs (integrator fix, phase 2). Player characters use
 * `characterKindLabel('player')` too — for example on the master's
 * "Personagens dos jogadores" list, which shows no chip for it, only NPCs
 * do (see `campaign-characters`). */
export function characterKindLabel(kind: CharacterKind): string {
  switch (kind) {
    case 'player':
      return 'Personagem de jogador';
    case 'enemy':
      return 'Inimigo';
    case 'boss':
      return 'Boss';
    case 'minion':
      return 'Minion';
    case 'story':
      return 'NPC de história';
  }
}

/** `CharacterState`'s pt-BR name, exactly as `docs/produto/regras.md`'s
 * lifecycle table names each state (plan §2). */
export function characterStateLabel(state: CharacterState): string {
  switch (state) {
    case 'draft':
      return 'Rascunho';
    case 'locked':
      return 'Travada';
    case 'dead':
      return 'Morto';
    case 'pending':
      return 'Pendente de aprovação';
  }
}

/**
 * A signed modifier, the way every ability, save and skill bonus is shown on
 * the sheet: always with a sign, even for zero. The sheet never recomputes
 * this from the score — it only formats whatever number the server sent, so
 * an inconsistent pair (e.g. score 18 with modifier +9) still renders
 * "+9" verbatim (`character-sheet.spec.ts`).
 */
export function formatModifier(modifier: number): string {
  return modifier >= 0 ? `+${modifier}` : `${modifier}`;
}

/** An SRD damage type, by the lower-case name of `DamageType`'s value
 * (`'fire'` for `DamageType.FIRE`). `''` is "not chosen yet" in a form. */
export type DamageTypeKey =
  | ''
  | 'acid'
  | 'bludgeoning'
  | 'cold'
  | 'fire'
  | 'force'
  | 'lightning'
  | 'necrotic'
  | 'piercing'
  | 'poison'
  | 'psychic'
  | 'radiant'
  | 'slashing'
  | 'thunder';

/** The damage types in the order the select lists them (alphabetical in
 * Portuguese), with the labels the SRD translation uses in the rules
 * content ("concussão", "elétrico", "energia"). Capitalised for a select;
 * `damageTypeLabel` lower-cases it for a sentence. */
export const DAMAGE_TYPE_OPTIONS: readonly { key: Exclude<DamageTypeKey, ''>; label: string }[] = [
  { key: 'acid', label: 'Ácido' },
  { key: 'slashing', label: 'Cortante' },
  { key: 'bludgeoning', label: 'Concussão' },
  { key: 'lightning', label: 'Elétrico' },
  { key: 'force', label: 'Energia' },
  { key: 'fire', label: 'Fogo' },
  { key: 'cold', label: 'Frio' },
  { key: 'necrotic', label: 'Necrótico' },
  { key: 'piercing', label: 'Perfurante' },
  { key: 'psychic', label: 'Psíquico' },
  { key: 'radiant', label: 'Radiante' },
  { key: 'thunder', label: 'Trovejante' },
  { key: 'poison', label: 'Veneno' },
];

/** "cortante" for `'slashing'`; empty for `''`. */
export function damageTypeLabel(key: DamageTypeKey): string {
  return DAMAGE_TYPE_OPTIONS.find((o) => o.key === key)?.label.toLowerCase() ?? '';
}

/** "1d6 + 2": dice, then the bonus with spaces around its sign (a real
 * minus), left out when zero. */
export function formatDamageDice(count: number, sides: number, bonus: number): string {
  const dice = `${count}d${sides}`;
  return bonus === 0 ? dice : `${dice} ${bonus > 0 ? '+' : '−'} ${Math.abs(bonus)}`;
}

/**
 * "dd/MM/yyyy HH:mm", the one date format this app shows (the "Ficha
 * travada desde ..." banner). A plain formatter instead of Angular's
 * `DatePipe` on purpose: `DatePipe`'s locale/`Intl` machinery lives in
 * `@angular/common`'s shared module chunk, which is already eager (`App`
 * needs `CommonModule`) — the first screen to use `DatePipe` pulls several
 * more kB of that chunk into the eager bundle even though the screen
 * itself is lazy-loaded. Local time, matching how the timestamp already
 * displays everywhere else in this app (no timezone conversion needed:
 * `Timestamp` → `Date` already carries the browser's local time).
 */
export function formatDateTime(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  const day = pad(date.getDate());
  const month = pad(date.getMonth() + 1);
  const hours = pad(date.getHours());
  const minutes = pad(date.getMinutes());
  return `${day}/${month}/${date.getFullYear()} ${hours}:${minutes}`;
}

/** How a skill's proficiency level reads out loud (screen readers, and the
 * sheet's own visible label next to each skill bonus). */
export type SkillProficiency = 'none' | 'half' | 'proficient' | 'expertise';

export function skillProficiencyLabel(proficiency: SkillProficiency): string {
  switch (proficiency) {
    case 'none':
      return 'sem proficiência';
    case 'half':
      return 'meia proficiência';
    case 'proficient':
      return 'proficiente';
    case 'expertise':
      return 'expertise';
  }
}

/**
 * "1º círculo", "9º círculo" — "círculo" is the pt-BR D&D term for a
 * spell's level (never "nível", which this app reserves for a
 * character's own level). A cantrip (level 0) is a "Truque", never called
 * a "círculo" (integrator fix). Used consistently on both the sheet (spell
 * slots) and the editor (each spell option's level).
 */
export function spellLevelLabel(level: number): string {
  return level === 0 ? 'Truque' : `${level}º círculo`;
}

/**
 * "1º círculo: 4 · 2º círculo: 2" — every spell level with at least one
 * slot, in order, index 0 = level 1 (`FullSheetVm.spellSlots`'s own
 * contract). A plain string instead of one `<span>` per level in the
 * template: Angular inserts no whitespace between sibling elements, so a
 * template `@for` there used to render "1º nível: 42º nível: 2" with no
 * separator at all (integrator fix).
 */
export function formatSpellSlots(slots: readonly number[]): string {
  return slots
    .map((count, i) => (count > 0 ? `${spellLevelLabel(i + 1)}: ${count}` : null))
    .filter((entry): entry is string => entry !== null)
    .join(' · ');
}

/**
 * "0 fichas travadas.", "1 ficha travada.", "N fichas travadas." — pt-BR
 * singular/plural of both the noun ("ficha"/"fichas") and its participle
 * ("travada"/"travadas") agree with the count (integrator fix: this used
 * to always say "fichas travadas", even for exactly one).
 */
export function lockedSheetCountLabel(count: number): string {
  return count === 1 ? '1 ficha travada.' : `${count} fichas travadas.`;
}

/** The armor's own name, split from the shield note that may follow it. */
export interface ArmorDescriptionVm {
  readonly armorNamePt: string;
  readonly hasShield: boolean;
}

/**
 * The exact suffix `armorClass` (`backend/internal/rules/armor.go`) appends
 * to `DerivedSheet.armor_class_description` when the sheet carries a
 * shield — see that function's `name += " + escudo"`.
 */
const SHIELD_SUFFIX = ' + escudo';

/**
 * Splits "Armadura de couro + escudo" into the armor's own name ("Armadura
 * de couro") and whether a shield is carried, or "Sem armadura" with no
 * shield into that name and `false`. "Equipamento" uses this to show the
 * armor and the shield as two separate lines, reading both facts from the
 * one string the server already sends — the sheet does not compute a rule
 * (ADR-0008) or ask for a second field for something `armor_class_description`
 * already carries.
 */
export function splitArmorDescription(description: string): ArmorDescriptionVm {
  if (description.endsWith(SHIELD_SUFFIX)) {
    return { armorNamePt: description.slice(0, -SHIELD_SUFFIX.length), hasShield: true };
  }
  return { armorNamePt: description, hasShield: false };
}
