import { type Combatant, type Encounter } from '../../../gen/meurpg/play/v1/combat_pb';
import { groupName, isCreature } from './creature-names';
import { actingIds, turnMembers } from './joint-turn';

/**
 * Everything a player moves in a combat (MR-037, E9-12): their own character and their creatures
 * (a familiar, summoned animals). The server says which are theirs (`controlled_by_me`); this
 * only gathers them into the tabs of the turn bar, one for the character and one for each
 * casting's creatures (a joint turn: they roll one initiative and act together) or for a creature
 * alone. Pure functions, tested without a DOM.
 */

/** The tab of the character itself. */
export const CHARACTER_TAB = 'character';

/** Where a tab stands in the order: its turn now, already played this round, or still to come. */
export type TabState = 'turn' | 'done' | 'wait';

export interface MineTab {
  /** `CHARACTER_TAB`, or the id of the creatures' casting (their `summon_group_id`) or of the creature alone. */
  readonly id: string;
  /** The combatants behind it: the character, or the creatures of one group, in the order. */
  readonly members: readonly Combatant[];
  readonly creature: boolean;
  /** "Sálvia", "Lobos atrozes (2)", "Nanquim". */
  readonly label: string;
  readonly state: TabState;
  /** The initiative total, when the server sent it. */
  readonly initiative: number | undefined;
  /** The members that may act now. */
  readonly acting: readonly Combatant[];
}

/** The combatants the caller plays: the character and its creatures. Always empty for the master. */
export function controlled(e: Encounter): Combatant[] {
  return e.combatants.filter((c) => c.controlledByMe || c.mine);
}

/** The creatures the caller plays. */
export function ownCreatures(e: Encounter): Combatant[] {
  return e.combatants.filter((c) => isCreature(c) && c.controlledByMe);
}

/** "Lobos atrozes (2)" for a group of several, the creature's own name for one. */
function tabLabel(members: readonly Combatant[]): string {
  return members.length > 1 ? `${groupName(members)} (${members.length})` : members[0].label;
}

/** The tabs of what the player plays: the character first, then each group in the order. The turn bar shows them only when there is more than one. */
export function mineTabs(e: Encounter): MineTab[] {
  const all = controlled(e);
  if (all.length === 0) {
    return [];
  }
  const acting = new Set(actingIds(e));
  const turnAt = Math.min(...turnMembers(e).map((m) => e.combatants.indexOf(m)).filter((i) => i >= 0));
  const groups = new Map<string, Combatant[]>();
  for (const c of all) {
    const key = c.mine ? CHARACTER_TAB : c.summonGroupId || c.id;
    groups.set(key, [...(groups.get(key) ?? []), c]);
  }
  const tabs = [...groups].map(([id, members]) => {
    const live = members.filter((m) => acting.has(m.id));
    const at = Math.min(...members.map((m) => e.combatants.indexOf(m)));
    const state: TabState = live.length > 0 ? 'turn' : Number.isFinite(turnAt) && at < turnAt ? 'done' : turnMembers(e).some((m) => members.includes(m)) ? 'done' : 'wait';
    return {
      id,
      members,
      creature: id !== CHARACTER_TAB,
      label: id === CHARACTER_TAB ? members[0].label : tabLabel(members),
      state,
      initiative: members[0].initiative,
      acting: live,
    } satisfies MineTab;
  });
  // The character first, then the creatures in the order they act.
  return tabs.sort((a, b) => (a.id === CHARACTER_TAB ? -1 : b.id === CHARACTER_TAB ? 1 : 0));
}

/** The tab's word: "Sua vez · 10", "Já agiu · 13", "Espera · vez 10"; without the number when `compact` (a 320 px phone). */
export function tabWord(tab: MineTab, compact = false): string {
  const n = tab.initiative;
  switch (tab.state) {
    case 'turn':
      return compact || n === undefined ? 'Sua vez' : `Sua vez · ${n}`;
    case 'done':
      return compact || n === undefined ? 'Já agiu' : `Já agiu · ${n}`;
    default:
      return compact || n === undefined ? 'Espera' : `Espera · vez ${n}`;
  }
}
