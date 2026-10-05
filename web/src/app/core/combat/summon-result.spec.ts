import { CombatantKind } from '../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from './combat-testing';
import { summonResult } from './summon-result';

describe('what the player reads after Conjurar Animais (E9-12 state 2)', () => {
  const wolf = (n: number) => combatant({ id: `w${n}`, label: `Lobo atroz ${n}`, kind: CombatantKind.CREATURE, controlledByMe: true, monsterKey: 'k', monsterNamePt: 'Lobo atroz', initiative: 10, initiativeFace: 8, initiativeBonus: 2 });
  const order = [combatant({ id: 'g', label: 'Goblin 1', initiative: 12 }), combatant({ id: 'g2', label: 'Goblin 2', initiative: 12 }), wolf(1), wolf(2)];

  it('says the group\'s total, the one d20 and who they act after', () => {
    const e = encounter({ combatants: order, npcOnlyGroups: [{ combatantIds: ['g', 'g2'] }] as never });
    const r = summonResult(e, ['w1', 'w2']);
    expect(r.text.replace(/ /g, ' ')).toBe('Os 2 Lobos atrozes entram no combate com iniciativa 10 (um d20 para os dois: 8 + 2). Eles agem juntos, depois dos Goblins.');
    expect(r.creatures.length).toBe(2);
  });

  it('is empty when the combat does not have them', () => {
    expect(summonResult(encounter({ combatants: [] }), ['x']).text).toBe('');
  });
});
