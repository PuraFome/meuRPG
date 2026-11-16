import {
  type ConsumableTarget,
  ConsumableTargetState,
  type UseConsumableResponse,
} from '../../../gen/meurpg/play/v1/charges_pb';
import { CombatantState, ConsumableUseOutcome } from '../../../gen/meurpg/play/v1/combat_pb';
import { stateWord } from '../combat/state-word';
import { article } from '../format/article';
import { upFirst } from '../format/text';
import { metersText } from '../units';
import type { AttunementEntryVm } from './attunement';

/**
 * The consumables of a sheet (potions and scrolls): which magic items are one, the rows that group identical ones,
 * the words of the effect line, the receivers of a gift and the notice of a use. The server words the effect of each item
 * (`SheetMagicItem.effect_pt`) and the use of one (`UseConsumableResponse`); the app only arranges what was sent. Pure,
 * tested without a DOM.
 */

/** What a consumable is: a potion (a flask) or a scroll (a page). */
export type ConsumableKind = 'potion' | 'scroll';

/** The kind of a catalog category, or `null` for an item that is no potion or scroll. */
export function consumableKind(category: string): ConsumableKind | null {
  return category === 'potion' || category === 'scroll' ? category : null;
}

/** A row of the magic items that holds the identical units of one consumable. */
export interface ConsumableRowVm {
  /** Stable for the row while any unit is on the sheet: `consumable:<catalog key>`. */
  readonly id: string;
  readonly key: string;
  readonly namePt: string;
  readonly kind: ConsumableKind;
  readonly icon: string;
  readonly effectPt: string;
  /** The line under the name: the server's, or "Só mostra o texto". */
  readonly effectLine: string;
  readonly effectIcon: string;
  /** How many units the sheet holds: "×2" from 2. */
  readonly count: number;
  /** The unit "Usar" sends: the first of the group (the server keeps one entry for each unit). */
  readonly first: AttunementEntryVm;
}

/** A row of the magic items: an item as it was, or a group of identical consumables. */
export type MagicRowVm =
  | { readonly type: 'item'; readonly id: string; readonly entry: AttunementEntryVm }
  | { readonly type: 'consumable'; readonly id: string; readonly row: ConsumableRowVm };

/** What an item that is no consumable has to say with no effect of its own. */
const TEXT_ONLY_LINE = 'Só mostra o texto';

/** The line of what "Usar" does, "Cura 2d4 + 2", or that the item only shows its text. */
export function effectLine(effectPt: string): string {
  return effectPt === '' ? TEXT_ONLY_LINE : effectPt;
}

/** The icon of an effect: a heart for a healing, a shield for a resistance, a book for the text alone. */
export function effectIcon(effectPt: string): string {
  if (effectPt === '') {
    return 'menu_book';
  }
  return effectPt.startsWith('Resist') ? 'shield' : 'favorite';
}

/**
 * The magic items as the sheet draws them: every consumable of the same key is one row, at the place of the first, and
 * anything else stays a row of its own. `categoryOf` is the catalog category of a key ('' while it is not read).
 */
export function magicRows(
  entries: readonly AttunementEntryVm[],
  categoryOf: (key: string) => string,
): MagicRowVm[] {
  const groups = new Map<string, AttunementEntryVm[]>();
  const kindOf = (e: AttunementEntryVm) => (e.charges ? null : consumableKind(categoryOf(e.key)));
  for (const e of entries) {
    if (kindOf(e)) {
      groups.set(e.key, [...(groups.get(e.key) ?? []), e]);
    }
  }
  const rows: MagicRowVm[] = [];
  const placed = new Set<string>();
  for (const e of entries) {
    const kind = kindOf(e);
    if (!kind) {
      rows.push({ type: 'item', id: e.id, entry: e });
    } else if (!placed.has(e.key)) {
      placed.add(e.key);
      const units = groups.get(e.key) ?? [e];
      const effectPt = e.effectPt ?? '';
      rows.push({
        type: 'consumable',
        id: `consumable:${e.key}`,
        row: {
          id: `consumable:${e.key}`,
          key: e.key,
          namePt: e.namePt,
          kind,
          icon: kind === 'potion' ? 'science' : 'description',
          effectPt,
          effectLine: effectLine(effectPt),
          effectIcon: effectIcon(effectPt),
          count: units.length,
          first: units[0],
        },
      });
    }
  }
  return rows;
}

/** A row of the combat's "Itens": one potion or scroll, or the group of identical ones, with the mark of what using it costs. */
export interface CombatItemRowVm {
  readonly id: string;
  readonly key: string;
  readonly namePt: string;
  readonly kind: ConsumableKind;
  readonly icon: string;
  /** The line under the name: what the item does, in lower case (the group's "×N" is the row's `count`). */
  readonly detail: string;
  readonly count: number;
  /** The Ação, for what costs it in a combat (`costsAction`); none for what costs no action. */
  readonly economy: 'action' | null;
  /** The unit "Usar" sends: the first of the group. */
  readonly first: AttunementEntryVm;
}

