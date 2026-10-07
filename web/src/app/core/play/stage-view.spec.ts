import { stageNpc } from './scene-testing';
import {
  STAGE_FULL_REASON,
  initialsOf,
  joinNames,
  stageAnnouncement,
  stageCount,
} from './stage-view';

describe('stage-view', () => {
  it('draws the initials: two letters of one word, one of each of the first two words', () => {
    expect(initialsOf('Aldo')).toBe('AL');
    expect(initialsOf('Capitão Goblin')).toBe('CG');
    expect(initialsOf('Barão Ivo de Alto Vale')).toBe('BI');
    expect(initialsOf('  mira ')).toBe('MI');
    expect(initialsOf('Ö')).toBe('Ö');
    expect(initialsOf('   ')).toBe('?');
  });

  it('counts the stage with the numbers tied to their words', () => {
    expect(stageCount(2)).toBe('2 de 4 em cena');
    expect(STAGE_FULL_REASON).toBe('A cena comporta 4 NPCs. Tire um para pôr outro.');
  });

  it('joins names the way a sentence does', () => {
    expect(joinNames([])).toBe('');
    expect(joinNames(['Mira'])).toBe('Mira');
    expect(joinNames(['Mira', 'Aldo'])).toBe('Mira e Aldo');
    expect(joinNames(['Mira', 'Aldo', 'Barão Ivo'])).toBe('Mira, Aldo e Barão Ivo');
  });

  describe('what a screen reader hears', () => {
    const mira = stageNpc('s1', 'Mira');
    const capitao = stageNpc('s2', 'Capitão Goblin');
    const aldo = stageNpc('s3', 'Aldo');
    const ivo = stageNpc('s4', 'Barão Ivo');

    it('says nothing when nothing changed', () => {
      expect(stageAnnouncement([mira], [mira])).toBe('');
    });

    it('says who came in and who left', () => {
      expect(stageAnnouncement([mira], [mira, capitao])).toBe('Capitão Goblin entrou na cena.');
      expect(stageAnnouncement([mira, capitao], [mira])).toBe('Capitão Goblin saiu da cena.');
    });

    it('joins several that came or left at once in one sentence each', () => {
      expect(stageAnnouncement([mira], [mira, aldo, ivo])).toBe(
        'Aldo e Barão Ivo entraram na cena.',
      );
      expect(stageAnnouncement([mira, aldo, ivo], [mira])).toBe('Aldo e Barão Ivo saíram da cena.');
      expect(stageAnnouncement([mira, capitao], [aldo, ivo])).toBe(
        'Aldo e Barão Ivo entraram na cena. Mira e Capitão Goblin saíram da cena.',
      );
    });

    it('says who speaks, and when nobody does', () => {
      const speaking = { ...capitao, speaking: true };
      expect(stageAnnouncement([mira, capitao], [mira, speaking])).toBe('Capitão Goblin fala.');
      expect(stageAnnouncement([mira, speaking], [mira, capitao])).toBe('Ninguém fala.');
      expect(stageAnnouncement([mira, speaking], [{ ...mira, speaking: true }, capitao])).toBe(
        'Mira fala.',
      );
    });

    it('says both when an NPC comes in speaking', () => {
      expect(stageAnnouncement([mira], [mira, { ...aldo, speaking: true }])).toBe(
        'Aldo entrou na cena. Aldo fala.',
      );
    });

    it('does not say "Ninguém fala" when the speaker simply left', () => {
      expect(stageAnnouncement([mira, { ...capitao, speaking: true }], [mira])).toBe(
        'Capitão Goblin saiu da cena.',
      );
    });
  });
});
