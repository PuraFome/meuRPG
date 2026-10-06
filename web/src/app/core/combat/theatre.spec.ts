import { CombatantKind, CombatantSide, CombatantState, CoverDegree, EncounterMode, OpportunityOfferSchema } from '../../../gen/meurpg/play/v1/combat_pb';
import { create } from '@bufbuild/protobuf';
import { combatant, encounter } from './combat-testing';
import {
  COVER_CHOICES,
  clampSpend,
  coverLine,
  coverTargets,
  everyoneStanding,
  isTheatre,
  reactorRows,
  restamText,
  spendLabel,
  spendPlan,
  stepSpend,
} from './theatre';

/** The app writes a number and its unit with a non-breaking space. */
const nb = (text: string) => text.replace(/ /g, '\u00a0');

const toren = combatant({ id: 't', label: 'Toren', kind: CombatantKind.PLAYER, side: CombatantSide.PARTY });
const g1 = combatant({ id: 'g1', label: 'Goblin 1' });
const cap = combatant({ id: 'cap', label: 'Capitão Goblin' });
const brisa = combatant({ id: 'b', label: 'Brisa', kind: CombatantKind.PLAYER, side: CombatantSide.PARTY });

describe('combat without a map', () => {
  it('branches on the encounter\'s mode', () => {
    expect(isTheatre(encounter({ mode: EncounterMode.THEATRE }))).toBe(true);
    expect(isTheatre(encounter({ mode: EncounterMode.GRID }))).toBe(false);
    // A combat the server did not tell the mode of is a map one.
    expect(isTheatre(encounter())).toBe(false);
    expect(isTheatre(null)).toBe(false);
  });

  describe('the amount of "Gastar movimento"', () => {
    const c = { speedDft: 300, movementLeftDft: 300, movementLeftFt: 30, movementUsedDft: 0 };

    it('steps from 1,5 m to 1,5 m and stops at what is left', () => {
      expect(clampSpend(5, 30)).toBe(5);
      expect(stepSpend(5, 1, 30)).toBe(10);
      expect(stepSpend(30, 1, 30)).toBe(30);
      expect(stepSpend(5, -1, 30)).toBe(5);
      // The last step may be shorter than 1,5 m: the server speaks whole feet and 7 ft is what is left.
      expect(stepSpend(5, 1, 7)).toBe(7);
    });

    it('is nothing when nothing is left, and never more than the server accepts', () => {
      expect(clampSpend(10, 0)).toBe(0);
      expect(stepSpend(5, 1, 0)).toBe(0);
      expect(clampSpend(900, 1000)).toBe(600);
    });

    it('says what is left after the amount, from the server\'s numbers (the artboard: 6,0 m of 9,0 m leaves 3,0 m)', () => {
      const plan = spendPlan(c, 20);
      expect(plan.amount).toBe(nb('6,0 m'));
      expect(plan.after).toBe(nb('3,0 m'));
      expect(plan.left).toBe(nb('9,0 m'));
      expect(plan.total).toBe(nb('9,0 m'));
      expect(plan.spent).toBe(nb('0,0 m'));
      expect(plan.canLess).toBe(true);
      expect(plan.canMore).toBe(true);
      expect(Math.round(plan.afterPercent)).toBe(33);
      expect(spendLabel(plan)).toBe(`Gastar ${nb('6,0 m')}`);
    });

    it('stops the plus at the limit and the minus at one step', () => {
      const all = spendPlan(c, 30);
      expect(all.canMore).toBe(false);
      expect(all.after).toBe(nb('0,0 m'));
      const one = spendPlan(c, 5);
      expect(one.canLess).toBe(false);
      expect(one.amount).toBe(nb('1,5 m'));
    });

    it('reads what is left after a turn that already spent some', () => {
      const plan = spendPlan({ speedDft: 300, movementLeftDft: 100, movementLeftFt: 10, movementUsedDft: 200 }, 5);
      expect(plan.spent).toBe(nb('6,0 m'));
      expect(plan.left).toBe(nb('3,0 m'));
      expect(plan.after).toBe(nb('1,5 m'));
      expect(restamText(100)).toBe(`Restam ${nb('3,0 m')}`);
      expect(restamText(0)).toBe('Sem movimento');
    });
  });

  describe('the master\'s offer', () => {
    it('lists the other side of whoever is on turn, standing, in the order the combat has them', () => {
      const e = encounter({
        mode: EncounterMode.THEATRE,
        currentCombatantId: 'g1',
        combatants: [brisa, g1, toren, cap, combatant({ id: 'x', label: 'Goblin 3', defeated: true })],
      });
      expect(reactorRows(e, (c) => (c.id === 't' ? 'Guerreiro 4' : '')).map((r) => [r.id, r.sub, r.player, r.spent, r.offered])).toEqual([
        ['b', '', true, false, false],
        ['t', 'Guerreiro 4', true, false, false],
      ]);
    });

    it('says a spent reaction instead of hiding the row, and an ally NPC counts with the party', () => {
      const ally = combatant({ id: 'al', label: 'Aliado', side: CombatantSide.PARTY });
      const e = encounter({
        currentCombatantId: 'g1',
        combatants: [g1, combatant({ id: 't', label: 'Toren', kind: CombatantKind.PLAYER, side: CombatantSide.PARTY, reactionUsed: true }), ally],
      });
      const rows = reactorRows(e, () => '');
      expect(rows.map((r) => [r.id, r.spent])).toEqual([['t', true], ['al', false]]);
    });

    it('leaves out a character at 0 hit points (it cannot react) and marks the reactor that already has an offer from this mover', () => {
      const down = combatant({ id: 'd', label: 'Caído', kind: CombatantKind.PLAYER, side: CombatantSide.PARTY, state: CombatantState.DOWN });
      const offer = create(OpportunityOfferSchema, { id: 'o', moverId: 'g1', reactorId: 't' });
      const e = encounter({ currentCombatantId: 'g1', combatants: [g1, toren, down], opportunityOffers: [offer] });
      expect(reactorRows(e, () => '').map((r) => [r.id, r.offered])).toEqual([['t', true]]);
    });

    it('is nothing in the master\'s turn', () => {
      expect(reactorRows(encounter({ currentCombatantId: '', combatants: [g1, toren] }), () => '')).toEqual([]);
    });
  });

  describe('cover', () => {
    it('has four rows and says each degree', () => {
      expect(COVER_CHOICES.map((c) => c.name)).toEqual(['Sem cobertura', 'Meia cobertura', 'Três quartos', 'Total (não dá para mirar)']);
      expect(COVER_CHOICES.map((c) => c.sub)).toEqual(['', '+2 na CA e em Destreza', '+5 na CA e em Destreza', '']);
      expect(coverLine(CoverDegree.NONE)).toBe('Sem cobertura');
      expect(coverLine(CoverDegree.HALF)).toBe('Meia cobertura: +2 na CA');
      expect(coverLine(CoverDegree.THREE_QUARTERS)).toBe('Três quartos: +5 na CA');
      expect(coverLine(CoverDegree.TOTAL)).toBe('Total: não dá para mirar');
    });

    it('lists everyone standing when nobody is the single mover (the master\'s turn), for the cover opened from the order\'s menu', () => {
      const e = encounter({ currentCombatantId: '', combatants: [toren, g1, combatant({ id: 'x', label: 'Goblin 3', defeated: true })] });
      expect(coverTargets(e).map((c) => c.label)).toEqual(['Toren', 'Goblin 1']);
      expect(everyoneStanding(e).map((c) => c.label)).toEqual(['Toren', 'Goblin 1']);
    });

    it('marks the targets of whoever is on turn: the other side, standing', () => {
      const e = encounter({ currentCombatantId: 't', combatants: [toren, g1, cap, brisa] });
      expect(coverTargets(e).map((c) => c.label)).toEqual(['Goblin 1', 'Capitão Goblin']);
    });
  });
});
