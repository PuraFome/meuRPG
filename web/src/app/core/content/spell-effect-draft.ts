import { clone, create, type MessageInitShape } from '@bufbuild/protobuf';

import {
  TableAuraAffects,
  TableEffectApplies,
  TableEffectAudience,
  type TableEffectModifier,
  TableEffectModifierKind as K,
  TableEffectModifierSchema,
  TableEffectRollKind,
  TableResistanceKind,
  type TableSpellEffect,
  TableSpellEffectSchema,
  TableEffectAuraSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { metersToFeet } from '../units';
import { rangeMeters } from './effect-draft';

/**
 * The "Deixa um efeito nos alvos" block of the spell editor (#579): a `TableSpell.effect`. The server's modifiers are a closed set of
 * fourteen kinds; the editor offers the seven a table spell can leave (`EFFECT_KINDS`), and a "Vantagem ou desvantagem" row is up to three
 * modifiers (attacks, saves, checks). Numbers stay as the text typed and are read when the save is built, so a half-typed "1" never jumps;
 * what is no whole number is said by `effectUnreadable` and sent as 0, which the server refuses on the same field.
 */

export type EffectKind =
  'ac_bonus' | 'roll' | 'check_bonus' | 'temp_hp' | 'resistance' | 'speed' | 'advantage';

export type RollWord = 'attack' | 'save' | 'check';
export type EffectApplyChoice = 'all' | 'failed_save' | 'hit';
export type AuraAffectsChoice = 'self' | 'allies' | 'enemies';

/** One row of "O que muda". `kind` is '' while it is not chosen and 'unknown' for a modifier the menu does not offer (kept as it came). */
export interface EffectRow {
  readonly id: number;
  readonly kind: EffectKind | '' | 'unknown';
  /** ac_bonus, temp_hp, check_bonus and the fixed number of a roll: text. */
  readonly value: string;
  readonly skill: string;
  readonly rollMode: 'fixed' | 'die';
  /** The die's faces, "4" for a d4. */
  readonly die: string;
  readonly sign: 1 | -1;
  readonly rolls: readonly RollWord[];
  readonly resistance: 'resist' | 'immune' | 'vulnerable';
  readonly damageTypes: readonly string[];
  readonly speedMode: 'add' | 'multiply';
  /** Metres, in steps of 1,5 (stored as feet). */
  readonly speedM: string;
  readonly percent: string;
  readonly advantage: 'adv' | 'dis';
  readonly abilities: readonly string[];
  /** The modifier of an 'unknown' row, sent back untouched. */
  readonly raw?: TableEffectModifier;
}

export interface AuraDraft {
  /** Metres. */
  readonly radiusM: string;
  readonly affects: AuraAffectsChoice;
}

export interface SpellEffectDraft {
  readonly on: boolean;
  readonly applies: EffectApplyChoice;
  readonly conditions: readonly string[];
  readonly rows: readonly EffectRow[];
  readonly visibility: 'public' | 'owner';
  readonly label: string;
  readonly aura: AuraDraft | null;
  /** The stored effect, so a field this build does not know goes back as it was. */
  readonly base?: TableSpellEffect;
}

export const MAX_EFFECT_ITEMS = 8;
export const MAX_LABEL = 30;
export const DEFAULT_AURA: AuraDraft = { radiusM: '9', affects: 'allies' };

export const EFFECT_KINDS: readonly { value: EffectKind; label: string; hint: string }[] = [
  { value: 'ac_bonus', label: 'Bônus na CA', hint: '' },
  {
    value: 'roll',
    label: 'Bônus nas jogadas',
    hint: 'número ou dado, em ataque, resistência ou teste',
  },
  { value: 'check_bonus', label: 'Bônus numa perícia', hint: '' },
  { value: 'temp_hp', label: 'PV temporários', hint: '' },
  { value: 'resistance', label: 'Resistência, imunidade ou vulnerabilidade', hint: '' },
  { value: 'speed', label: 'Deslocamento', hint: 'soma metros ou multiplica' },
  {
    value: 'advantage',
    label: 'Vantagem ou desvantagem',
    hint: 'em ataques, resistências ou testes',
  },
];

/** The kinds an aura cannot give: they are written on the creature when the effect lands (the server refuses them at `effect.modifiers[i].kind`). */
export const AURA_REFUSED: readonly EffectKind[] = ['ac_bonus', 'speed', 'temp_hp'];

export const AURA_REFUSED_PT =
  'Uma aura só dá o que a jogada lê no momento: não vale CA, deslocamento nem PV temporários, e não põe condição.';

let nextRow = 1;

export function emptyRow(kind: EffectKind | '' | 'unknown' = ''): EffectRow {
  return {
    id: nextRow++,
    kind,
    value: '',
    skill: '',
    rollMode: 'fixed',
    die: '4',
    sign: 1,
    rolls: ['attack', 'save'],
    resistance: 'resist',
    damageTypes: [],
    speedMode: 'add',
    speedM: '',
    percent: '50',
    advantage: 'adv',
    abilities: [],
  };
}

export function emptyEffect(): SpellEffectDraft {
  return {
    on: false,
    applies: 'all',
    conditions: [],
    rows: [],
    visibility: 'public',
    label: '',
    aura: null,
  };
}

const ROLL_TO_ENUM: Record<RollWord, TableEffectRollKind> = {
  attack: TableEffectRollKind.ATTACK,
  save: TableEffectRollKind.SAVE,
  check: TableEffectRollKind.CHECK,
};
export const ROLL_WORDS: readonly RollWord[] = ['attack', 'save', 'check'];
const RES_TO_ENUM = {
  resist: TableResistanceKind.RESIST,
  immune: TableResistanceKind.IMMUNE,
  vulnerable: TableResistanceKind.VULNERABLE,
} as const;
const APPLIES = {
  all: TableEffectApplies.ALL,
  failed_save: TableEffectApplies.FAILED_SAVE,
  hit: TableEffectApplies.HIT,
} as const;
const AFFECTS = {
  self: TableAuraAffects.SELF,
  allies: TableAuraAffects.ALLIES,
  enemies: TableAuraAffects.ENEMIES,
} as const;

/** The advantage kinds of the three rolls, by whether it is advantage. */
const ADV_KINDS: Record<RollWord, readonly [K, K]> = {
  attack: [K.ATTACK_ADVANTAGE, K.ATTACK_DISADVANTAGE],
  save: [K.SAVE_ADVANTAGE, K.SAVE_DISADVANTAGE],
  check: [K.CHECK_ADVANTAGE, K.CHECK_DISADVANTAGE],
};

/** A whole number the text writes ("-2", "10"), or NaN. Empty is NaN too: the server refuses the 0 it is sent as. */
export function readWhole(text: string): number {
  const t = text.trim();
  return /^[+-]?\d+$/.test(t) ? Number(t) : Number.NaN;
}

export function whole(text: string): number {
  const n = readWhole(text);
  return Number.isNaN(n) ? 0 : n;
}

/** Metres typed ("3", "4,5") as feet, 0 when it is no number. */
export function feetOf(text: string): number {
  const t = text.trim().replace(',', '.');
  return /^\d+(\.\d+)?$/.test(t) ? metersToFeet(Number(t)) : 0;
}

// ---- the modifiers of a stored effect, as rows ----

function advantageRow(m: TableEffectModifier, roll: RollWord, adv: boolean): EffectRow {
  return {
    ...emptyRow('advantage'),
    advantage: adv ? 'adv' : 'dis',
    rolls: [roll],
    abilities: roll === 'attack' ? [] : [...m.abilities],
  };
}

function advantageOf(m: TableEffectModifier): { roll: RollWord; adv: boolean } | undefined {
  for (const roll of ROLL_WORDS) {
    const [a, d] = ADV_KINDS[roll];
    if (m.kind === a || m.kind === d) {
      return { roll, adv: m.kind === a };
    }
  }
  return undefined;
}

function rowOf(m: TableEffectModifier): EffectRow {
  const rolls = m.appliesTo
    .map((r) => ROLL_WORDS.find((w) => ROLL_TO_ENUM[w] === r))
    .filter((w): w is RollWord => !!w);
  switch (m.kind) {
    case K.AC_BONUS:
      return { ...emptyRow('ac_bonus'), value: String(m.value) };
    case K.TEMP_HP:
      return { ...emptyRow('temp_hp'), value: String(m.value) };
    case K.CHECK_BONUS:
      return { ...emptyRow('check_bonus'), value: String(m.value), skill: m.skill };
    case K.ROLL_BONUS:
      return { ...emptyRow('roll'), rollMode: 'fixed', value: String(m.value), rolls };
    case K.ROLL_DIE:
      return {
        ...emptyRow('roll'),
        rollMode: 'die',
        die: String(m.die),
        sign: m.sign < 0 ? -1 : 1,
        rolls,
      };
    case K.RESISTANCE:
      return {
        ...emptyRow('resistance'),
        resistance:
          m.resistanceKind === TableResistanceKind.IMMUNE
            ? 'immune'
            : m.resistanceKind === TableResistanceKind.VULNERABLE
              ? 'vulnerable'
              : 'resist',
        damageTypes: [...m.damageTypes],
      };
    case K.SPEED_ADD:
      return { ...emptyRow('speed'), speedMode: 'add', speedM: rangeMeters(m.value) };
    case K.SPEED_MULTIPLIER:
      return { ...emptyRow('speed'), speedMode: 'multiply', percent: String(m.value) };
    default:
      return { ...emptyRow('unknown'), raw: m };
  }
}

/** The modifiers as rows: consecutive advantage modifiers of one mode (attack, then save, then check, the way a row writes them) are one row. */
function rowsOf(mods: readonly TableEffectModifier[]): EffectRow[] {
  const out: EffectRow[] = [];
  let open: { row: EffectRow; last: number } | undefined;
  for (const m of mods) {
    const adv = advantageOf(m);
    if (!adv) {
      open = undefined;
      out.push(rowOf(m));
      continue;
    }
    const at = ROLL_WORDS.indexOf(adv.roll);
    const same = open && (open.row.advantage === 'adv') === adv.adv && at > open.last;
    const abilities = [...m.abilities];
    const joins =
      same &&
      (adv.roll === 'attack' ||
        open!.row.abilities.length === 0 ||
        (open!.row.abilities.length === abilities.length &&
          open!.row.abilities.every((a, i) => a === abilities[i])));
    if (open && joins) {
      const row: EffectRow = {
        ...open.row,
        rolls: [...open.row.rolls, adv.roll],
        abilities: open.row.abilities.length > 0 ? open.row.abilities : abilities,
      };
      out[out.length - 1] = row;
      open = { row, last: at };
    } else {
      const row = advantageRow(m, adv.roll, adv.adv);
      out.push(row);
      open = { row, last: at };
    }
  }
  return out;
}

/** The stored effect of a spell as the form's block; no effect is the block off. */
export function effectToDraft(e: TableSpellEffect | undefined): SpellEffectDraft {
  if (!e) {
    return emptyEffect();
  }
  const aura = e.aura;
  return {
    on: true,
    applies:
      e.applies === TableEffectApplies.FAILED_SAVE
        ? 'failed_save'
        : e.applies === TableEffectApplies.HIT
          ? 'hit'
          : 'all',
    conditions: [...e.conditions],
    rows: rowsOf(e.modifiers),
    // Unset is OWNER on the server: an effect nobody said was public stays private.
    visibility: e.visibility === TableEffectAudience.PUBLIC ? 'public' : 'owner',
    label: e.playerLabelPt,
    aura: aura
      ? {
          radiusM: rangeMeters(aura.radiusFt),
          affects:
            aura.affects === TableAuraAffects.SELF
              ? 'self'
              : aura.affects === TableAuraAffects.ENEMIES
                ? 'enemies'
                : 'allies',
        }
      : null,
    base: e,
  };
}

// ---- rows as modifiers ----

function modifier(init: MessageInitShape<typeof TableEffectModifierSchema>): TableEffectModifier {
  return create(TableEffectModifierSchema, init);
}

/** The modifiers a row sends: none while it has no kind. */
export function rowToModifiers(r: EffectRow): TableEffectModifier[] {
  switch (r.kind) {
    case 'ac_bonus':
      return [modifier({ kind: K.AC_BONUS, value: whole(r.value) })];
    case 'temp_hp':
      return [modifier({ kind: K.TEMP_HP, value: whole(r.value) })];
    case 'check_bonus':
      return [modifier({ kind: K.CHECK_BONUS, value: whole(r.value), skill: r.skill })];
    case 'roll': {
      const appliesTo = r.rolls.map((w) => ROLL_TO_ENUM[w]);
      return [
        r.rollMode === 'die'
          ? modifier({ kind: K.ROLL_DIE, die: whole(r.die), sign: r.sign, appliesTo })
          : modifier({ kind: K.ROLL_BONUS, value: whole(r.value), appliesTo }),
      ];
    }
    case 'resistance':
      return [
        modifier({
          kind: K.RESISTANCE,
          resistanceKind: RES_TO_ENUM[r.resistance],
          damageTypes: [...r.damageTypes],
        }),
      ];
    case 'speed':
      return [
        r.speedMode === 'add'
          ? modifier({ kind: K.SPEED_ADD, value: feetOf(r.speedM) })
          : modifier({ kind: K.SPEED_MULTIPLIER, value: whole(r.percent) }),
      ];
    case 'advantage': {
      const i = r.advantage === 'adv' ? 0 : 1;
      return ROLL_WORDS.filter((w) => r.rolls.includes(w)).map((w) =>
        modifier({
          kind: ADV_KINDS[w][i],
          ...(w === 'attack' ? {} : { abilities: [...r.abilities] }),
        }),
      );
    }
    case 'unknown':
      return r.raw ? [clone(TableEffectModifierSchema, r.raw)] : [];
    default:
      return [];
  }
}

/** The index in the sent modifiers of the first one a row writes, for the paths the server refuses at. */
export function modifierIndexOf(rows: readonly EffectRow[], row: number): number {
  return rows.slice(0, row).reduce((n, r) => n + rowToModifiers(r).length, 0);
}

/** The row, and the field of it, a refusal at `effect.modifiers[i](.field)` is about; -1 when no row writes that modifier. */
export function rowOfModifier(rows: readonly EffectRow[], index: number): number {
  let n = 0;
  for (const [i, r] of rows.entries()) {
    n += rowToModifiers(r).length;
    if (index < n) {
      return i;
    }
  }
  return -1;
}

/** The effect the save sends, or undefined while the block is off. An aura takes no conditions and applies to all (the server's rules for it). */
export function draftToEffect(d: SpellEffectDraft): TableSpellEffect | undefined {
  if (!d.on) {
    return undefined;
  }
  const out = d.base ? clone(TableSpellEffectSchema, d.base) : create(TableSpellEffectSchema);
  out.applies = APPLIES[d.applies];
  out.conditions = d.aura ? [] : [...d.conditions];
  out.modifiers = d.rows.flatMap(rowToModifiers);
  out.visibility =
    d.visibility === 'owner' ? TableEffectAudience.OWNER : TableEffectAudience.PUBLIC;
  out.playerLabelPt = d.label.trim();
  out.aura = d.aura
    ? create(TableEffectAuraSchema, {
        radiusFt: feetOf(d.aura.radiusM),
        affects: AFFECTS[d.aura.affects],
      })
    : undefined;
  return out;
}
