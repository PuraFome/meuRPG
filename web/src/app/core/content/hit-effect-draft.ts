import { clone, create } from '@bufbuild/protobuf';

import { Ability } from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  TableEffectModifierKind,
  type TableEffectModifier,
  TableEffectModifierSchema,
  TableHitConditionDuration,
  TableHitTrigger,
  type TableSpellEffect,
  TableSpellEffectSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { HIT_TRIGGER_PT } from './content-violation-texts-hit';
import { UNREADABLE_COUNT, readWhole, wholeText } from './whole-number';

/**
 * "Efeito no próximo golpe" of the spell editor (#577): the `weapon_hit_damage` modifier a table spell keeps in its effect.
 * The draft edits only that one modifier. The stored effect (its conditions, the other modifiers, who sees it) is the
 * spell's `kept` effect, and `applyHit` puts the modifier back into it without touching anything else, in the place it
 * had. The rules are the server's (`rules/weaponhitdamage.go`); the form only keeps what the person typed and says early
 * what it can: a trigger left unset, the locked card, a spell that does not last, a cap that is missing, a condition with no test.
 */

export type HitTriggerKey = 'next_hit' | 'next_melee_hit' | 'each_hit' | 'marked_target';
/** A trigger of the server, or "" for one a stored modifier has that the form does not know: no card is checked until the person picks. */
export type HitTriggerChoice = HitTriggerKey | '';
export type HitLasts = 'next_turn' | 'spell_end';

export interface HitDraft {
  enabled: boolean;
  trigger: HitTriggerChoice;
  /** The dice at the spell's own level: text, so a half-typed number stays. */
  dice: string;
  /** The die's faces: 4, 6, 8, 10 or 12. */
  die: number;
  /** The dice added for each level above the spell's own; empty is 0. */
  perSlot: string;
  /** The most dice at any level. The server requires it (1 to 20): empty is refused before the save. */
  cap: string;
  /** The SRD damage type key; empty is the weapon's own. */
  damageType: string;
  /** The ability key of the target's saving throw ("con"); empty is no test. */
  saveAbility: string;
  /** The condition a failed test gives ("condition:blinded"); empty is none. */
  condition: string;
  /** Conditions after the first, kept as they were stored (the form edits one). */
  extraConditions: readonly string[];
  conditionLasts: HitLasts;
}

export const HIT_NEXT: HitTriggerKey = 'next_hit';
export const HIT_EACH: HitTriggerKey = 'each_hit';
export const HIT_MARKED: HitTriggerKey = 'marked_target';
/** The spell's duration that keeps no effect (`DurationChoice`), and the target the marked target needs. */
const DURATION_INSTANT = 'instant';
const TARGET_ONE_CREATURE = 'creature';

export const HIT_DICE_MAX = 10;
export const HIT_PER_SLOT_MAX = 5;
export const HIT_CAP_MAX = 20;
/** The die's faces the server takes (`weaponHitDice`): d4, d6, d8, d10 and d12. */
// eslint-disable-next-line @typescript-eslint/no-magic-numbers -- the faces of the dice, as the server lists them
export const HIT_DICE_FACES: readonly number[] = [4, 6, 8, 10, 12];

/** What the card of the marked target says while it cannot be taken. */
export const MARKED_LOCK_PT = 'Pede o alvo Uma criatura.';
export const MARKED_LOCK_SLOT_PT =
  'Pede o alvo Uma criatura, sem criaturas a mais por nível de espaço.';
export const INSTANT_LOCK_PT =
  'Uma magia instantânea não guarda efeito para o golpe. Escolha uma duração em Dados da magia.';
export const NEEDS_CONCENTRATION_PT =
  'Este gatilho dura enquanto você se concentra. Ligue Concentração em Dados da magia.';
export const HIT_CAP_PROMPT_PT = 'Diga o máximo de dados, de 1 a 20.';
export const HIT_CONDITION_PAIR_PT = 'Escolha a condição de quem falhar, ou tire o teste.';

export function emptyHit(): HitDraft {
  return {
    enabled: false,
    trigger: HIT_NEXT,
    dice: '1',
    die: 6,
    perSlot: '',
    cap: '',
    damageType: '',
    saveAbility: '',
    condition: '',
    extraConditions: [],
    conditionLasts: 'next_turn',
  };
}

