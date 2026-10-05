import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import { MapPointKind, MapPointSchema } from '../../../gen/meurpg/maps/v1/maps_pb';
import { TreasureWatch, foundToastTitle, poText, summaryLine } from './treasure-text';

const found = (id: string, who: string[] = ['Brisa']) =>
  create(MapPointSchema, {
    id,
    kind: MapPointKind.TREASURE,
    name: 'Baú de moedas',
    treasureValuePo: 250,
    treasureFoundAt: timestampFromDate(new Date(2026, 9, 4, 21, 40)),
    treasureFoundBy: who.map((n) => ({ characterId: n, characterName: n })),
  });
const hidden = (id: string) => create(MapPointSchema, { id, kind: MapPointKind.TREASURE, name: 'Baú com agulha' });
const plain = (t: string) => t.replace(/ /g, ' ');

describe('treasure text', () => {
  it('writes PO with the unit tied to the number', () => {
    expect(poText(1250)).toBe('1.250 PO');
  });

  it('writes the toast', () => {
    expect(foundToastTitle(found('a'))).toBe('Brisa encontrou: Baú de moedas');
    expect(foundToastTitle(found('a', ['Brisa', 'Toren']))).toBe('Brisa e Toren encontraram: Baú de moedas');
  });

  it('says what goes into the summary without dividing', () => {
    expect(summaryLine([], 250)).toBe('Ninguém marcado. Escolha quem encontrou.');
    expect(plain(summaryLine(['Brisa'], 250))).toBe('No resumo da sessão: Brisa · 250 PO');
    expect(plain(summaryLine(['Brisa', 'Toren'], 250))).toBe('No resumo da sessão: Brisa e Toren · 250 PO');
  });

  it('never calls a treasure found before the page arrived news', () => {
    const w = new TreasureWatch();
    expect(w.newlyFound('m1', [found('a'), hidden('b')])).toEqual([]);
    expect(w.newlyFound('m1', [found('a'), found('b')]).map((p) => p.id)).toEqual(['b']);
    expect(w.newlyFound('m1', [found('a'), found('b')])).toEqual([]);
    expect(w.newlyFound('m2', [found('c')])).toEqual([]);
    expect(w.newlyFound(null, [])).toEqual([]);
  });
});
