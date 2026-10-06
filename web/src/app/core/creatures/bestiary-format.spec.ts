import { acAndHp, alignmentPt, capitalized, creatureKeyOf, creatureSlug, listWithE, npcSpeedFt, typeAndSize } from './bestiary-format';
import { challengeBounds } from './creature-types';
import { ogre, summary } from './creatures-testing';

describe('the bestiary\'s words (MR-042)', () => {
  it('a row says the type and size, and CA and PV, as the artboard does', () => {
    const wolf = summary('monster:wolf', 'Lobo', { typePt: 'fera', sizePt: 'Médio', armorClass: 13, hitPoints: 11 });
    expect(typeAndSize(wolf).replace(/\u00a0/g, ' ')).toBe('Fera · Médio');
    expect(acAndHp(wolf).replace(/ /g, ' ')).toBe('CA 13 · PV 11');
    expect(capitalized('')).toBe('');
  });

  it('the alignment is Portuguese when the table knows it, and the book\'s English (marked) when not', () => {
    expect(alignmentPt('chaotic evil')).toEqual({ text: 'caótico e mau', english: false });
    expect(alignmentPt('Unaligned')).toEqual({ text: 'sem alinhamento', english: false });
    expect(alignmentPt('neutral good (50%) or neutral evil (50%)')).toEqual({ text: 'neutral good (50%) or neutral evil (50%)', english: true });
    expect(alignmentPt('')).toEqual({ text: '', english: false });
  });

  it('the route uses the key without its prefix', () => {
    expect(creatureSlug('monster:giant-wolf-spider')).toBe('giant-wolf-spider');
    expect(creatureKeyOf('ogre')).toBe('monster:ogre');
  });

  it('an NPC walks at the creature\'s speed, or at its best one when it does not walk', () => {
    expect(npcSpeedFt(ogre())).toBe(40);
    expect(npcSpeedFt(ogre({ speedWalkFt: 0, speedSwimFt: 60, speedFlyFt: 30 }))).toBe(60);
  });

  it('lists names with "e" before the last', () => {
    expect(listWithE([])).toBe('');
    expect(listWithE(['Cimitarra'])).toBe('Cimitarra');
    expect(listWithE(['Cimitarra', 'Adaga'])).toBe('Cimitarra e Adaga');
    expect(listWithE(['A', 'B', 'C'])).toBe('A, B e C');
  });

  it('the ND filter is one rating or a range', () => {
    expect(challengeBounds('')).toEqual({ minCr: '', maxCr: '' });
    expect(challengeBounds('1/4')).toEqual({ minCr: '1/4', maxCr: '1/4' });
    expect(challengeBounds('1-4')).toEqual({ minCr: '1', maxCr: '4' });
    expect(challengeBounds('11-30')).toEqual({ minCr: '11', maxCr: '30' });
  });
});
