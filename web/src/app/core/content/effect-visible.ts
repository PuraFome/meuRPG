import type { EffectMenuField } from '../../../gen/meurpg/rules/v1/table_content_pb';

/** What `visibleFields` reads of a draft. */
interface Shape {
  readonly type: string;
  readonly applies: string;
  readonly spellChange: string;
}

/** What a spell_modifier's change reads besides the fields every change has: the swap lists the types it swaps among, the bonus
 * damage its amount and type, the extra dice their count. The other fields stay out of the editor (and of the request). */
const SPELL_CHANGE_FIELDS: Readonly<Record<string, readonly string[]>> = {
  damage_type_swap: ['from'],
  bonus_damage: ['value', 'damage_type'],
  extra_dice_per_slot_over: ['count'],
};
const SPELL_CHANGE_ONLY: ReadonlySet<string> = new Set(Object.values(SPELL_CHANGE_FIELDS).flat());

/** A superiority_die on a roll of the character's own (checks, saves, initiative) reads fewer fields: only the checks take a
 * `targets` list, only a save an `ability`, and none of the three costs an action. */
const ROLL_DIE_HIDES: Readonly<Record<string, readonly string[]>> = {
  check: ['ability', 'economy'],
  save: ['targets', 'economy'],
  initiative: ['ability', 'targets', 'economy'],
};

/** The fields of the draft's type that apply to it now: a spell_modifier shows only what its chosen change reads (none of the
 * change's own fields until one is chosen), a superiority_die on a roll only what that roll reads; every other type shows all
 * of its menu fields. */
export function visibleFields(
  fields: readonly EffectMenuField[],
  d: Shape,
): readonly EffectMenuField[] {
  if (d.type === 'superiority_die') {
    const hidden = ROLL_DIE_HIDES[d.applies] ?? ['targets'];
    return fields.filter((f) => !hidden.includes(f.name));
  }
  if (d.type !== 'spell_modifier') {
    return fields;
  }
  const own = SPELL_CHANGE_FIELDS[d.spellChange] ?? [];
  return fields.filter((f) => !SPELL_CHANGE_ONLY.has(f.name) || own.includes(f.name));
}
