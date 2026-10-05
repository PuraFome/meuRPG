import { CombatantKind } from '../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from './combat-testing';
import { CHARACTER_TAB, actingTab, membersToEnd, mineTabs, tabWord, validTab } from './mine';

const salvia = combatant({ id: 's', label: 'Sálvia', kind: CombatantKind.PLAYER, mine: true, controlledByMe: true, initiative: 13 });
const wolf = (n: number, over = {}) =>
  combatant({ id: `w${n}`, label: `Lobo atroz ${n}`, kind: CombatantKind.CREATURE, controlledByMe: true, ownerCharacterId: 'char-s', summonGroupId: 'cast-1', monsterKey: 'monster:dire-wolf', monsterNamePt: 'Lobo atroz', initiative: 10, ...over });
const goblin = combatant({ id: 'g', label: 'Goblin 1' });

describe('the tabs of what a player plays', () => {
  it('has one tab (so no bar) for a player with one combatant', () => {
    expect(mineTabs(encounter({ combatants: [salvia, goblin] })).length).toBe(1);
  });

  it('has the character first and one tab for the creatures of a casting', () => {
    const e = encounter({ combatants: [salvia, goblin, wolf(1), wolf(2)], currentCombatantId: 'w1', turnGroupIds: ['w1', 'w2'] });
    const tabs = mineTabs(e);
    expect(tabs.map((t) => t.label)).toEqual(['Sálvia', 'Lobos atrozes (2)']);
    expect(tabs[0].id).toBe(CHARACTER_TAB);
    expect(tabs.map((t) => t.state)).toEqual(['done', 'turn']);
    expect(tabWord(tabs[0])).toBe('Já agiu · 13');
    expect(tabWord(tabs[1])).toBe('Sua vez · 10');
    expect(tabWord(tabs[1], true)).toBe('Sua vez');
  });

  it('says who still waits, with the initiative they act on', () => {
    const e = encounter({ combatants: [salvia, goblin, wolf(1), wolf(2)], currentCombatantId: 's', turnGroupIds: ['s'] });
    expect(mineTabs(e).map((t) => tabWord(t))).toEqual(['Sua vez · 13', 'Espera · vez 10']);
  });

  it('keeps a familiar as a tab of its own', () => {
    const nanquim = combatant({ id: 'n', label: 'Nanquim', kind: CombatantKind.CREATURE, controlledByMe: true, initiative: 8 });
    const tabs = mineTabs(encounter({ combatants: [salvia, nanquim], currentCombatantId: 'n', turnGroupIds: ['n'] }));
    expect(tabs.map((t) => t.label)).toEqual(['Sálvia', 'Nanquim']);
    expect(tabs[1].state).toBe('turn');
  });
});

describe('following the turn and ending a part', () => {
  const joint = encounter({ combatants: [salvia, goblin, wolf(1), wolf(2)], currentCombatantId: 'w1', turnGroupIds: ['w1', 'w2'] });

  it('the page follows the turn to the tab that acts, and goes back to the character when the creatures are gone', () => {
    expect(actingTab(mineTabs(joint))).toBe('cast-1');
    expect(actingTab(mineTabs(encounter({ combatants: [salvia, goblin, wolf(1), wolf(2)], currentCombatantId: 'g', turnGroupIds: ['g'] })))).toBe('');
    expect(validTab(mineTabs(joint), 'cast-1')).toBe('cast-1');
    // The wolves left (concentration lost): the tab is not there any more.
    expect(validTab(mineTabs(encounter({ combatants: [salvia, goblin], currentCombatantId: 's' })), 'cast-1')).toBe(CHARACTER_TAB);
  });

  it('ends the part of each member that still acts, and never one that ended already', () => {
    const tab = mineTabs(joint).find((t) => t.id === 'cast-1')!;
    expect(membersToEnd(joint, tab).map((m) => m.id)).toEqual(['w1', 'w2']);
    const one = encounter({ combatants: [salvia, goblin, wolf(1, { turnPartEnded: true }), wolf(2)], currentCombatantId: 'w1', turnGroupIds: ['w1', 'w2'] });
    expect(membersToEnd(one, tab).map((m) => m.id)).toEqual(['w2']);
  });
});
