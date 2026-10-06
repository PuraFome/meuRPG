import type { ClassOptionVm, RulesCatalogVm, SpellOptionVm, SubclassOptionVm } from './character-editor.types';
import {
  allOfSection,
  blocksOf,
  cantripsOf,
  casterSections,
  hitDiceAfterFirst,
  leveledOf,
  offered,
  outsideTheLists,
  unlisted,
  sectionName,
  totalLevel,
} from './class-blocks';

const SRD = { fromTable: false, archived: false, off: false };
const TABLE = { fromTable: true, archived: false, off: false };

const mk = (over: Partial<ClassOptionVm> & Pick<ClassOptionVm, 'key' | 'namePt'>): ClassOptionVm => ({
  hitDie: 8,
  isCaster: false,
  preparation: null,
  subclasses: [],
  subclassLevel: 3,
  spellcastingFirstLevel: 0,
  maxSpellLevelByLevel: [],
  skillChoose: 2,
  savingThrows: [],
  spellListClassKey: '',
  ...SRD,
  ...over,
});

const subclass = (over: Partial<SubclassOptionVm> & Pick<SubclassOptionVm, 'key' | 'namePt'>): SubclassOptionVm => ({ casting: null, alwaysPrepared: [], ...SRD, ...over });

// The wizard's circles by class level, as the server's table gives them (first rows are enough here).
const WIZARD_CIRCLES = [1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 9, 9];
const CLERIC_CIRCLES = [1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 9, 9];
const THIRD_CIRCLES = [0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4];

const spell = (key: string, namePt: string, level: number, classKeys: string[]): SpellOptionVm => ({ key, namePt, level, classKeys, fromTable: key.endsWith('@mesa'), archived: false, off: false });

function catalog(): RulesCatalogVm {
  return {
    races: [],
    classes: [
      mk({ key: 'class:wizard', namePt: 'Mago', hitDie: 6, isCaster: true, preparation: 'spellbook', spellcastingFirstLevel: 1, maxSpellLevelByLevel: WIZARD_CIRCLES, spellListClassKey: 'class:wizard' }),
      mk({
        key: 'class:cleric',
        namePt: 'Clérigo',
        hitDie: 8,
        isCaster: true,
        preparation: 'prepared',
        spellcastingFirstLevel: 1,
        maxSpellLevelByLevel: CLERIC_CIRCLES,
        spellListClassKey: 'class:cleric',
        subclassLevel: 1,
        subclasses: [subclass({ key: 'subclass:life', namePt: 'Domínio da Vida' }), subclass({ key: 'subclass:path@mesa', namePt: 'Domínio do Caminho', ...TABLE })],
      }),
      mk({ key: 'class:bard', namePt: 'Bardo' }),
      mk({ key: 'class:druid', namePt: 'Druida' }),
      mk({ key: 'class:ranger', namePt: 'Patrulheiro' }),
      // A table class that casts from the druid's list.
      mk({ key: 'class:guardiao@mesa', namePt: 'Guardião do Vale', hitDie: 10, isCaster: true, preparation: 'prepared', spellcastingFirstLevel: 2, maxSpellLevelByLevel: [0, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5], spellListClassKey: 'class:druid', ...TABLE }),
      // A fighter whose subclass casts from the wizard's list (a third caster).
      mk({
        key: 'class:fighter',
        namePt: 'Guerreiro',
        hitDie: 10,
        subclasses: [
          subclass({ key: 'subclass:champion', namePt: 'Campeão' }),
          subclass({
            key: 'subclass:ink@mesa',
            namePt: 'Lâmina de Tinta',
            ...TABLE,
            casting: { preparation: 'known', listClassKey: 'class:wizard', firstLevel: 3, maxSpellLevelByLevel: THIRD_CIRCLES },
          }),
        ],
      }),
    ],
    backgrounds: [],
    skills: [],
    armor: [],
    weapons: [],
    spells: [
      spell('spell:fire-bolt', 'Raio de Fogo', 0, ['class:wizard']),
      spell('spell:guidance', 'Orientação', 0, ['class:cleric', 'class:druid']),
      spell('spell:shield', 'Escudo Arcano', 1, ['class:wizard']),
      spell('spell:bless', 'Bênção', 1, ['class:cleric']),
      spell('spell:fireball', 'Bola de Fogo', 3, ['class:wizard']),
      spell('spell:animal-friendship', 'Amizade Animal', 1, ['class:bard', 'class:druid', 'class:ranger']),
      spell('spell:ink-blade@mesa', 'Lâmina de Nanquim', 1, ['class:wizard']),
    ],
    viewerIsMaster: false,
    toolsAndLanguages: [],
    challengeRatings: [],
  };
}

const block = (classKey: string, level: number, subclassKey = '') => ({ classKey, level, subclassKey, customSubclassName: '' });

describe('a sheet of several classes', () => {
  it('adds the levels of every block into the total level', () => {
    expect(totalLevel([block('class:wizard', 3), block('class:cleric', 1)])).toBe(4);
    // A level not typed yet counts as 0 until it is a whole number.
    expect(totalLevel([block('class:wizard', 3), block('class:cleric', Number.NaN)])).toBe(3);
  });

  it('puts the first class first and copies the others', () => {
    const extras = [{ classKey: 'class:cleric', level: 1, subclassKey: '', customSubclassName: '' }];
    const blocks = blocksOf(block('class:wizard', 3), extras);
    expect(blocks.map((b) => b.classKey)).toEqual(['class:wizard', 'class:cleric']);
    expect(blocks[1]).not.toBe(extras[0]);
  });

  it('lists the hit die of every level after the first, class by class, in the order of the rolls', () => {
    // Mago 3 (d6) then Clérigo 1 (d8): levels 2 and 3 of the wizard, then the cleric's first level.
    expect(hitDiceAfterFirst(catalog(), [block('class:wizard', 3), block('class:cleric', 1)])).toEqual([6, 6, 8]);
    expect(hitDiceAfterFirst(catalog(), [block('class:wizard', 1)])).toEqual([]);
  });
});

