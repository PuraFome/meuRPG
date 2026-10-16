import type { RulesCatalogVm, SpellLimitsVm, SpellOptionVm } from './character-editor.types';
import type { CasterSection } from './class-blocks';
import {
  availableCantripsOf,
  availableSpellsOf,
  buildSectionView,
  filterByName,
  keepAvailable,
  preparedCountLine,
  sectionExtrasOf,
  spellListLimits,
} from './spell-views';

const spell = (key: string, namePt: string, level: number, classKeys: string[]): SpellOptionVm => ({
  key,
  namePt,
  level,
  classKeys,
  fromTable: false,
  archived: false,
  off: false,
});

const SPELLS = [
  spell('spell:fire-bolt', 'Raio de Fogo', 0, ['class:wizard']),
  spell('spell:frost', 'Raio de Gelo', 0, ['class:wizard']),
  spell('spell:shield', 'Escudo Arcano', 1, ['class:wizard']),
  spell('spell:missile', 'Mísseis Mágicos', 1, ['class:wizard']),
  spell('spell:fireball', 'Bola de Fogo', 3, ['class:wizard']),
  spell('spell:bless', 'Bênção', 1, ['class:cleric']),
  spell('spell:hex', 'Maldição', 1, ['class:warlock']),
];

const catalog = { spells: SPELLS, classes: [] } as unknown as RulesCatalogVm;

const section = (over: Partial<CasterSection> = {}): CasterSection => ({
  index: 0,
  classKey: 'class:wizard',
  namePt: 'Mago',
  subclassNamePt: '',
  level: 3,
  listClassKey: 'class:wizard',
  preparation: 'spellbook',
  maxCircle: 2,
  firstLevel: 1,
  alwaysPrepared: [],
  alwaysSourcePt: '',
  ...over,
});

const keys = (list: readonly { key: string }[]) => list.map((s) => s.key);

describe('filterByName', () => {
  it('narrows by a case-insensitive part of the Portuguese name, and keeps all for a blank search', () => {
    expect(keys(filterByName(SPELLS, ' RAIO '))).toEqual(['spell:fire-bolt', 'spell:frost']);
    expect(filterByName(SPELLS, '  ')).toBe(SPELLS);
    expect(filterByName(SPELLS, 'zzz')).toEqual([]);
  });
});

describe('preparedCountLine', () => {
  it('says how many are prepared against the number, and whether it is past it', () => {
    expect(preparedCountLine('prepared', 3, 5)).toEqual({ text: 'Preparadas 3 de 5', over: false });
    expect(preparedCountLine('spellbook', 7, 5)).toEqual({ text: 'Preparadas 7 de 5', over: true });
  });

  it('says nothing for a known caster or a class with no number', () => {
    expect(preparedCountLine('known', 3, 5)).toEqual({ text: '', over: false });
    expect(preparedCountLine('prepared', 3, undefined)).toEqual({ text: '', over: false });
    expect(preparedCountLine(null, 3, undefined)).toEqual({ text: '', over: false });
  });
});

describe('spellListLimits', () => {
  const server: SpellLimitsVm = {
    classKey: 'class:wizard',
    cantripsKnown: 3,
    spellsKnown: 4,
    preparedMax: 6,
  };

  it('takes the numbers of the list each kind of class has', () => {
    expect(spellListLimits(server, 'prepared', 9)).toEqual({
      cantrips: 3,
      known: null,
      prepared: 6,
      preparedMax: 6,
    });
    expect(spellListLimits(server, 'known', undefined)).toMatchObject({ known: 4, prepared: null });
  });

  it("falls back to the saved sheet's prepared number, and shows none where there is no number", () => {
    expect(spellListLimits(undefined, 'prepared', 5)).toEqual({
      cantrips: null,
      known: null,
      prepared: 5,
      preparedMax: 5,
    });
    expect(
      spellListLimits({ ...server, preparedMax: 0, cantripsKnown: 0 }, 'prepared', undefined),
    ).toEqual({
      cantrips: null,
      known: null,
      prepared: null,
      preparedMax: undefined,
    });
  });
});

describe('the spells the sheet may send', () => {
  it('unites the cantrips of the sections once each, by name', () => {
    const both = [
      section(),
      section({ index: 1, classKey: 'class:cleric', listClassKey: 'class:wizard' }),
    ];
    expect(keys(availableCantripsOf(catalog, both, new Set()))).toEqual([
      'spell:fire-bolt',
      'spell:frost',
    ]);
    expect(keys(availableCantripsOf(catalog, [], new Set()))).toEqual([]);
  });

  it("unites the leveled spells of the lists, then the extra lists' not already there", () => {
    const extras = [
      {
        index: 0,
        lists: [
          {
            id: 'patron' as const,
            title: 'Do patrono',
            hint: '',
            spells: [SPELLS[2], SPELLS[6]],
          },
        ],
      },
    ];
    expect(keys(availableSpellsOf(catalog, [section()], extras))).toEqual([
      'spell:shield',
      'spell:missile',
      'spell:fireball',
      'spell:hex',
    ]);
  });
});