const TRIGGER: Record<HitTriggerKey, TableHitTrigger> = {
  next_hit: TableHitTrigger.NEXT_HIT,
  next_melee_hit: TableHitTrigger.NEXT_MELEE_HIT,
  each_hit: TableHitTrigger.EACH_HIT,
  marked_target: TableHitTrigger.MARKED_TARGET,
};

/** Whether the trigger lasts as long as the caster concentrates (the server's `HitTriggerNeedsConcentration`). */
export function needsConcentration(trigger: HitTriggerChoice): boolean {
  return trigger === HIT_EACH || trigger === HIT_MARKED;
}

const ABILITY_KEY: Readonly<Record<number, string>> = {
  [Ability.STRENGTH]: 'str',
  [Ability.DEXTERITY]: 'dex',
  [Ability.CONSTITUTION]: 'con',
  [Ability.INTELLIGENCE]: 'int',
  [Ability.WISDOM]: 'wis',
  [Ability.CHARISMA]: 'cha',
};

/** The wire key of an ability ("con"), which `save` carries; empty for one this does not know. */
export function hitAbilityKey(ability: Ability): string {
  return ABILITY_KEY[ability] ?? '';
}

function triggerChoice(t: TableHitTrigger): HitTriggerChoice {
  return (Object.entries(TRIGGER).find(([, v]) => v === t)?.[0] as HitTriggerKey | undefined) ?? '';
}

const isHit = (m: TableEffectModifier): boolean =>
  m.kind === TableEffectModifierKind.WEAPON_HIT_DAMAGE;

/** Where the weapon-hit modifier sits among the effect's modifiers, -1 when there is none. */
export function hitIndex(effect: TableSpellEffect | undefined): number {
  return effect?.modifiers.findIndex(isHit) ?? -1;
}

/** The hit effect of a stored spell's effect, as the form's block (on when the effect has the modifier). */
export function hitFromEffect(effect: TableSpellEffect | undefined): HitDraft {
  const i = hitIndex(effect);
  const m = i < 0 ? undefined : effect?.modifiers[i];
  if (!m) {
    return emptyHit();
  }
  const [first = '', ...rest] = m.onFailConditions;
  return {
    enabled: true,
    trigger: triggerChoice(m.trigger),
    dice: wholeText(m.dice),
    die: m.die,
    perSlot: wholeText(m.perSlotDice),
    cap: wholeText(m.maxDice),
    damageType: m.damageType,
    saveAbility: m.save,
    condition: first,
    extraConditions: rest,
    conditionLasts:
      m.conditionDuration === TableHitConditionDuration.UNTIL_SPELL_END ? 'spell_end' : 'next_turn',
  };
}

/** A whole number from a count field's text; one that is no number goes as 0 (the form refuses it before the save). */
const count = (text: string): number => {
  const n = readWhole(text, false);
  return Number.isNaN(n) ? 0 : n;
};

/** The modifier the draft says, with only the fields the weapon hit reads. */
export function hitToModifier(h: HitDraft): TableEffectModifier {
  const hasSave = h.saveAbility !== '';
  const conditions = h.condition ? [h.condition, ...h.extraConditions] : [];
  return create(TableEffectModifierSchema, {
    kind: TableEffectModifierKind.WEAPON_HIT_DAMAGE,
    trigger: h.trigger === '' ? TableHitTrigger.UNSPECIFIED : TRIGGER[h.trigger],
    die: h.die,
    dice: count(h.dice),
    perSlotDice: count(h.perSlot),
    maxDice: count(h.cap),
    damageType: h.damageType,
    ...(hasSave
      ? {
          save: h.saveAbility,
          onFailConditions: conditions,
          conditionDuration:
            h.conditionLasts === 'spell_end'
              ? TableHitConditionDuration.UNTIL_SPELL_END
              : TableHitConditionDuration.UNTIL_END_OF_NEXT_TURN,
        }
      : {}),
  });
}

/** The server refuses an effect with no condition and no modifier, and the audience and the label mean nothing without them. */
const emptyEffect = (e: TableSpellEffect): boolean =>
  e.modifiers.length === 0 && e.conditions.length === 0;

/**
 * The effect to send: the stored one with the weapon-hit modifier written in (in its own place, or last when it is new),
 * or taken out when the block is off or the spell cannot keep it. Nothing else changes: the other modifiers keep their
 * order, and the conditions, `applies`, the audience and the label go through. An effect with no condition and no modifier left is `undefined` (the server refuses one, and its label and audience mean nothing alone);
 * a spell with no effect has one made with only this modifier.
 */