/** The consumables of a character as the combat's "Itens" lists them: the same rows as the sheet, with their cost. */
export function combatItemRows(
  entries: readonly AttunementEntryVm[],
  categoryOf: (key: string) => string,
): CombatItemRowVm[] {
  return magicRows(entries, categoryOf).flatMap((r) => {
    if (r.type !== 'consumable') {
      return [];
    }
    const c = r.row;
    return [
      {
        id: c.id,
        key: c.key,
        namePt: c.namePt,
        kind: c.kind,
        icon: c.icon,
        detail: lowerFirst(c.effectLine),
        count: c.count,
        economy: costsAction(c.kind, c.key) ? ('action' as const) : null,
        first: c.first,
      },
    ];
  });
}

/** The dice of a healing line: how many, their faces and the number added; `text` is the formula as the line writes it. */
export interface HealingDice {
  readonly count: number;
  readonly sides: number;
  readonly modifier: number;
  readonly text: string;
}

const HEALING_DICE = /(\d+)d(\d+)(?:\s*([+−-])\s*(\d+))?/;

/** The dice of "Cura 2d4 + 2", or `null` for a line that is no healing. */
export function healingDice(effectPt: string): HealingDice | null {
  if (!effectPt.startsWith('Cura')) {
    return null;
  }
  const m = HEALING_DICE.exec(effectPt);
  if (!m) {
    return null;
  }
  const added = m[4] ? Number(m[4]) : 0;
  return {
    count: Number(m[1]),
    sides: Number(m[2]),
    modifier: m[3] && m[3] !== '+' ? -added : added,
    text: m[0],
  };
}

/**
 * The SRD's oils, by catalog key: the potions that are spent as text with no gate (the master judges the time they take).
 * The server names them by key too; this is the one list of them in the web.
 */
export const OILS: readonly string[] = [
  'item:oil-of-etherealness',
  'item:oil-of-sharpness',
  'item:oil-of-slipperiness',
];

/** Whether the catalog key is one of the SRD's oils. */
export function isOil(key: string): boolean {
  return OILS.includes(key);
}

/**
 * Whether using the item costs the turn and the action in a combat: every potion does, a text-only one included, except the
 * oils. A scroll and any other consumable costs no turn and no action.
 */
export function costsAction(kind: ConsumableKind, key: string): boolean {
  return kind === 'potion' && !isOil(key);
}

/** "resta 1" / "restam 2": how many units are left, in lower case for a line that goes on. */
export function leftWords(left: number): string {
  return left === 1 ? 'resta 1' : `restam ${left}`;
}

/**
 * The line under the title of the "Usar" sheet: the count and, in a combat, what the use costs. The master's title names
 * the character, so his line names the item first. `inCombat` is `null` where the page does not know whether there is a
 * combat (the sheet): the cost is then said for the player and left out for the master, whose sheet says it below.
 */
export function subtitleOf(input: {
  readonly itemName: string;
  readonly left: number;
  readonly actingFor: string | null;
  readonly inCombat: boolean | null;
  /** The use costs the action (`costsAction`): an item that does not says nothing of the cost. */
  readonly costsAction: boolean;
}): string {
  const costs =
    input.costsAction &&
    (input.inCombat === true || (input.inCombat === null && input.actingFor === null));
  const parts = input.actingFor
    ? [input.itemName, leftWords(input.left)]
    : [upFirst(leftWords(input.left))];
  if (costs) {
    parts.push('em combate, gasta a ação');
  }
  return parts.join(' · ');
}

/** What the page knows keeps a use from the Action: it is not the user's turn, or the Action of the turn is spent. */
export type TurnBlock = 'turn' | 'action';

/**
 * What the combat knows that keeps a potion from the Action: it is not the character's turn (the turn comes before the Action),
 * or the Action of the turn is spent. `null` while both are there. Only an item that costs the action asks (`costsAction`).
 */
export function turnBlockOf(input: {
  readonly myTurn: boolean;
  readonly actionUsed: boolean;
}): TurnBlock | null {
  if (!input.myTurn) {
    return 'turn';
  }
  return input.actionUsed ? 'action' : null;
}

/** A character the consumable may be given to, as the radio list reads. */
export interface ConsumableTargetVm {
  readonly id: string;
  readonly name: string;
  readonly down: boolean;
  /** "A 1,5 m · Caída" in a combat, empty out of one (the server tells how a character stands only in a running combat). */
  readonly line: string;
}

