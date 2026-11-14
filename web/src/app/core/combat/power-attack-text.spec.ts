import type { CombatLogPowerAttack } from '../../../gen/meurpg/play/v1/combat_pb';
import { powerAttackNote, powerAttackWords } from './power-attack-text';

const marker = (p: Partial<CombatLogPowerAttack>) => p as CombatLogPowerAttack;

describe('powerAttackWords', () => {
  it('writes the numbers of the master and the attacker with the app signs and a true minus', () => {
    const words = powerAttackWords(
      marker({ key: 'feat:golpe@mesa', namePt: 'Golpe Pesado', penalty: 5, damageBonus: 10 }),
    );
    expect(words).toBe('ataque poderoso (−5 no ataque, +10 no dano)');
    expect(words).not.toContain('-');
  });

  it('says only that a trade was made to another player', () => {
    expect(powerAttackWords(marker({ key: '', namePt: 'Golpe Pesado', penalty: 0 }))).toBe(
      'usou um ataque poderoso',
    );
  });

  it('says the same to another player when the master hid the feature, and never "Outra fonte"', () => {
    const words = powerAttackWords(marker({ key: '', namePt: 'Outra fonte', hidden: true }));
    expect(words).toBe('usou um ataque poderoso');
    expect(words).not.toContain('Outra fonte');
  });

  it('says nothing of an attack that made no trade', () => {
    expect(powerAttackWords(undefined)).toBe('');
    expect(powerAttackNote(undefined)).toBe('');
  });
});

describe('powerAttackNote', () => {
  it('joins the words to the attack sentence', () => {
    expect(powerAttackNote(marker({ key: 'feat:golpe@mesa', penalty: 5, damageBonus: 10 }))).toBe(
      ', com ataque poderoso (−5 no ataque, +10 no dano)',
    );
    expect(powerAttackNote(marker({ key: '', namePt: 'Golpe' }))).toBe(', usou um ataque poderoso');
    expect(powerAttackNote(marker({ key: '', namePt: 'Outra fonte', hidden: true }))).toBe(
      ', usou um ataque poderoso',
    );
  });
});