export function applyHit(
  effect: TableSpellEffect | undefined,
  hit: HitDraft,
  available: boolean,
): TableSpellEffect | undefined {
  const i = hitIndex(effect);
  if (hit.enabled && available) {
    const mod = hitToModifier(hit);
    if (!effect) {
      return create(TableSpellEffectSchema, { modifiers: [mod] });
    }
    const next = clone(TableSpellEffectSchema, effect);
    next.modifiers =
      i < 0 ? [...next.modifiers, mod] : next.modifiers.map((m, j) => (j === i ? mod : m));
    return next;
  }
  if (!effect || i < 0) {
    return effect;
  }
  const next = clone(TableSpellEffectSchema, effect);
  next.modifiers = next.modifiers.filter((_, j) => j !== i);
  return emptyEffect(next) ? undefined : next;
}

/** The spell facts the block depends on: the form's target and duration, and the extra targets per level. */
export interface HitSpellFacts {
  readonly target: string;
  readonly perSlot: number;
  readonly duration: string;
}

/** An instantaneous spell has no effect to keep for the golpe. */
export function hitAvailable(d: Pick<HitSpellFacts, 'duration'>): boolean {
  return d.duration !== DURATION_INSTANT;
}

/** Why the card of the marked target is locked, or "" when it is open: it needs one creature and no extra targets per level. */
export function markedLock(d: Pick<HitSpellFacts, 'target' | 'perSlot'>): string {
  if (d.target !== TARGET_ONE_CREATURE) {
    return MARKED_LOCK_PT;
  }
  return d.perSlot > 0 ? MARKED_LOCK_SLOT_PT : '';
}

/** The draft with a locked trigger moved to the next hit (a marked target the spell can no longer have). */
export function settleHit<T extends HitSpellFacts & { hit: HitDraft }>(d: T): T {
  if (d.hit.trigger === HIT_MARKED && markedLock(d) !== '') {
    return { ...d, hit: { ...d.hit, trigger: HIT_NEXT } };
  }
  return d;
}

/** The path of the weapon-hit modifier's field in a request, where the server's refusals land ("" field is the modifier's `kind`). */
export function hitPath(effect: TableSpellEffect | undefined, field: string): string {
  const i = hitIndex(effect);
  const at = i < 0 ? (effect?.modifiers.length ?? 0) : i;
  return `table_spell.effect.modifiers[${at}].${field}`;
}

/** The inputs the block draws while it is on (the paths where a refusal lands), without the `table_spell.` start. */
export function hitFieldPaths(effect: TableSpellEffect | undefined): string[] {
  const base = hitPath(effect, '').slice('table_spell.'.length);
  const fields = [
    'kind',
    'trigger',
    'dice',
    'die',
    'per_slot_dice',
    'max_dice',
    'damage_type',
    'save',
    'on_fail_conditions',
    'condition_duration',
  ];
  return fields.map((f) => `${base}${f}`);
}

export interface HitRefusal {
  readonly path: string;
  readonly text: string;
}

/**
 * What the form says before the save, at the field: a number that is no whole number, the cap the server requires, and the
 * condition and the test that need each other. Nothing here when the block is off or the spell cannot keep it.
 */
export function hitRefusals(
  effect: TableSpellEffect | undefined,
  h: HitDraft,
  available: boolean,
): HitRefusal[] {
  if (!h.enabled || !available) {
    return [];
  }
  const at = (f: string) => hitPath(effect, f);
  const out: HitRefusal[] = [];
  if (h.trigger === '') out.push({ path: at('trigger'), text: HIT_TRIGGER_PT });
  const bad = (text: string) => Number.isNaN(readWhole(text, false));
  if (bad(h.dice)) out.push({ path: at('dice'), text: UNREADABLE_COUNT });
  if (bad(h.perSlot)) out.push({ path: at('per_slot_dice'), text: UNREADABLE_COUNT });
  if (bad(h.cap)) {
    out.push({ path: at('max_dice'), text: UNREADABLE_COUNT });
  } else if (h.cap.trim() === '') {
    out.push({ path: at('max_dice'), text: HIT_CAP_PROMPT_PT });
  }
  if (h.saveAbility !== '' && h.condition === '') {
    out.push({ path: at('on_fail_conditions'), text: HIT_CONDITION_PAIR_PT });
  }
  return out;
}
