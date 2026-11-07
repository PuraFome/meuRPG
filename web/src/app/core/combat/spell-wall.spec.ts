import { WallSide } from '../../../gen/meurpg/play/v1/combat_pb';
import {
  WALL_SPELL_KEY,
  laysWall,
  lengthNote,
  lengthText,
  newWall,
  sideChoices,
  sideNote,
  stepLength,
  switchShape,
  wallBody,
  wallHint,
  wallZoneView,
} from './spell-wall';

const plain = (t: string) => t.replace(/\u00a0/g, ' ');

describe('spell-wall: what the spell lays', () => {
  it('knows the spell that lays a wall by its catalog key, and no other', () => {
    expect(WALL_SPELL_KEY).toBe('spell:wall-of-fire');
    expect(laysWall('spell:wall-of-fire')).toBe(true);
    expect(laysWall('spell:fireball')).toBe(false);
  });

  it('starts a line eastward at the full 60 ft on the right, and a ring at 20 ft inside', () => {
    expect(newWall(false)).toEqual({
      ring: false,
      direction: { dx: 1, dy: 0 },
      lengthFt: 60,
      side: WallSide.RIGHT,
    });
    expect(newWall(true)).toEqual({
      ring: true,
      direction: null,
      lengthFt: 20,
      side: WallSide.INNER,
    });
  });

  it('steps the length a square (5 ft, 1,5 m) at a time, within 5 to 60 ft for a line and 10 to 20 ft for a ring', () => {
    const line = { ...newWall(false), lengthFt: 40 };
    expect(stepLength(line, -1).lengthFt).toBe(35);
    expect(stepLength(line, 1).lengthFt).toBe(45);
    expect(stepLength({ ...line, lengthFt: 5 }, -1).lengthFt).toBe(5);
    expect(stepLength({ ...line, lengthFt: 60 }, 1).lengthFt).toBe(60);
    const ring = newWall(true);
    expect(stepLength(ring, 1).lengthFt).toBe(20);
    expect(stepLength({ ...ring, lengthFt: 15 }, -1).lengthFt).toBe(10);
    expect(stepLength({ ...ring, lengthFt: 10 }, -1).lengthFt).toBe(10);
  });

  it('says the length in meters and how many squares it is', () => {
    expect(plain(lengthText({ ...newWall(false), lengthFt: 40 }))).toBe('12 m');
    expect(plain(lengthText({ ...newWall(false), lengthFt: 5 }))).toBe('1,5 m');
    expect(plain(lengthNote({ ...newWall(false), lengthFt: 40 }))).toBe(
      'De 1,5 m a 18 m: 8 quadrados.',
    );
    expect(plain(lengthNote(newWall(true)))).toBe('De 3 m a 6 m.');
  });

  it('offers left and right for a line, inside and outside for a ring, never mixed', () => {
    expect(
      sideChoices(false).map((c: { side: WallSide; label: string }) => [c.side, c.label]),
    ).toEqual([
      [WallSide.LEFT, 'À esquerda'],
      [WallSide.RIGHT, 'À direita'],
    ]);
    expect(
      sideChoices(true).map((c: { side: WallSide; label: string }) => [c.side, c.label]),
    ).toEqual([
      [WallSide.INNER, 'Por dentro'],
      [WallSide.OUTER, 'Por fora'],
    ]);
  });

  it('changing the shape resets the side and the direction and keeps the length inside the new size', () => {
    const line = { ...newWall(false), lengthFt: 40, side: WallSide.LEFT };
    const ring = switchShape(line, true);
    expect(ring).toEqual({ ring: true, direction: null, lengthFt: 20, side: WallSide.INNER });
    const back = switchShape({ ...ring, lengthFt: 10 }, false);
    expect(back.ring).toBe(false);
    expect(back.direction).toEqual({ dx: 1, dy: 0 });
    expect(back.side).toBe(WallSide.RIGHT);
    expect(back.lengthFt).toBe(10);
    expect(switchShape(line, false)).toBe(line);
  });

  it('writes the request: a line has a direction, a ring has none, and the length is always explicit', () => {
    expect(wallBody({ ...newWall(false), direction: { dx: 1, dy: -1 }, lengthFt: 35 })).toEqual({
      ring: false,
      direction: { dx: 1, dy: -1 },
      lengthFt: 35,
      side: WallSide.RIGHT,
    });
    const ring = wallBody(newWall(true));
    expect(ring).toEqual({ ring: true, lengthFt: 20, side: WallSide.INNER });
    expect('direction' in ring).toBe(false);
  });

  it('writes the note of the side with the dice of the slot, and the ring says which side', () => {
    expect(plain(sideNote(newWall(false), '5d8'))).toBe(
      'A seta no mapa mostra o lado: 5d8 de fogo para quem termina o turno ali, até 3 m.',
    );
    expect(plain(sideNote(newWall(true), '6d8'))).toBe(
      'Quem termina o turno dentro do anel, ou entra na muralha, sofre 6d8 de fogo.',
    );
    expect(plain(sideNote({ ...newWall(true), side: WallSide.OUTER }, ''))).toBe(
      'Quem termina o turno fora do anel, ou entra na muralha, sofre dano de fogo.',
    );
  });

  it('says how to place a line or a ring before and after the point is placed', () => {
    expect(wallHint(newWall(false), false)).toContain('Toque no começo e arraste para onde vai');
    expect(wallHint(newWall(false), true)).toContain('Começo marcado');
    expect(wallHint(newWall(true), false)).toContain('Toque no centro do anel');
    expect(wallHint(newWall(true), true)).toContain('Centro marcado');
  });

  it('makes the zone the map draws from the answer: the wall, the burning side and its side', () => {
    const view = wallZoneView(
      {
        origin: { col: 4, row: 3 },
        squares: [
          { col: 4, row: 3 },
          { col: 5, row: 3 },
        ],
        burningSquares: [{ col: 4, row: 4 }],
      },
      WallSide.LEFT,
    );
    expect(view.spellKey).toBe(WALL_SPELL_KEY);
    expect(view.origin).toEqual({ col: 4, row: 3 });
    expect(view.squares).toHaveLength(2);
    expect(view.burningSquares).toEqual([{ col: 4, row: 4 }]);
    expect(view.wallSide).toBe(WallSide.LEFT);
    expect(view.letter).toBe('');
  });
});
