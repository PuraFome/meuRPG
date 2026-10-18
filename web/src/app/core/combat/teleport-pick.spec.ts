import { CombatantKind } from '../../../gen/meurpg/play/v1/combat_pb';
import { NO_LAYERS } from '../maps/layers';
import { Sight, type Vision } from '../maps/vision';
import { combatant } from './combat-testing';
import {
  type TeleportScene,
  pickableSquares,
  squareSpoken,
  teleportJudge,
  verdictText,
} from './teleport-pick';

const plain = (t: string) => t.replace(/\u00a0/g, ' ');

const COLUMNS = 12;
const ROWS = 8;
const REACH_FT = 30;

const caster = combatant({
  id: 'p',
  label: 'Pensantus',
  kind: CombatantKind.PLAYER,
  col: 2,
  row: 4,
});

function scene(over: Partial<TeleportScene> = {}): TeleportScene {
  return {
    caster: { col: 2, row: 4 },
    reachFt: REACH_FT,
    master: false,
    casterId: 'p',
    combatants: [caster],
    layers: null,
    fog: null,
    columns: COLUMNS,
    rows: ROWS,
    ...over,
  };
}

function fogOf(seen: readonly { col: number; row: number }[]): Vision {
  const states = new Uint8Array(COLUMNS * ROWS);
  for (const s of seen) {
    states[s.row * COLUMNS + s.col] = Sight.Bright;
  }
  return {
    columns: COLUMNS,
    rows: ROWS,
    states,
    revision: 1,
    characterOnMap: true,
    fogEnabled: true,
    groupVision: true,
    tilesPath: '',
    tileSquares: 0,
    tiles: [],
  };
}

describe('teleportJudge', () => {
  it('lets a player pick a free square in reach, and refuses one beyond it (6 squares)', () => {
    const judge = teleportJudge(scene());
    expect(judge({ col: 8, row: 4 })).toBe('ok');
    expect(judge({ col: 9, row: 4 })).toBe('far');
  });

  it('refuses a creature the player sees and a wall, but not the caster own square', () => {
    const judge = teleportJudge(
      scene({
        combatants: [caster, combatant({ id: 'g', label: 'Goblin', col: 4, row: 4 })],
        layers: { ...NO_LAYERS, columns: COLUMNS, rows: ROWS, walls: [{ col: 3, row: 4 }] },
      }),
    );
    expect(judge({ col: 4, row: 4 })).toBe('unavailable');
    expect(judge({ col: 3, row: 4 })).toBe('unavailable');
    expect(judge({ col: 2, row: 4 })).toBe('ok');
  });

  it('does not count a defeated creature as taking the square', () => {
    const judge = teleportJudge(
      scene({
        combatants: [
          caster,
          combatant({ id: 'g', label: 'Goblin', col: 4, row: 4, defeated: true }),
        ],
      }),
    );
    expect(judge({ col: 4, row: 4 })).toBe('ok');
  });

  it('refuses a square the fog does not show as seen now', () => {
    const judge = teleportJudge(scene({ fog: fogOf([{ col: 3, row: 4 }]) }));
    expect(judge({ col: 3, row: 4 })).toBe('ok');
    expect(judge({ col: 4, row: 4 })).toBe('unavailable');
  });

  it('tells the far square from the one that is out of sight: reach first, as the server does', () => {
    const judge = teleportJudge(scene({ fog: fogOf([]) }));
    expect(judge({ col: 11, row: 4 })).toBe('far');
  });

  it('holds the master to nothing: any square is his to try', () => {
    const judge = teleportJudge(scene({ master: true, fog: fogOf([]) }));
    expect(judge({ col: 11, row: 7 })).toBe('ok');
    expect(pickableSquares(scene({ master: true }))).toEqual([]);
  });
});

describe('pickableSquares and the words', () => {
  it('lists only the squares a player can pick', () => {
    const squares = pickableSquares(
      scene({
        fog: fogOf([
          { col: 3, row: 4 },
          { col: 9, row: 4 },
        ]),
      }),
    );
    expect(squares).toEqual([{ col: 3, row: 4 }]);
  });

  it('uses the words of the server refusals', () => {
    expect(verdictText('far')).toBe('Longe demais');
    expect(verdictText('unavailable')).toBe('Esse espaço não está disponível');
    expect(verdictText('ok')).toBe('');
  });

  it('says the square, its distance and whether it was picked', () => {
    const from = { col: 2, row: 4 };
    expect(plain(squareSpoken({ col: 5, row: 4 }, from, 'ok'))).toBe(
      'Coluna 6, linha 5, a 4,5 m. Escolhido.',
    );
    expect(plain(squareSpoken({ col: 5, row: 4 }, from, 'far'))).toBe(
      'Coluna 6, linha 5, a 4,5 m. Longe demais.',
    );
  });
});
