import { MapMoveStop } from '../../../gen/meurpg/maps/v1/maps_pb';
import {
  DIRECTIONS,
  type MoveOutcome,
  type TokenMoveResult,
  centerOf,
  metersOf,
  moveNotice,
  squareAt,
  squaresBetween,
  stepsWords,
  stopLabel,
  targetAfter,
} from './token-move';

const GRID = { columns: 24, rows: 16 };
const plain = (text: string) => text.replace(/ /g, ' ');
const dir = (id: string) => DIRECTIONS.find((d) => d.id === id)!;

function result(
  name: string,
  asked: number,
  moved: number,
  stopped = MapMoveStop.UNSPECIFIED,
): TokenMoveResult {
  return { characterId: name.toLowerCase(), name, xBp: 0, yBp: 0, asked, moved, stopped };
}

describe('token movement outside combat (B2)', () => {
  it('finds the square of a point and its center, kept inside the grid', () => {
    expect(squareAt({ xBp: 0, yBp: 0 }, GRID)).toEqual({ col: 0, row: 0 });
    expect(squareAt({ xBp: 10000, yBp: 10000 }, GRID)).toEqual({ col: 23, row: 15 });
    const center = centerOf(2, 3, GRID);
    expect(squareAt(center, GRID)).toEqual({ col: 2, row: 3 });
  });

  it('heads steps squares in a direction from the token, and stops at the edge of the map', () => {
    const from = centerOf(5, 5, GRID);
    expect(squareAt(targetAfter(from, dir('e'), 3, GRID), GRID)).toEqual({ col: 8, row: 5 });
    expect(squareAt(targetAfter(from, dir('nw'), 2, GRID), GRID)).toEqual({ col: 3, row: 3 });
    expect(squareAt(targetAfter(from, dir('w'), 20, GRID), GRID)).toEqual({ col: 0, row: 5 });
    expect(squareAt(targetAfter(from, dir('s'), 20, GRID), GRID)).toEqual({ col: 5, row: 15 });
  });

  it('counts squares as the grid does: the larger of the column and the row distance', () => {
    expect(squaresBetween(centerOf(1, 1, GRID), centerOf(4, 3, GRID), GRID)).toBe(3);
    expect(plain(metersOf(3))).toBe('4,5 m');
  });

  it('says the way in words', () => {
    const from = centerOf(5, 5, GRID);
    expect(plain(stepsWords(from, centerOf(8, 5, GRID), GRID))).toBe('3 quadrados para a direita');
    expect(plain(stepsWords(from, centerOf(5, 4, GRID), GRID))).toBe('1 quadrado para cima');
    expect(plain(stepsWords(from, centerOf(7, 7, GRID), GRID))).toBe(
      '2 quadrados na diagonal, para baixo e para a direita',
    );
    expect(plain(stepsWords(from, centerOf(9, 6, GRID), GRID))).toBe(
      '4 quadrados para a direita e 1 para baixo',
    );
    expect(stepsWords(from, from, GRID)).toBe('nenhum quadrado');
  });

  describe('what the master reads after a move', () => {
    const names = new Map([['jabuti', 'Jabuti']]);

    it('says it got there', () => {
      const one: MoveOutcome = { moved: true, blocked: '', results: [result('Tor', 5, 5)] };
      expect(moveNotice(one, names)).toEqual({
        tone: 'done',
        lead: '',
        text: 'Tor chegou ao destino.',
      });
      const three: MoveOutcome = {
        moved: true,
        blocked: '',
        results: [result('P', 3, 3), result('B', 3, 3), result('b', 3, 3)],
      };
      expect(moveNotice(three, names).text).toBe('Os 3 chegaram ao destino, juntos.');
    });

    it("says where it stopped and why, by the server's reason", () => {
      const stop = (reason: MapMoveStop, moved: number): MoveOutcome => ({
        moved: true,
        blocked: '',
        results: [result('Tor', 10, moved, reason)],
      });
      const noPass = moveNotice(stop(MapMoveStop.NO_PASS, 5), names);
      expect([noPass.tone, noPass.lead, plain(noPass.text)]).toEqual([
        'stopped',
        'O movimento parou antes.',
        'O próximo quadrado está marcado como “Não dá para passar”. Tor andou 7,5 m.',
      ]);
      expect(plain(moveNotice(stop(MapMoveStop.WALL, 2), names).text)).toBe(
        'Ninguém atravessa parede. Tor andou 3 m.',
      );
      expect(moveNotice(stop(MapMoveStop.LOCKED, 4), names).text).toContain('porta está trancada');
    });

    it('says nobody moved when a group does not fit, and who is in the way', () => {
      const group: MoveOutcome = {
        moved: false,
        blocked: 'jabuti',
        results: [result('Tor', 5, 0), result('Jabuti', 5, 0)],
      };
      const notice = moveNotice(group, names);
      expect(notice.lead).toBe('Ninguém se moveu.');
      expect(notice.text).toContain('Jabuti não cabe ali');
      expect(notice.text).toContain('tire Jabuti da seleção');
    });

    it('labels the stop on the map', () => {
      expect(plain(stopLabel(result('Tor', 10, 5, MapMoveStop.NO_PASS)))).toBe(
        '7,5 m · parou antes',
      );
    });
  });
});
