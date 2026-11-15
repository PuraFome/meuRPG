import {
  type CombatLogEntry,
  CombatLogKind,
  ItemUseOutcome,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { article } from '../format/article';
import type { LogLine } from './combat-log-types';

/** What a builder of these lines is given: the entry and the fields every line shares. */
interface ItemUseArgs {
  readonly e: CombatLogEntry;
  readonly base: { id: string; actor: string; hidden: boolean; undoable: boolean };
}

/** The plural of the kinds of ammunition the server names ("Flecha", "Projétil de funda": the catalog's singular names). */
const MANY: Readonly<Record<string, string>> = {
  flecha: 'flechas',
  'virote de besta': 'virotes de besta',
  'projétil de funda': 'projéteis de funda',
  'agulha de zarabatana': 'agulhas de zarabatana',
};

const cargas = (n: number) => `${n} ${n === 1 ? 'carga' : 'cargas'}`;

/** "da Varinha de teia", "do Bordão do poder". */
const of = (name: string) => `${article(name) === 'a' ? 'da' : 'do'} ${name}`;

/** "a Varinha de teia". */
const the = (name: string) => `${article(name)} ${name}`;

/** "ITEM_CHARGE_SPENT": "gastou a última carga da Varinha de teia", or how many went and how many are left. */
function chargeSpent({ e, base }: ItemUseArgs): LogLine | null {
  const use = e.itemUse;
  if (!use) {
    return null;
  }
  const last = `gastou a última carga ${of(use.itemNamePt)}`;
  const text =
    use.outcome === ItemUseOutcome.LAST_CHARGE
      ? last
      : use.outcome === ItemUseOutcome.LOST
        ? `${last}: ${the(use.itemNamePt)} se perdeu`
        : `gastou ${cargas(use.amount)} ${of(use.itemNamePt)}: ${use.left === 1 ? 'resta' : 'restam'} ${use.left} de ${use.max}`;
  return { ...base, icon: 'bolt', text: ` ${text}` };
}

/** "ITEM_LAST_CHARGE_ROLLED": "rolou 1 no d20 da última carga: a Varinha de teia virou cinzas". */
function lastChargeRolled({ e, base }: ItemUseArgs): LogLine | null {
  const use = e.itemUse;
  if (!use) {
    return null;
  }
  const item = the(use.itemNamePt);
  const result =
    use.outcome === ItemUseOutcome.LOST
      ? `${item} virou cinzas`
      : use.outcome === ItemUseOutcome.REGAINED
        ? `${item} recuperou ${cargas(use.amount)}`
        : `${item} fica, sem cargas`;
  return { ...base, icon: 'casino', text: ` rolou ${use.d20} no d20 da última carga: ${result}` };
}

/** "AMMUNITION_SPENT": "atirou com o Arco longo: 1 flecha, restam 3". */
function ammunitionSpent({ e, base }: ItemUseArgs): LogLine | null {
  const use = e.itemUse;
  if (!use) {
    return null;
  }
  const one = use.ammunitionNamePt.toLowerCase();
  const piece = use.amount === 1 ? one : (MANY[one] ?? one);
  return {
    ...base,
    icon: 'north_east',
    text: ` atirou com ${the(use.itemNamePt)}: ${use.amount} ${piece}, ${use.left === 1 ? 'resta' : 'restam'} ${use.left}`,
  };
}

/** The lines of the charges of a magic item and of the ammunition of a ranged attack: the master's and the owner's (RN-10). */
export const ITEM_USE_LINES: readonly (readonly [
  CombatLogKind,
  (args: ItemUseArgs) => LogLine | null,
])[] = [
  [CombatLogKind.ITEM_CHARGE_SPENT, chargeSpent],
  [CombatLogKind.ITEM_LAST_CHARGE_ROLLED, lastChargeRolled],
  [CombatLogKind.AMMUNITION_SPENT, ammunitionSpent],
];
