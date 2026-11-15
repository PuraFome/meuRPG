import type { Attack } from '../../../gen/meurpg/rules/v1/rules_pb';

/** The tag of a counted kind on an attack row: "3 flechas"; `warning` at 3 or fewer, `empty` at 0 (the shot does not go). */
export interface AmmunitionTag {
  readonly text: string;
  readonly level: 'ok' | 'warning' | 'empty';
}

interface Kind {
  readonly one: string;
  readonly many: string;
}

const ARROW: Kind = { one: 'flecha', many: 'flechas' };
const BOLT: Kind = { one: 'virote', many: 'virotes' };
const BULLET: Kind = { one: 'bala de funda', many: 'balas de funda' };
const NEEDLE: Kind = { one: 'agulha de zarabatana', many: 'agulhas de zarabatana' };

/** The SRD weapons that fire ammunition, by key: the attack does not carry the kind, so the weapon names it. */
const KIND_BY_WEAPON: Readonly<Record<string, Kind>> = {
  'equipment:shortbow': ARROW,
  'equipment:longbow': ARROW,
  'equipment:light-crossbow': BOLT,
  'equipment:hand-crossbow': BOLT,
  'equipment:heavy-crossbow': BOLT,
  'equipment:sling': BULLET,
  'equipment:blowgun': NEEDLE,
};

/** From this many pieces down the tag warns. */
const LOW_AMMUNITION = 3;

const MELEE_SUFFIX = ':melee';

/** The kind a weapon fires, from its key (the improvised melee line carries the weapon's key and ":melee"); `null` for a weapon of the table. */
function kindOf(attack: Attack): Kind | null {
  const weapon = attack.key.endsWith(MELEE_SUFFIX)
    ? attack.key.slice(0, -MELEE_SUFFIX.length)
    : attack.key;
  return KIND_BY_WEAPON[weapon] ?? null;
}

/** The count tag of a ranged line whose kind the app counts; `null` for every other line (no `ammunition_left`: not counted). */
export function ammunitionTag(attack: Attack): AmmunitionTag | null {
  const left = attack.ammunitionLeft;
  if (left === undefined) {
    return null;
  }
  const kind = kindOf(attack);
  const text = kind ? `${left} ${left === 1 ? kind.one : kind.many}` : `${left} de munição`;
  return { text, level: left === 0 ? 'empty' : left <= LOW_AMMUNITION ? 'warning' : 'ok' };
}

/** "Sem flechas: o tiro não sai." at 0, and `''` otherwise: the reason the "Atacar" button is off. */
export function ammunitionEmptyLine(attack: Attack): string {
  if (attack.ammunitionLeft !== 0) {
    return '';
  }
  return `Sem ${kindOf(attack)?.many ?? 'munição'}: o tiro não sai.`;
}

/** "Improvisado: não gasta flecha." under the improvised melee line of a weapon of ammunition; `''` for any other line. */
export function ammunitionNote(attack: Attack): string {
  return attack.improvised ? `Improvisado: não gasta ${kindOf(attack)?.one ?? 'munição'}.` : '';
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
