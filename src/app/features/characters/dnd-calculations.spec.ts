import {
  abilityModifier,
  computedSkillBonus,
  effectiveSkillBonus,
  formatModifier,
  passivePerception,
  proficiencyBonusForLevel,
  proficiencyContribution,
  savingThrowBonus,
  spellAttackBonus,
  spellSaveDc,
} from './dnd-calculations';
import type { SkillEntry } from '../../core/models/character';

describe('dnd-calculations', () => {
  describe('abilityModifier', () => {
    it('follows floor((score - 10) / 2)', () => {
      expect(abilityModifier(1)).toBe(-5);
      expect(abilityModifier(8)).toBe(-1);
      expect(abilityModifier(10)).toBe(0);
      expect(abilityModifier(11)).toBe(0);
      expect(abilityModifier(14)).toBe(2);
      expect(abilityModifier(20)).toBe(5);
      expect(abilityModifier(30)).toBe(10);
    });

    it('defaults non-finite input to a 10 score', () => {
      expect(abilityModifier(Number.NaN)).toBe(0);
    });
  });

  describe('proficiencyBonusForLevel', () => {
    it('scales in the standard 5e steps', () => {
      expect(proficiencyBonusForLevel(1)).toBe(2);
      expect(proficiencyBonusForLevel(4)).toBe(2);
      expect(proficiencyBonusForLevel(5)).toBe(3);
      expect(proficiencyBonusForLevel(9)).toBe(4);
      expect(proficiencyBonusForLevel(13)).toBe(5);
      expect(proficiencyBonusForLevel(17)).toBe(6);
      expect(proficiencyBonusForLevel(20)).toBe(6);
    });

    it('clamps out-of-range levels', () => {
      expect(proficiencyBonusForLevel(0)).toBe(2);
      expect(proficiencyBonusForLevel(99)).toBe(6);
    });
  });

  describe('proficiencyContribution', () => {
    it('double-counts expertise', () => {
      expect(proficiencyContribution(false, false, 3)).toBe(0);
      expect(proficiencyContribution(true, false, 3)).toBe(3);
      expect(proficiencyContribution(false, true, 3)).toBe(6);
    });
  });

  describe('skill bonuses', () => {
    const attributes = { for: 14, des: 16, con: 12, int: 10, sab: 16, car: 11 };

    it('computes ability + proficiency, doubled on expertise', () => {
      const proficient: SkillEntry = {
        name: 'Atletismo',
        ability: 'for',
        proficient: true,
        expertise: false,
      };
      const expert: SkillEntry = { ...proficient, expertise: true };
      expect(computedSkillBonus(proficient, attributes, 2)).toBe(4); // +2 mod + 2
      expect(computedSkillBonus(expert, attributes, 2)).toBe(6); // +2 mod + 4
    });

    it('honours a manual override', () => {
      const override: SkillEntry = {
        name: 'Atletismo',
        ability: 'for',
        proficient: false,
        expertise: false,
        bonusOverride: 9,
      };
      expect(effectiveSkillBonus(override, attributes, 2)).toBe(9);
    });
  });

  describe('savingThrowBonus', () => {
    it('adds proficiency only when trained', () => {
      const attributes = { des: 16 };
      expect(savingThrowBonus('des', false, attributes, 3)).toBe(3);
      expect(savingThrowBonus('des', true, attributes, 3)).toBe(6);
    });
  });

  describe('passivePerception', () => {
    it('is 10 + the Perception bonus', () => {
      const attributes = { sab: 16 };
      const perception: SkillEntry = {
        name: 'Percepção',
        ability: 'sab',
        proficient: true,
        expertise: false,
      };
      expect(passivePerception(attributes, 2, perception)).toBe(15); // 10 + 3 + 2
    });

    it('defaults to a non-proficient check', () => {
      expect(passivePerception({ sab: 12 }, 2, null)).toBe(11); // 10 + 1
    });
  });

  describe('spellcasting', () => {
    it('computes save DC and attack bonus from the ability modifier', () => {
      expect(spellSaveDc(3, 4)).toBe(15); // 8 + 4 + 3
      expect(spellAttackBonus(3, 4)).toBe(7); // 4 + 3
    });
  });

  describe('formatModifier', () => {
    it('always shows an explicit sign', () => {
      expect(formatModifier(2)).toBe('+2');
      expect(formatModifier(0)).toBe('+0');
      expect(formatModifier(-1)).toBe('-1');
    });
  });
});