import { SpellZoneVisibility } from '../../../gen/meurpg/play/v1/combat_pb';
import { spellZone } from './combat-testing';
import { arrowTurn, BURNING_PATTERN, ZONE_PATTERNS, zoneLegend, zonePattern } from './zone-pattern';
import { zoneView, type ZoneViewer } from './zones';

const master: ZoneViewer = { master: true, combatants: [] };

describe('the pattern of a zone', () => {
  it('gives each of the six spells its own pattern, the same every time', () => {
    const keys = [
      'spell:moonbeam',
      'spell:web',
      'spell:cloudkill',
      'spell:spirit-guardians',
      'spell:grease',
      'spell:entangle',
    ];
    const picked = keys.map(zonePattern);
    expect(new Set(picked).size).toBe(6);
    expect(keys.map(zonePattern)).toEqual(picked);
  });

  const CATALOG = [
    'spell:moonbeam',
    'spell:web',
    'spell:spirit-guardians',
    'spell:cloudkill',
    'spell:grease',
    'spell:entangle',
    'spell:silence',
    'spell:spike-growth',
    'spell:fog-cloud',
    'spell:stinking-cloud',
    'spell:darkness',
    'spell:wall-of-fire',
  ];

  it('gives every catalog spell its own pattern, drawing class and drawing', () => {
    const picked = CATALOG.map(zonePattern);
    expect(new Set(picked).size).toBe(CATALOG.length);
    const all = [...picked, BURNING_PATTERN].map((i) => ZONE_PATTERNS[i]);
    expect(new Set(all.map((t) => t.cls)).size).toBe(all.length);
    expect(new Set(all.map((t) => t.d)).size).toBe(all.length);
    expect(new Set(ZONE_PATTERNS.map((t) => t.cls)).size).toBe(ZONE_PATTERNS.length);
  });

  it('keeps the looks of the first six as they were', () => {
    const tint = (k: string) => ZONE_PATTERNS[zonePattern(k)];
    expect(tint('spell:moonbeam')).toMatchObject({
      cls: 'vlines',
      tint: 'ink',
      veil: { tone: 'surface', pct: 38 },
    });
    expect(tint('spell:spirit-guardians').tint).toBe('accent');
    expect(tint('spell:grease').cls).toBe('triangles');
  });

  it('veils the clouds, the Darkness and the wall as the board says', () => {
    const veil = (k: string) => ZONE_PATTERNS[zonePattern(k)].veil;
    expect(veil('spell:silence')).toEqual({ tone: 'surface', pct: 38 });
    expect(veil('spell:fog-cloud')).toEqual({ tone: 'surface', pct: 72 });
    expect(veil('spell:stinking-cloud')).toEqual({ tone: 'surface', pct: 72 });
    expect(veil('spell:darkness')).toEqual({ tone: 'ink', pct: 80 });
    expect(veil('spell:wall-of-fire')).toEqual({ tone: 'accent', pct: 72 });
    expect(ZONE_PATTERNS[zonePattern('spell:darkness')].tint).toBe('light');
    expect(ZONE_PATTERNS[zonePattern('spell:wall-of-fire')].tint).toBe('light');
  });

  it('draws the burning side with arrows, garnet and dashed', () => {
    expect(ZONE_PATTERNS[BURNING_PATTERN]).toMatchObject({
      cls: 'arrows',
      tint: 'accent',
      dashed: true,
    });
  });

  it('never hashes a table spell onto a look that veils', () => {
    for (const k of ['table:abc-123', 'table:x', 'spell:unknown', 'spell:fireball']) {
      expect(zonePattern(k)).toBeLessThan(6);
    }
  });

  it('turns the arrows from the wall to its burning side', () => {
    const wall = [
      { col: 4, row: 3 },
      { col: 5, row: 3 },
    ];
    expect(
      arrowTurn(wall, [
        { col: 4, row: 4 },
        { col: 5, row: 4 },
      ]),
    ).toBe(90);
    expect(
      arrowTurn(wall, [
        { col: 4, row: 2 },
        { col: 5, row: 2 },
      ]),
    ).toBe(270);
    expect(arrowTurn([{ col: 4, row: 3 }], [{ col: 5, row: 3 }])).toBe(0);
    expect(arrowTurn([{ col: 4, row: 3 }], [{ col: 3, row: 3 }])).toBe(180);
    expect(arrowTurn(wall, [])).toBe(0);
  });

  it('gives a table spell a pattern inside the palette', () => {
    const p = zonePattern('table:abc-123');
    expect(p).toBeGreaterThanOrEqual(0);
    expect(p).toBeLessThan(ZONE_PATTERNS.length);
    expect(zonePattern('table:abc-123')).toBe(p);
  });
});

describe('the legend of the zones on the map', () => {
  it('lists each spell once, in the order they were cast', () => {
    const views = [
      zoneView(spellZone({ id: 'a' }), master),
      zoneView(spellZone({ id: 'b', spellKey: 'spell:web', spellNamePt: 'Teia' }), master),
      zoneView(spellZone({ id: 'c' }), master),
    ];
    expect(zoneLegend(views).map((i) => i.name)).toEqual(['Raio Lunar', 'Teia']);
  });

  it('says "só você vê" for a spell whose every zone is the master\'s alone', () => {
    const hidden = spellZone({
      id: 'a',
      spellKey: 'spell:cloudkill',
      spellNamePt: 'Nuvem Mortal',
      visibility: SpellZoneVisibility.MASTER_ONLY,
    });
    const [item] = zoneLegend([zoneView(hidden, master)]);
    expect(item.masterOnly).toBe(true);
    const both = zoneLegend([
      zoneView(hidden, master),
      zoneView({ ...hidden, id: 'b', visibility: SpellZoneVisibility.EVERYONE }, master),
    ]);
    expect(both[0].masterOnly).toBe(false);
  });

  it("carries the letters of a spell's zones and adds the burning side of a wall under the wall's letter", () => {
    const wall = spellZone({
      id: 'w',
      spellKey: 'spell:wall-of-fire',
      spellNamePt: 'Muralha de Fogo',
      burningSquares: [{ col: 1, row: 1 }],
    });
    const views = [
      zoneView(spellZone({ id: 'a' }), master, 0),
      zoneView(wall, master, 1),
      zoneView(spellZone({ id: 'c' }), master, 2),
    ];
    const legend = zoneLegend(views);
    expect(legend.map((i) => [i.name, i.letters])).toEqual([
      ['Raio Lunar', ['A', 'C']],
      ['Muralha de Fogo', ['B']],
      ['Lado que queima', ['B']],
    ]);
  });

  it('is empty with no zone', () => {
    expect(zoneLegend([])).toEqual([]);
  });
});
