import { CombatantKind, CombatantState } from '../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from './combat-testing';
import {
  combatantInitial,
  ownCombatant,
  initiativeFormula,
  moveInGroup,
  nextCombatant,
  pendingFormula,
  stateWord,
  tieGroups,
  tieSentence,
  turnBanner,
} from './combat-view';

describe('combat view helpers', () => {
  it('finds the player\'s own combatant by `mine`, never a creature of theirs (MR-037)', () => {
    // The server marks a character's creature `controlledByMe`, never `mine`: a
    // combat with a familiar must not make the screen pick the owl as "me".
    const e = encounter({
      combatants: [
        combatant({ id: 'nanquim', label: 'Nanquim', kind: CombatantKind.CREATURE, mine: false, controlledByMe: true, ownerCharacterId: 'pens' }),
        combatant({ id: 'pens', label: 'Pensantus', kind: CombatantKind.PLAYER, mine: true, controlledByMe: true }),
        combatant({ id: 'toren', label: 'Toren', kind: CombatantKind.PLAYER }),
      ],
    });
    expect(ownCombatant(e)?.label).toBe('Pensantus');
    // A player whose character is not in the fight but whose creature is has no "own" combatant.
    expect(ownCombatant(encounter({ combatants: [e.combatants[0]!, e.combatants[2]!] }))).toBeNull();
  });

  it('says how hurt an NPC is in words, never numbers (RN-20)', () => {
    expect(stateWord(CombatantState.UNHURT)).toBe('Ileso');
    expect(stateWord(CombatantState.HURT)).toBe('Ferido');
    expect(stateWord(CombatantState.BADLY_HURT)).toBe('Muito ferido');
    expect(stateWord(CombatantState.DEFEATED)).toBe('Derrotado');
  });

  it('puts the number of a numbered copy on its token', () => {
    expect(combatantInitial('Goblin 2')).toBe('G2');
    expect(combatantInitial('Capitão Goblin')).toBe('C');
    expect(combatantInitial('pensantus')).toBe('P');
  });

  it('writes the initiative as the artboard does', () => {
    const c = combatant({ id: 'a', label: 'Brisa', initiative: 19, initiativeFace: 15, initiativeBonus: 4 });
    expect(initiativeFormula(c)).toBe('1d20 (15) + 4 = 19');
    const low = combatant({ id: 'b', label: 'Toren', initiative: 14, initiativeFace: 15, initiativeBonus: -1 });
    expect(initiativeFormula(low)).toBe('1d20 (15) − 1 = 14');
    const waiting = combatant({ id: 'c', label: 'Toren', initiativeBonus: 2 });
    expect(initiativeFormula(waiting)).toBeNull();
    expect(pendingFormula(waiting)).toBe('1d20 + 2 = —');
  });

  describe('ties', () => {
    const list = [
      combatant({ id: 'brisa', label: 'Brisa', initiative: 19, initiativeBonus: 4 }),
      combatant({ id: 'g1', label: 'Goblin 1', initiative: 12, initiativeBonus: 2, tieUnresolved: true }),
      combatant({ id: 'g2', label: 'Goblin 2', initiative: 12, initiativeBonus: 2, tieUnresolved: true }),
      combatant({ id: 'g3', label: 'Goblin 3', initiative: 9, initiativeBonus: 2 }),
    ];

    it('groups the unordered ties that follow each other', () => {
      expect(tieGroups(list)).toEqual([['g1', 'g2']]);
      expect(tieGroups(list.slice(0, 1))).toEqual([]);
    });

    it('keeps two ties on different totals apart', () => {
      const two = [
        combatant({ id: 'a', label: 'A', initiative: 12, initiativeBonus: 1, tieUnresolved: true }),
        combatant({ id: 'b', label: 'B', initiative: 12, initiativeBonus: 1, tieUnresolved: true }),
        combatant({ id: 'c', label: 'C', initiative: 10, initiativeBonus: 0, tieUnresolved: true }),
        combatant({ id: 'd', label: 'D', initiative: 10, initiativeBonus: 0, tieUnresolved: true }),
      ];
      expect(tieGroups(two)).toEqual([['a', 'b'], ['c', 'd']]);
    });

    it('moves a combatant inside its group only: the outer arrows do nothing', () => {
      expect(moveInGroup(['g1', 'g2'], 'g1', 1)).toEqual(['g2', 'g1']);
      expect(moveInGroup(['g1', 'g2'], 'g1', -1)).toEqual(['g1', 'g2']);
      expect(moveInGroup(['g1', 'g2'], 'g2', 1)).toEqual(['g1', 'g2']);
      expect(moveInGroup(['a', 'b', 'c'], 'b', -1)).toEqual(['b', 'a', 'c']);
    });

    it('says the tie as the artboard does', () => {
      expect(tieSentence(['Goblin 1', 'Goblin 2'], 12)).toBe(
        'Empate em 12: Goblin 1 e Goblin 2. Escolha a ordem com as setas.',
      );
      expect(tieSentence(['A', 'B', 'C'], 7)).toBe('Empate em 7: A, B e C. Escolha a ordem com as setas.');
    });
  });

  describe('the turn banner', () => {
    const order = [
      combatant({ id: 'brisa', label: 'Brisa', kind: CombatantKind.PLAYER }),
      combatant({ id: 'cap', label: 'Capitão Goblin' }),
      combatant({ id: 'pen', label: 'Pensantus', kind: CombatantKind.PLAYER, mine: true }),
      combatant({ id: 'g1', label: 'Goblin 1', defeated: true }),
    ];

    it('names the one on turn, and who is next when it is the player', () => {
      const banner = turnBanner(encounter({ combatants: order, currentCombatantId: 'cap' }));
      expect(banner.title).toBe('Vez do Capitão Goblin');
      expect(banner.nextIsMine).toBe(true);
      expect(banner.next?.label).toBe('Pensantus');
    });

    it('says "Sua vez" on the player\'s own turn', () => {
      const banner = turnBanner(encounter({ combatants: order, currentCombatantId: 'pen' }));
      expect(banner.title).toBe('Sua vez, Pensantus');
      expect(banner.mine).toBe(true);
    });

    it('skips a defeated combatant and goes round the order', () => {
      const e = encounter({ combatants: order, currentCombatantId: 'pen' });
      expect(nextCombatant(e)?.label).toBe('Brisa');
    });

    it('says "Vez do mestre" for a hidden turn, with no name and no next', () => {
      const banner = turnBanner(encounter({ combatants: order, currentCombatantId: '', masterTurn: true }));
      expect(banner.title).toBe('Vez do mestre');
      expect(banner.who).toBeNull();
      expect(banner.next).toBeNull();
      expect(banner.nextIsMine).toBe(false);
    });

    it('has no next when it is the only one standing', () => {
      const alone = encounter({ combatants: [order[0]], currentCombatantId: 'brisa' });
      expect(nextCombatant(alone)).toBeNull();
    });
  });
});
