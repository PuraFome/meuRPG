import { catalogChanged, offFieldOf } from './catalog-changes';
import type { RulesCatalogVm } from './character-editor.types';

const base = (): RulesCatalogVm => ({
  races: [{ key: 'race:gnome', namePt: 'Gnomo', constitutionBonus: 0, subraces: [{ key: 'subrace:rock', namePt: 'Gnomo da Rocha', constitutionBonus: 1 }] }],
  classes: [{ key: 'class:wizard', namePt: 'Mago', hitDie: 6, isCaster: true, preparation: null, subclasses: [{ key: 'subclass:evocation', namePt: 'Evocação' }], subclassLevel: 2, spellcastingFirstLevel: 1 } as never],
  backgrounds: [{ key: 'background:acolyte', namePt: 'Acólito' } as never],
  skills: [],
  armor: [],
  weapons: [],
  spells: [{ key: 'spell:light', namePt: 'Luz', level: 0, classKeys: ['class:wizard'] }],
  challengeRatings: [],
});

describe('catalogChanged: what the master\'s switches move in the pickers', () => {
  it('is false for the same lists', () => {
    expect(catalogChanged(base(), base())).toBe(false);
  });

  it('sees a race, a subrace, a class, a subclass, a background or a spell come or go', () => {
    expect(catalogChanged(base(), { ...base(), races: [] })).toBe(true);
    expect(catalogChanged(base(), { ...base(), races: [{ ...base().races[0], subraces: [] }] })).toBe(true);
    expect(catalogChanged(base(), { ...base(), classes: [] })).toBe(true);
    expect(catalogChanged(base(), { ...base(), classes: [{ ...base().classes[0], subclasses: [] }] })).toBe(true);
    expect(catalogChanged(base(), { ...base(), backgrounds: [] })).toBe(true);
    expect(catalogChanged(base(), { ...base(), spells: [] })).toBe(true);
    // A spell that left a class's list too.
    expect(catalogChanged(base(), { ...base(), spells: [{ key: 'spell:light', namePt: 'Luz', level: 0, classKeys: [] }] })).toBe(true);
  });
});

describe('offFieldOf: which field holds the refused key', () => {
  const form = { race: 'race:gnome', subrace: 'subrace:rock', className: 'class:wizard', subclassName: 'subclass:evocation', background: 'background:acolyte' };

  it('names the field', () => {
    expect(offFieldOf('race:gnome', form)).toBe('race');
    expect(offFieldOf('subrace:rock', form)).toBe('subrace');
    expect(offFieldOf('class:wizard', form)).toBe('class');
    expect(offFieldOf('subclass:evocation', form)).toBe('subclass');
    expect(offFieldOf('background:acolyte', form)).toBe('background');
  });

  it('is null when no field holds it: a spell, or a choice the person already changed', () => {
    expect(offFieldOf('spell:light', form)).toBeNull();
    expect(offFieldOf('race:elf', form)).toBeNull();
  });
});
