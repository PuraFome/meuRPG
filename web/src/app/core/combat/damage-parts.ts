import type {
  DamagePart,
  DamagePartRoll,
  SlotOption,
} from '../../../gen/meurpg/play/v1/combat_rolls_pb';

/** What the player marked for one extra: the slot Divine Smite spends. */
export interface ExtraPick {
  readonly key: string;
  readonly slotLevel: number;
  readonly pact: boolean;
}

/** The slot an extra is marked with when nothing was chosen: the lowest one that is free. */
export function defaultSlot(part: DamagePart): SlotOption | null {
  return (
    [...part.slotOptions].filter((o) => o.free > 0).sort((a, b) => a.level - b.level)[0] ?? null
  );
}

/** The extras that come marked (`selected`), each with its slot when it needs one. */
export function initialPicks(parts: readonly DamagePart[]): readonly ExtraPick[] {
  return parts
    .filter((p) => p.choosable && p.selected && p.available)
    .map((p) => {
      const slot = p.needsSlot ? defaultSlot(p) : null;
      return { key: p.key, slotLevel: slot?.level ?? 0, pact: slot?.pact ?? false };
    });
}

/** The dice of a part with the slot it is marked with (Divine Smite rolls the slot's dice). */
export function partDice(part: DamagePart, pick?: ExtraPick): { count: number; sides: number } {
  const slot =
    part.needsSlot && pick
      ? part.slotOptions.find((o) => o.level === pick.slotLevel && o.pact === pick.pact)
      : undefined;
  return { count: slot?.diceCount ?? part.diceCount, sides: part.diceSides };
}

/** "2d6 + 3": the dice and the number added without rolling. */
export function partFormula(part: DamagePart, pick?: ExtraPick): string {
  const { count, sides } = partDice(part, pick);
  const dice = count > 0 ? `${count}d${sides}` : '';
  if (part.flat === 0) {
    return dice || '0';
  }
  const flat = `${part.flat < 0 ? '−' : '+'} ${Math.abs(part.flat)}`;
  return dice ? `${dice} ${flat}` : `${part.flat}`;
}

/**
 * A spell's hit effect that this weapon does not carry (a melee-only effect on a ranged line): listed with its
 * dice and its reason, never choosable, never rolled, never in the total. Weapon and automatic lines are never
 * `available` on the wire, so what marks it is the reason.
 */
export function isOffPart(part: DamagePart): boolean {
  return !part.choosable && !part.available && !!part.reasonPt;
}

/** The parts that count in the damage: the weapon and the automatic lines, and the extras that are marked. */
export function activeParts(
  parts: readonly DamagePart[],
  picks: readonly ExtraPick[],
): readonly DamagePart[] {
  return parts.filter((p) => (p.choosable ? picks.some((x) => x.key === p.key) : !isOffPart(p)));
}

/**
 * What "Rolar ... no app" rolls: the dice of every part that counts, then the fixed numbers added up
 * ("1d8 + 2d6 + 3", "1d8 − 1"). Nothing but fixed numbers reads as the number alone.
 */
export function rollFormula(parts: readonly DamagePart[], picks: readonly ExtraPick[]): string {
  const active = activeParts(parts, picks);
  const dice = active
    .map((p) =>
      partDice(
        p,
        picks.find((x) => x.key === p.key),
      ),
    )
    .filter((d) => d.count > 0)
    .map((d) => `${d.count}d${d.sides}`);
  const flat = active.reduce((sum, p) => sum + p.flat, 0);
  if (dice.length === 0) {
    return `${flat}`;
  }
  const bonus = flat === 0 ? '' : ` ${flat < 0 ? '−' : '+'} ${Math.abs(flat)}`;
  return `${dice.join(' + ')}${bonus}`;
}

/** What a part took off the damage if it were taken out: its dice and its fixed number. */
export function partTotal(roll: DamagePartRoll): number {
  return roll.sum + roll.flat;
}

/**
 * The part that carries the sum of a typed damage-type group when `roll` is another dice part of it: a part with dice, no faces
 * and sum 0 that counts, while a counted part of the same damage type has no faces and a sum. The player typed one sum for the
 * type, so the server keeps it on the first part and the others read 0; "= 0" would say they did nothing. Undefined for any
 * other part (a rolled part, a flat part, a part whose group holder is hidden from the reader has no holder here either).
 */
export function groupHolder(
  roll: DamagePartRoll,
  rolls: readonly DamagePartRoll[],
): DamagePartRoll | undefined {
  if (roll.diceCount <= 0 || roll.faces.length > 0 || roll.sum !== 0 || !roll.counted) {
    return undefined;
  }
  if (roll.damageTypeKey === '') {
    return undefined;
  }
  return rolls.find(
    (o) =>
      o !== roll &&
      o.counted &&
      o.damageTypeKey === roll.damageTypeKey &&
      o.diceCount > 0 &&
      o.faces.length === 0 &&
      o.sum > 0,
  );
}

/**
 * Whether `roll` is a part of a typed damage-type group: the part that carries the group's sum (a counted part with dice, no
 * faces and a sum, beside a same-type counted follower) or a follower of it (`groupHolder`). The master cannot take such a
 * part out. The server does not send the group on the wire, so this is the heuristic of the rows: a group whose holder or
 * followers are masked from the reader (a part hidden from the others) is not seen here, and the server's refusal stands.
 */
export function inTypedGroup(roll: DamagePartRoll, rolls: readonly DamagePartRoll[]): boolean {
  if (groupHolder(roll, rolls)) {
    return true;
  }
  return (
    roll.counted &&
    roll.diceCount > 0 &&
    roll.faces.length === 0 &&
    roll.sum > 0 &&
    roll.damageTypeKey !== '' &&
    rolls.some((o) => o !== roll && groupHolder(o, rolls) === roll)
  );
}

/** "rolou de novo 1→4" for each die Great Weapon Fighting rolled again. */
export function rerollText(roll: DamagePartRoll): string {
  return roll.rerolled.map((r) => `rolou de novo ${r.from}→${r.to}`).join(', ');
}
