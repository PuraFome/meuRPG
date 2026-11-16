import { SPELL_NOTES, spellNotes } from './spell-notes';

describe('the notes of a spell', () => {
  it('has the notes the master applies, for each spell that needs them', () => {
    expect(Object.keys(SPELL_NOTES).sort()).toEqual([
      'spell:disintegrate',
      'spell:eyebite',
      'spell:faithful-hound',
      'spell:flame-blade',
      'spell:vampiric-touch',
    ]);
  });

  it('words Disintegrate as a condition, never as an outcome', () => {
    expect(spellNotes('spell:disintegrate')).toEqual([
      'Se uma criatura chegar a 0 PV por essa magia, ela vira pó: o mestre aplica isso.',
    ]);
  });

  it('says the later attacks of Flame Blade and Vampiric Touch are an action while the spell lasts', () => {
    expect(spellNotes('spell:flame-blade')[0]).toMatch(/enquanto a magia durar.*como uma ação/i);
    expect(spellNotes('spell:vampiric-touch')[0]).toMatch(/enquanto a magia durar.*como uma ação/i);
  });

  it('leaves Eyebite and the Faithful Hound to the master', () => {
    expect(spellNotes('spell:eyebite')[0]).toContain('O mestre aplica o efeito');
    expect(spellNotes('spell:faithful-hound')[0]).toContain('O mestre aplica a mordida');
  });

  it('has nothing for another spell, and no gendered word or foot in any note', () => {
    expect(spellNotes('spell:shield')).toEqual([]);
    for (const n of Object.values(SPELL_NOTES).flat()) {
      expect(n).not.toMatch(/\b(ft|pés?)\b|\b(concentrad|cansad)[oa]\b/);
    }
  });
});