describe('sectionExtrasOf', () => {
  it('is empty when the server named no source', () => {
    expect(sectionExtrasOf(catalog, [section()], [], new Set(), false)).toEqual([]);
  });

  it('gives each section the lists of the source for its class', () => {
    const sources = [
      {
        classKey: 'class:wizard',
        patronSpells: [{ spellKey: 'spell:hex', classLevel: 1 }],
        secrets: 0,
        secretsMaxSpellLevel: 0,
      },
    ];
    const out = sectionExtrasOf(
      catalog,
      [section(), section({ index: 1, classKey: 'class:cleric' })],
      sources,
      new Set(),
      false,
    );
    expect(out.map((e) => e.index)).toEqual([0, 1]);
    expect(out[0].lists.map((l) => l.id)).toEqual(['patron']);
    expect(out[1].lists).toEqual([]);
  });
});

describe('keepAvailable', () => {
  it('keeps what is available and what the catalog does not know, and drops what left the lists', () => {
    const picked = new Set(['spell:shield', 'spell:bless', 'spell:ghost']);
    expect(keepAvailable(picked, [{ key: 'spell:shield' }], SPELLS)).toEqual([
      'spell:shield',
      'spell:ghost',
    ]);
  });
});

describe('buildSectionView', () => {
  const input = (over: Partial<Parameters<typeof buildSectionView>[0]> = {}) => ({
    catalog,
    sections: [section()],
    section: section(),
    cantrips: new Set<string>(),
    known: new Set<string>(),
    prepared: new Set<string>(),
    master: false,
    filters: {},
    extras: [],
    serverLimits: undefined,
    savedPreparedMax: undefined,
    ...over,
  });

  it('lists the cantrips and the spells up to the circle the level reaches', () => {
    const view = buildSectionView(input());
    expect(keys(view.cantrips.all)).toEqual(['spell:fire-bolt', 'spell:frost']);
    expect(view.cantrips.show).toBe(true);
    expect(keys(view.known.all)).toEqual(['spell:shield', 'spell:missile']);
    expect(view.known.show).toBe(true);
    expect(view.id).toBe('0');
    expect(view.castingStarts).toBe('O Mago conjura magias a partir do nível 1.');
  });

  it("lists a spellbook wizard's preparable spells as the book's only", () => {
    expect(keys(buildSectionView(input()).prepared.all)).toEqual([]);
    const view = buildSectionView(
      input({ known: new Set(['spell:shield']), prepared: new Set(['spell:missile']) }),
    );
    expect(keys(view.prepared.all)).toEqual(['spell:shield', 'spell:missile']);
    expect(view.preparedCount).toEqual({ text: '', over: false });
  });

  it('narrows each list by its own search and finds spells outside the lists', () => {
    const view = buildSectionView(
      input({ filters: { '0:cantrips': 'gelo', '0:known': 'male', '0:prepared': 'bênção' } }),
    );
    expect(keys(view.cantrips.shown)).toEqual(['spell:frost']);
    expect(view.cantrips.filter).toBe('gelo');
    expect(view.known.shown).toEqual([]);
    expect(view.prepared.outside.map((o) => o.spell.key)).toEqual(['spell:bless']);
  });

  it('keeps a picked spell above the circle so it can be unchecked, and the picker of a cantrip on the sheet', () => {
    const view = buildSectionView(
      input({ known: new Set(['spell:fireball']), cantrips: new Set(['spell:ghost']) }),
    );
    expect(keys(view.known.all)).toContain('spell:fireball');
    const none = buildSectionView(
      input({ section: section({ maxCircle: 0 }), sections: [section({ maxCircle: 0 })] }),
    );
    expect(none.noLeveledYet).toBe(true);
    expect(none.known.show).toBe(false);
    expect(none.prepared.show).toBe(false);
  });

  it('takes the always-prepared spells out of the lists and shows the ones the catalog has', () => {
    const cleric = section({
      classKey: 'class:cleric',
      listClassKey: 'class:cleric',
      preparation: 'prepared',
      maxCircle: 3,
      alwaysPrepared: ['spell:bless', 'spell:ghost'],
      alwaysSourcePt: 'Domínio da Vida',
    });
    const view = buildSectionView(input({ section: cleric, sections: [cleric] }));
    expect(keys(view.always)).toEqual(['spell:bless']);
    expect(keys(view.prepared.all)).toEqual([]);
    expect(view.alwaysSourcePt).toBe('Domínio da Vida');
    expect(view.extrasToKnown).toBe(false);
  });

  it("counts the prepared picks against the server's number, else the saved one", () => {
    const cleric = section({
      classKey: 'class:cleric',
      listClassKey: 'class:wizard',
      preparation: 'prepared',
    });
    const server: SpellLimitsVm = {
      classKey: 'class:cleric',
      cantripsKnown: 3,
      spellsKnown: 0,
      preparedMax: 1,
    };
    const picks = new Set(['spell:shield', 'spell:missile']);
    const view = buildSectionView(
      input({ section: cleric, sections: [cleric], prepared: picks, serverLimits: server }),
    );
    expect(view.preparedCount).toEqual({ text: 'Preparadas 2 de 1', over: true });
    expect(view.cantripsLimit).toBe(3);
    expect(view.preparedLimit).toBe(1);
    const saved = buildSectionView(
      input({ section: cleric, sections: [cleric], prepared: picks, savedPreparedMax: 4 }),
    );
    expect(saved.preparedCount).toEqual({ text: 'Preparadas 2 de 4', over: false });
  });

  it('hands the section its own extra lists', () => {
    const lists = [{ id: 'secrets' as const, title: 'Segredos', hint: '', spells: [] }];
    expect(buildSectionView(input({ extras: [{ index: 0, lists }] })).extras).toBe(lists);
    expect(buildSectionView(input({ extras: [{ index: 5, lists }] })).extras).toEqual([]);
  });
});
