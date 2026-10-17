import type { CasterSection } from './class-blocks';
import { SpellPicks } from './spell-picks';

const section = (preparation: CasterSection['preparation']): CasterSection => ({
  index: 0,
  classKey: 'class:wizard',
  namePt: 'Mago',
  subclassNamePt: '',
  level: 1,
  listClassKey: 'class:wizard',
  preparation,
  maxCircle: 1,
  firstLevel: 1,
  alwaysPrepared: [],
  alwaysSourcePt: '',
});

describe('SpellPicks', () => {
  const picks = (sections: CasterSection[]) => new SpellPicks({ sections: () => sections });

  it('toggles each list on its own', () => {
    const p = picks([]);
    p.toggleCantrip('c');
    p.toggleKnown('k');
    p.togglePrepared('p');
    expect([...p.cantrips()]).toEqual(['c']);
    expect([...p.known()]).toEqual(['k']);
    expect([...p.prepared()]).toEqual(['p']);
    p.toggleCantrip('c');
    expect(p.cantrips().size).toBe(0);
  });

  it("unprepares a spell that leaves a wizard's spellbook", () => {
    const p = picks([section('spellbook')]);
    p.toggleKnown('s');
    p.togglePrepared('s');
    p.toggleKnown('s');
    expect(p.known().has('s')).toBe(false);
    expect(p.prepared().has('s')).toBe(false);
  });

  it('keeps it prepared for a class that prepares from its whole list, or when only some classes keep a book', () => {
    for (const sections of [
      [section('prepared')],
      [section('spellbook'), section('prepared')],
      [],
    ]) {
      const p = picks(sections);
      p.toggleKnown('s');
      p.togglePrepared('s');
      p.toggleKnown('s');
      expect(p.prepared().has('s')).toBe(true);
    }
  });

  it('leaves the prepared spells alone when a spell is added to the book', () => {
    const p = picks([section('spellbook')]);
    p.togglePrepared('s');
    p.toggleKnown('s');
    expect(p.prepared().has('s')).toBe(true);
  });

  it('adds a cantrip once and keeps the same set when it is there', () => {
    const p = picks([]);
    p.addCantrip('c');
    const first = p.cantrips();
    p.addCantrip('c');
    expect(p.cantrips()).toBe(first);
  });

  it('keeps the search of each list apart', () => {
    const p = picks([]);
    expect(p.filterOf('0:cantrips')).toBe('');
    p.setFilter('0:cantrips', 'fo');
    p.setFilter('1:known', 'ge');
    expect(p.filterOf('0:cantrips')).toBe('fo');
    expect(p.filterOf('1:known')).toBe('ge');
  });

  it("loads a saved sheet's picks and starts over", () => {
    const p = picks([]);
    p.load(['c'], ['k'], ['p']);
    p.setFilter('0:cantrips', 'x');
    expect([...p.cantrips(), ...p.known(), ...p.prepared()]).toEqual(['c', 'k', 'p']);
    p.reset();
    expect([...p.cantrips(), ...p.known(), ...p.prepared()]).toEqual([]);
    expect(p.filters()).toEqual({});
  });
});