/** The receivers the server sent that may receive it now, with the words of their line. */
export function targetRows(targets: readonly ConsumableTarget[]): ConsumableTargetVm[] {
  return targets
    .filter((t) => t.mayReceive)
    .map((t) => {
      const down = t.state === ConsumableTargetState.DOWN;
      const parts = [
        t.distanceFt > 0 ? `A ${metersText(t.distanceFt)}` : '',
        down
          ? stateWord(CombatantState.DOWN, t.name)
          : t.state === ConsumableTargetState.OK
            ? 'De pé'
            : '',
      ].filter((p) => p !== '');
      return {
        id: t.characterId,
        name: t.name,
        down,
        line: parts.join(' · '),
      };
    });
}

/** What a use says on the sheet, in the live region under the magic items. */
export interface ConsumableNotice {
  readonly tone: 'good' | 'neutral';
  readonly icon: string;
  readonly headline: string;
  readonly detail: string;
  /** The item's text, for a consumable the app carries out no effect for. */
  readonly text: readonly string[];
}

export interface UseNoticeInput {
  readonly itemName: string;
  readonly effectPt: string;
  readonly userName: string;
  readonly receiverName: string;
  /** The potion went to another character. */
  readonly gift: boolean;
  /** The receiver was down: a heal gets them up. */
  readonly woke: boolean;
  /** The units of the item that are left on the sheet after this use. */
  readonly left: number;
}

function lowerFirst(text: string): string {
  return `${text.charAt(0).toLowerCase()}${text.slice(1)}`;
}

/**
 * How the sheet and the log speak of an item by its name: "a Poção de cura", "uma Poção de cura", "bebeu". The catalog carries no
 * gender and files the oils with the potions, so a potion is what the name says ("Poção…"): it is drunk, and anything else
 * (a scroll, an oil, a bead) is used, with the article its name asks for.
 */
export function itemWords(name: string): {
  readonly definite: string;
  readonly indefinite: string;
  readonly verb: 'bebeu' | 'usou';
} {
  const potion = /^poç(ão|ões)/i.test(name);
  const feminine = potion || article(name) === 'a';
  return {
    definite: `${feminine ? 'a' : 'o'} ${name}`,
    indefinite: `${feminine ? 'uma' : 'um'} ${name}`,
    verb: potion ? 'bebeu' : 'usou',
  };
}

/** "Resta 1 Poção de cura.", "Restam 3 unidades de Poção de cura." or "Era a última unidade.". */
function leftSentence(itemName: string, left: number): string {
  if (left <= 0) {
    return 'Era a última unidade.';
  }
  return left === 1 ? `Resta 1 ${itemName}.` : `Restam ${left} unidades de ${itemName}.`;
}

/** The roll written out: "3 + 2 + 2" for the faces and the number added (the total less the faces). */
export function rollSum(faces: readonly number[], total: number): string {
  const modifier = total - faces.reduce((a, n) => a + n, 0);
  const sum = faces.join(' + ');
  return modifier === 0 ? sum : `${sum} ${modifier < 0 ? '−' : '+'} ${Math.abs(modifier)}`;
}

/** What the server said of a use, as the sheet words it: the heal with its dice, the resistance, or the item's own text. */
export function useNotice(res: UseConsumableResponse, input: UseNoticeInput): ConsumableNotice {
  const { definite: the, verb } = itemWords(input.itemName);
  switch (res.outcome) {
    case ConsumableUseOutcome.HEALED: {
      const dice = healingDice(input.effectPt)?.text ?? res.diceText;
      return {
        tone: 'good',
        icon: 'favorite',
        headline:
          res.healed > 0
            ? `${input.receiverName} recebeu ${res.healed} PV${input.woke ? ' e acorda' : ''}.`
            : `${input.receiverName} já estava com os PV cheios: a poção não curou nada.`,
        detail: `${dice}: ${rollSum(res.faces, res.rollTotal)} = ${res.rollTotal}. ${leftSentence(input.itemName, input.left)}`,
        text: [],
      };
    }
    case ConsumableUseOutcome.RESISTANCE:
      return {
        tone: 'good',
        icon: 'check_circle',
        headline: `${input.receiverName} ${input.gift ? 'recebeu' : verb} ${the}: ${lowerFirst(effectLine(input.effectPt))}.`,
        detail: '',
        text: [],
      };
    default:
      return {
        tone: 'neutral',
        icon: 'menu_book',
        headline: input.gift
          ? `${input.userName} deu ${the} a ${input.receiverName}.`
          : `${input.userName} ${verb} ${the}.`,
        detail: 'O app não aplica este efeito: o mestre e você seguem o texto.',
        text: res.text,
      };
  }
}