describe('the spell lists a sheet reads from', () => {
  it('makes a section per casting class, with the highest circle at its own level', () => {
    const sections = casterSections(catalog(), [block('class:wizard', 3), block('class:cleric', 1)]);
    expect(sections.map((s) => [s.namePt, s.maxCircle, s.listClassKey])).toEqual([
      ['Mago', 2, 'class:wizard'],
      ['Clérigo', 1, 'class:cleric'],
    ]);
  });

  it("reads a table class's own list from the class it casts from, not from its own key", () => {
    const [section] = casterSections(catalog(), [block('class:guardiao@mesa', 2)]);
    expect(section.listClassKey).toBe('class:druid');
    expect(section.maxCircle).toBe(1);
    expect(section.firstLevel).toBe(2);
    const names = allOfSection(catalog().spells, section).map((s) => s.namePt);
    expect(names).toEqual(['Orientação', 'Amizade Animal']);
  });

  it("gives a third caster's subclass its own section from its level, with the list it casts from", () => {
    const early = casterSections(catalog(), [block('class:fighter', 2, 'subclass:ink@mesa')]);
    expect(early).toEqual([]);
    const [section] = casterSections(catalog(), [block('class:fighter', 3, 'subclass:ink@mesa')]);
    expect(section).toMatchObject({ namePt: 'Guerreiro', subclassNamePt: 'Lâmina de Tinta', listClassKey: 'class:wizard', maxCircle: 1, preparation: 'known' });
    expect(sectionName(section)).toBe('Guerreiro (Lâmina de Tinta)');
    // The master's own spell is on the wizard's list, so the fighter sees it too.
    expect(leveledOf(catalog().spells, section).map((s) => s.namePt)).toEqual(['Escudo Arcano', 'Lâmina de Nanquim']);
    // No section for a fighter with a subclass that does not cast, nor for a class that never casts.
    expect(casterSections(catalog(), [block('class:fighter', 5, 'subclass:champion')])).toEqual([]);
  });

  it('keeps a picked spell above the circle listed, so it can be unchecked', () => {
    const [section] = casterSections(catalog(), [block('class:wizard', 1)]);
    expect(leveledOf(catalog().spells, section).map((s) => s.key)).not.toContain('spell:fireball');
    expect(leveledOf(catalog().spells, section, new Set(['spell:fireball'])).map((s) => s.key)).toContain('spell:fireball');
    expect(cantripsOf(catalog().spells, section).map((s) => s.key)).toEqual(['spell:fire-bolt']);
  });

  it('finds what a search matches outside every list, with the classes that have it', () => {
    const sections = casterSections(catalog(), [block('class:wizard', 3), block('class:cleric', 1)]);
    const out = outsideTheLists(catalog(), sections, 'amizade', false);
    expect(out.map((o) => o.spell.namePt)).toEqual(['Amizade Animal']);
    expect(out[0].classes).toBe('Bardo, Druida e Patrulheiro');
    // What a list has is not "outside"; an empty search finds nothing.
    expect(outsideTheLists(catalog(), sections, 'escudo', false)).toEqual([]);
    expect(outsideTheLists(catalog(), sections, '', false)).toEqual([]);
  });
});

describe('what a list offers as a new choice (RN-23, 10.1d)', () => {
  const entry = (key: string, archived = false, off = false) => ({ key, archived, off });
  const items = [entry('a'), entry('arch', true), entry('off', false, true), entry('both', true, true)];

  it('offers an archived entry to nobody, and a switched-off one only to the master', () => {
    expect(offered(items, [], false).map((i) => i.key)).toEqual(['a']);
    expect(offered(items, [], true).map((i) => i.key)).toEqual(['a', 'off']);
  });

  it('keeps the value the form already has, whatever it is, so the field is never blank', () => {
    expect(offered(items, ['arch', 'off'], false).map((i) => i.key)).toEqual(['a', 'arch', 'off']);
  });

  it('says a value the catalog does not list at all', () => {
    expect(unlisted(items, 'ghost')).toBe(true);
    expect(unlisted(items, 'arch')).toBe(false);
    expect(unlisted(items, '')).toBe(false);
  });

  it('lists a retired spell only when it is picked, or, switched off, for the master', () => {
    const [section] = casterSections(catalog(), [{ classKey: 'class:wizard', level: 3, subclassKey: '', customSubclassName: '' }]);
    const spells = [...catalog().spells, { ...catalog().spells[0], key: 'spell:old', namePt: 'Velha', archived: true }, { ...catalog().spells[0], key: 'spell:off', namePt: 'Desligada', off: true }];
    const keys = (selected: string[], master: boolean) => cantripsOf(spells, section, new Set(selected), master).map((s) => s.key);
    expect(keys([], false)).toEqual(['spell:fire-bolt']);
    expect(keys([], true).sort()).toEqual(['spell:fire-bolt', 'spell:off']);
    expect(keys(['spell:old'], false)).toContain('spell:old');
  });

  it('never prints a class key where the catalog has no name for it', () => {
    const sections = casterSections(catalog(), [{ classKey: 'class:wizard', level: 3, subclassKey: '', customSubclassName: '' }]);
    const cat = catalog();
    const out = outsideTheLists({ ...cat, spells: [...cat.spells, spell('spell:odd', 'Estranha', 1, ['class:ghost'])] }, sections, 'estranha', false);
    expect(out[0].classes).toBe('');
  });
});
