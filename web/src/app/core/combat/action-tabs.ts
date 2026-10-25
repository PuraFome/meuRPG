import {
  ActionEconomy,
  type ActionOption,
  type TurnOptions,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { type EconomyGroup, groupOf } from './combat-options';

/** The tabs of "O que você pode fazer": what the character attacks with, casts, its class's abilities, and the actions everyone has. */
export type ActionTabId = 'attacks' | 'spells' | 'class' | 'other';

/** The name of the tab that holds a class's abilities, by the feature that grants them; the class itself is not in the turn options. */
const CLASS_TAB_BY_FEATURE: readonly (readonly [prefix: string, tab: string])[] = [
  ['feature:rage', 'Bárbaro'],
  ['feature:reckless-attack', 'Bárbaro'],
  ['feature:bardic-inspiration', 'Bardo'],
  ['feature:cutting-words', 'Bardo'],
  ['feature:channel-divinity-sacred-weapon', 'Paladino'],
  ['feature:channel-divinity-turn-the-unholy', 'Paladino'],
  ['feature:channel-divinity', 'Clérigo'],
  ['feature:wild-shape', 'Druida'],
  ['feature:second-wind', 'Guerreiro'],
  ['feature:action-surge', 'Guerreiro'],
  ['feature:fighter-fighting-style', 'Guerreiro'],
  ['feature:flurry-of-blows', 'Chi'],
  ['feature:patient-defense', 'Chi'],
  ['feature:step-of-the-wind', 'Chi'],
  ['feature:deflect-missiles', 'Chi'],
  ['feature:slow-fall', 'Chi'],
  ['feature:divine-sense', 'Paladino'],
  ['feature:lay-on-hands', 'Paladino'],
  ['feature:fighting-style-protection', 'Paladino'],
  ['feature:hunters-prey', 'Patrulheiro'],
  ['feature:cunning-action', 'Ladino'],
  ['feature:fast-hands', 'Ladino'],
  ['feature:uncanny-dodge', 'Ladino'],
  ['feature:flexible-casting', 'Feiticeiro'],
];

/** What a tab with abilities of no class in the list (a race's breath, a feat) is called. */
export const OTHER_FEATURES_TAB = 'Habilidades';

/**
 * The name of the tab of the class's abilities: the class the features belong to ("Guerreiro", "Ladino"), "Chi" for the Monk
 * (its abilities spend ki), and "Habilidades" when they come from more than one class or from none the table knows.
 */
export function classTabName(features: readonly ActionOption[]): string {
  const names = new Set(
    features.map(
      (f) =>
        CLASS_TAB_BY_FEATURE.find(([prefix]) => (f.action?.key ?? '').startsWith(prefix))?.[1] ??
        OTHER_FEATURES_TAB,
    ),
  );
  return names.size === 1 ? [...names][0] : OTHER_FEATURES_TAB;
}

/** The economy a feature spends; one that costs nothing (Surto de Ação) is never "used up" with the Action, so it has none. */
export function featureGroup(a: ActionOption): EconomyGroup | null {
  return a.action?.economy === ActionEconomy.FREE ? null : groupOf(a.action?.economy ?? 0);
}

/** What has been spent this turn, by economy. */
export type SpentGroups = Readonly<Record<EconomyGroup, boolean>>;

const GROUP_NAME: Readonly<Record<EconomyGroup, string>> = {
  action: 'Ação',
  bonus: 'Ação bônus',
  reaction: 'Reação',
};
const GROUP_ORDER: readonly EconomyGroup[] = ['action', 'bonus', 'reaction'];

/** The rows of one economy that is spent, folded to one line ("Ação bônus · Usada"). */
export interface FoldedGroup<T> {
  readonly group: EconomyGroup;
  readonly title: string;
  readonly rows: readonly T[];
}

/** The rows still in play, and the groups that are spent folded behind a line each. */
export interface GroupParts<T> {
  readonly open: readonly T[];
  readonly folded: readonly FoldedGroup<T>[];
}

/**
 * Splits rows by the economy they spend: what is still there stays in the list (in the order it came), and a group that is spent
 * (its Action, Bonus action or Reaction used, or none usable) becomes one folded line named for it, "Ação bônus · Usada" (or
 * `Indisponível` for one who cannot act). A row with no economy is always open.
 */
export function partByEconomy<T>(
  rows: readonly T[],
  groupFor: (row: T) => EconomyGroup | null,
  spent: SpentGroups,
  word: string,
): GroupParts<T> {
  const open: T[] = [];
  const folded = new Map<EconomyGroup, T[]>();
  for (const row of rows) {
    const group = groupFor(row);
    if (group && spent[group]) {
      folded.set(group, [...(folded.get(group) ?? []), row]);
    } else {
      open.push(row);
    }
  }
  return {
    open,
    folded: GROUP_ORDER.filter((g) => folded.has(g)).map((g) => ({
      group: g,
      title: `${GROUP_NAME[g]} · ${word}`,
      rows: folded.get(g) ?? [],
    })),
  };
}

/** How many of each tab, from the turn's options. */
export interface ActionCounts {
  readonly attacks: number;
  readonly spells: number;
  readonly features: number;
  readonly standard: number;
}

/** One tab of the list: its id, label, count and the count in words for the tab's accessible name. */
export interface ActionTabItem {
  readonly id: ActionTabId;
  readonly label: string;
  readonly badge: string;
  readonly words: string;
}

const plural = (n: number, one: string, many: string): string => `${n} ${n === 1 ? one : many}`;

/**
 * The tabs to draw: "Ataques" and "Outras" always (when there is anything to attack with), "Magias" for a caster, and the class's
 * tab when it grants actions (or the character is a beast, whose way back is there).
 */
export function actionTabs(
  counts: ActionCounts,
  className: string,
  options: { readonly beast?: boolean } = {},
): ActionTabItem[] {
  const tabs: ActionTabItem[] = [];
  if (counts.attacks > 0) {
    tabs.push({
      id: 'attacks',
      label: 'Ataques',
      badge: String(counts.attacks),
      words: plural(counts.attacks, 'ataque', 'ataques'),
    });
  }
  if (counts.spells > 0) {
    tabs.push({
      id: 'spells',
      label: 'Magias',
      badge: String(counts.spells),
      words: plural(counts.spells, 'magia', 'magias'),
    });
  } else if (options.beast) {
    // A druid in a beast's shape casts nothing, and the tab says so instead of vanishing.
    tabs.push({ id: 'spells', label: 'Magias', badge: '', words: '' });
  }
  if (counts.features > 0 || options.beast) {
    tabs.push({
      id: 'class',
      label: className,
      badge: String(counts.features),
      words: plural(counts.features, 'habilidade', 'habilidades'),
    });
  }
  tabs.push({
    id: 'other',
    label: 'Outras',
    badge: String(counts.standard),
    words: plural(counts.standard, 'ação', 'ações'),
  });
  return tabs;
}

/** The counts of a turn's options; the contest attacks (Agarrar, Empurrar) and "Escapar" are attacks too. */
export function actionCounts(
  options: TurnOptions,
  extra: { readonly contestAttacks: number; readonly escape: boolean; readonly standard: number },
): ActionCounts {
  return {
    attacks: options.attacks.length + extra.contestAttacks + (extra.escape ? 1 : 0),
    spells: options.spells.length,
    features: options.featureActions.length,
    standard: extra.standard,
  };
}
