import type { Attack } from '../../../gen/meurpg/rules/v1/rules_pb';
import { type AmmunitionKind, ammunitionKindOfKey } from '../characters/charges';

/** The tag of a counted kind on an attack row: "3 flechas"; `warning` at 3 or fewer, `empty` at 0 (the shot does not go). */
export interface AmmunitionTag {
  readonly text: string;
  readonly level: 'ok' | 'warning' | 'empty';
}

/** From this many pieces down the tag warns. */
const LOW_AMMUNITION = 3;

const MELEE_SUFFIX = ':melee';

/** The kind a line fires, from the ammunition key the server sends on it; `null` for a line without one or with a key the table does not know. */
function kindOf(attack: Attack): AmmunitionKind | null {
  return ammunitionKindOfKey(attack.ammunitionKey) ?? null;
}

/** The count tag of a ranged line whose kind the app counts; `null` for every other line (no `ammunition_left`: not counted). */
export function ammunitionTag(attack: Attack): AmmunitionTag | null {
  const left = attack.ammunitionLeft;
  if (left === undefined) {
    return null;
  }
  const kind = kindOf(attack);
  const text = kind
    ? `${left} ${left === 1 ? kind.one : kind.many}`
    : `${left} ${left === 1 ? 'munição' : 'munições'}`;
  return { text, level: left === 0 ? 'empty' : left <= LOW_AMMUNITION ? 'warning' : 'ok' };
}

/** "Sem flechas: o tiro não sai." at 0, and `''` otherwise: the reason the "Atacar" button is off. */
export function ammunitionEmptyLine(attack: Attack): string {
  if (attack.ammunitionLeft !== 0) {
    return '';
  }
  return `Sem ${kindOf(attack)?.many ?? 'munição'}: o tiro não sai.`;
}

/** The kind of the ranged line of the same weapon as an improvised melee line (its key without ":melee"), among the lines of the options. */
function siblingKind(melee: Attack, lines: readonly { attack?: Attack }[]): AmmunitionKind | null {
  const weapon = melee.key.endsWith(MELEE_SUFFIX)
    ? melee.key.slice(0, -MELEE_SUFFIX.length)
    : melee.key;
  const ranged = lines.find((l) => l.attack?.key === weapon)?.attack;
  return ranged ? kindOf(ranged) : null;
}

/** "Improvisado: não gasta flecha." under the improvised melee line of a weapon of ammunition (the kind comes from its ranged sibling in `lines`); `''` for any other line. */
export function ammunitionNote(attack: Attack, lines: readonly { attack?: Attack }[]): string {
  return attack.improvised
    ? `Improvisado: não gasta ${siblingKind(attack, lines)?.one ?? 'munição'}.`
    : '';
}

/** What spends ammunition, under the attack list while a kind is counted. */
export const AMMUNITION_RULE =
  'Cada tiro gasta uma flecha: o Ataque Extra gasta duas. Uma besta com Recarga dispara uma vez por ação. Arremesso e corpo a corpo não gastam.';

/** Some attack of the list counts ammunition. */
export const countsAmmunition = (attacks: readonly { attack?: Attack }[]): boolean =>
  attacks.some((a) => a.attack?.ammunitionLeft !== undefined);

/** What the attack rows and the list under them say about ammunition, by name, for a template and its component. */
export const AMMUNITION = {
  tag: ammunitionTag,
  empty: ammunitionEmptyLine,
  note: ammunitionNote,
  rule: AMMUNITION_RULE,
  counted: countsAmmunition,
};
