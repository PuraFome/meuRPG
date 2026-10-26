import type { Attack, Creature } from '../../../../../gen/meurpg/rules/v1/rules_pb';

/** One part of a routine: "2 × Pancada", with the attack of the NPC that makes it (`''` when the card has none by that name). */
export interface RoutinePart {
  readonly name: string;
  readonly count: number;
  readonly attackKey: string;
}

/**
 * One way to spend a creature's Multiattack (SRD 5.1): the attacks the master makes one after another, each with its own
 * target. `total` counts the parts that are attacks of the stat block (a gaze or a spell is no attack roll).
 */
export interface Routine {
  /** "2 × Pancada", "1 × Mordida + 1 × Garras". */
  readonly label: string;
  readonly total: number;
  readonly parts: readonly RoutinePart[];
}

/** The attack of the card a stat block's action is: same name in Portuguese (the sheet cuts a long name at its limit). */
function attackKeyOf(name: string, attacks: readonly Attack[]): string {
  const found = attacks.find((a) => {
    const n = a.namePt || a.name;
    return n === name || (n.length > 0 && name.startsWith(n));
  });
  return found?.key ?? '';
}

/**
 * The routines of the creature's Multiattack, as the rules API sends them (`CreatureAction.multiattack`), named in
 * Portuguese and tied to the attacks the NPC has. Empty for no stat block and for a creature with no Multiattack, which
 * keeps the card as it was.
 */
export function routinesOf(creature: Creature | null, attacks: readonly Attack[]): Routine[] {
  const multi = creature?.actions.find((a) => a.multiattack.length > 0);
  if (!creature || !multi) {
    return [];
  }
  return multi.multiattack.map((routine) => {
    let total = 0;
    const parts: RoutinePart[] = [];
    const words: string[] = [];
    for (const x of routine.attacks) {
      const action = creature.actions.find((a) => a.name === x.name);
      const name = action?.namePt || action?.name || x.name;
      words.push(`${x.countText || x.count} × ${name}`);
      if (action?.hasAttack) {
        total += x.count;
        parts.push({ name, count: x.count, attackKey: attackKeyOf(name, attacks) });
      }
    }
    return { label: words.join(' + '), total, parts };
  });
}

/** How many attacks of the Attack action were made this turn, from the economy the server counts (0 before the first). */
export function attacksMade(per: number | undefined, left: number | undefined): number {
  if (per === undefined || left === undefined) {
    return 0;
  }
  return Math.max(per - left, 0);
}
