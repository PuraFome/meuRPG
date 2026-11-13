import { create } from '@bufbuild/protobuf';

import {
  GetMoveOptionsResponseSchema,
  MoveRefusal,
  ReachableSquareSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant } from '../../../../core/combat/combat-testing';
import {
  type JumpScene,
  canMoveOf,
  cappedMovement,
  centreScroll,
  errorShown,
  frameOf,
  goIconOf,
  goLabelOf,
  jumpRequestOf,
  jumpSummary,
  keepInViewScroll,
  leadOf,
  reachOf,
  trapNameOf,
  walkSummary,
} from './move-page-view';

describe('move page view', () => {
  const origin = { col: 2, row: 2 };
  const scene = (over: Partial<JumpScene> = {}): JumpScene => ({
    kind: 'long',
    left: 300,
    leftHere: 300,
    height: 0,
    chosen: { col: 4, row: 2 },
    origin,
    verdict: null,
    cap: 160,
    limit: 160,
    cost: 30,
    combatants: [],
    trapName: '',
    ...over,
  });

  it('words the high jump, and refuses it when not a step fits', () => {
    expect(jumpSummary(scene({ kind: 'high', height: 60 })).title).toBe('Saltar 1,8\u00a0m');
    expect(jumpSummary(scene({ kind: 'high', height: 0 })).title).toBe('Sem salto em altura');
  });

  it('says what is left after a high jump from the counter of where the jumper stands, not from the floor', () => {
    // In water with a swim speed: the floor's lead (300) pays for the jump, the counter there (400) is what it is taken from.
    const detail = jumpSummary(scene({ kind: 'high', height: 60, leftHere: 400 })).detail;
    expect(detail.replace(/\u00a0/g, ' ')).toContain('Depois restam 10,2 m.');
  });

  it('says what is left after a long jump as the server counted it at the landing surface', () => {
    // The floor's lead is 300 and the jump costs 150, but it lands in water with a swim speed of 400: 250 are left.
    const square = create(ReachableSquareSchema, {
      col: 4,
      row: 2,
      costDft: 150,
      leftAfterDft: 250,
    });
    const detail = jumpSummary(
      scene({ cost: 150, cap: 160, verdict: { kind: 'ok', square } }),
    ).detail;
    expect(detail.replace(/\u00a0/g, ' ')).toContain('Depois restam 7,5 m.');
  });

  it('is idle until a landing is chosen, and refuses the square of the jumper', () => {
    expect(jumpSummary(scene({ chosen: null })).kind).toBe('idle');
    expect(jumpSummary(scene({ chosen: origin })).title).toBe('Você já está aqui');
  });

  it('refuses a wall, an occupied square, a creature standing there and a landing too far', () => {
    const wall = { kind: 'refused', reason: MoveRefusal.WALL } as const;
    const taken = { kind: 'refused', reason: MoveRefusal.OCCUPIED } as const;
    expect(jumpSummary(scene({ verdict: wall })).title).toBe('Sem caminho');
    expect(jumpSummary(scene({ verdict: taken })).title).toBe('Ocupado');
    const standing = combatant({ id: 'g', col: 4, row: 2, placed: true } as never);
    expect(jumpSummary(scene({ combatants: [standing] })).title).toBe('Ocupado');
    expect(jumpSummary(scene({ cost: 200 })).detail).toContain('Seu salto vai até');
    expect(jumpSummary(scene({ cost: 100, cap: 60 })).detail).toContain('Você só tem');
  });

  it('words a landing in reach, with the provoked and the trap', () => {
    const square = create(ReachableSquareSchema, {
      col: 4,
      row: 2,
      costDft: 30,
      provokesReactorIds: ['g'],
    });
    const g = combatant({ id: 'g', label: 'Goblin' });
    const ok = jumpSummary(
      scene({ verdict: { kind: 'ok', square }, combatants: [g], trapName: 'Fosso' }),
    );
    expect(ok.kind).toBe('ok');
    expect(ok.warning).toContain('Goblin');
    expect(ok.trap).toBe('Fosso');
  });

  it('is idle for a walk with nothing chosen or unknown, and words a cost and a refusal', () => {
    expect(walkSummary(null, null, 300, []).kind).toBe('idle');
    expect(walkSummary(origin, { kind: 'unknown' }, 300, []).kind).toBe('idle');
    const square = create(ReachableSquareSchema, { col: 3, row: 2, costDft: 71 });
    expect(walkSummary(origin, { kind: 'ok', square }, 300, []).title).toBe('Mover 2,1\u00a0m');
    expect(walkSummary(origin, { kind: 'refused', reason: MoveRefusal.WALL }, 300, []).kind).toBe(
      'refused',
    );
  });

  it('words the swim, the climb and the crawl that the server added to the cost', () => {
    const swum = create(ReachableSquareSchema, {
      col: 3,
      row: 2,
      costDft: 150,
      extraDft: 50,
      swimExtraDft: 50,
      pathDft: 100,
    });
    const summary = walkSummary(origin, { kind: 'ok', square: swum }, 150, []);
    expect(summary.detail.replace(/\u00a0/g, ' ')).toBe('Depois restam 0,0 m.');
    expect(summary.extra?.parts.map((p) => p.kind)).toEqual(['swim']);
    expect(summary.extra?.note).toBe(': cada metro na água conta dobrado.');
    // A prone creature in water: the card says the two parts, and the detail says the sum.
    const crawling = create(ReachableSquareSchema, {
      col: 3,
      row: 2,
      costDft: 150,
      extraDft: 50,
      swimExtraDft: 50,
      crawlExtraDft: 50,
      pathDft: 50,
      leftAfterDft: 150,
    });
    const both = walkSummary(origin, { kind: 'ok', square: crawling }, 300, []);
    expect(both.extra?.parts.map((p) => p.kind)).toEqual(['swim', 'crawl']);
    expect(both.detail.replace(/\u00a0/g, ' ')).toBe(
      '1,5 m do caminho + 1,5 m nadando + 1,5 m rastejando. Depois restam 4,5 m.',
    );
    const plain = create(ReachableSquareSchema, { col: 3, row: 2, costDft: 71 });
    expect(walkSummary(origin, { kind: 'ok', square: plain }, 300, []).extra).toBeUndefined();
  });

  it('says what is left after a square as the server counted it, never the lead less the cost', () => {
    // On the floor with 300 and a swim speed of 400: a square 150 away in water leaves 250 of the swim speed, not 300 - 150.
    const swum = create(ReachableSquareSchema, { col: 3, row: 2, costDft: 150, leftAfterDft: 250 });
    const detail = walkSummary(origin, { kind: 'ok', square: swum }, 300, []).detail;
    expect(detail.replace(/\u00a0/g, ' ')).toBe('Depois restam 7,5 m.');
  });

  it('frames the chosen square by what the move is', () => {
    const idle = walkSummary(null, null, 300, []);
    expect(frameOf(null, idle, false, 'long', null, 0)).toBeNull();
    expect(frameOf(origin, idle, true, 'high', null, 0)).toBeNull();
    expect(frameOf(origin, idle, false, 'long', { kind: 'unknown' }, 0)?.refused).toBe(false);
    const refusal = {
      kind: 'refused',
      title: 'Sem caminho reto',
      detail: '',
      warning: '',
      trap: '',
    } as const;
    expect(frameOf(origin, refusal, false, 'long', null, 0)).toMatchObject({
      refused: true,
      label: 'Sem caminho',
    });
    const ok = { kind: 'ok', title: 'x', detail: '', warning: 'w', trap: '' } as const;
    expect(frameOf(origin, ok, true, 'long', null, 30)).toMatchObject({
      label: '0,9\u00a0m',
      note: 'Cai fora do alcance',
    });
  });

  it('words the lead, the button and its icon', () => {
    expect(leadOf(true, 'high', 300, 300, 0)).not.toContain('Toque');
    expect(leadOf(true, 'long', 300, 300, 0)).toContain('onde quer cair');
    expect(leadOf(false, 'long', 300, 300, 30)).toContain('Você já andou');
    expect(goLabelOf(false, 'long', false, 0)).toBe('Mover para cá');
    expect(goLabelOf(true, 'long', true, 0)).toBe('Saltar e desengajar');
    expect(goLabelOf(true, 'high', false, 30)).toBe('Saltar 0,9\u00a0m para cima');
    expect(goIconOf(false, 'long')).toBe('arrow_forward');
    expect(goIconOf(true, 'high')).toBe('arrow_upward');
    expect(goIconOf(true, 'long')).toBe('north_east');
  });

  it('draws the reach of a walk, the circle of a long jump, and none for a high jump or a read-only page', () => {
    const options = create(GetMoveOptionsResponseSchema, {
      reachable: [create(ReachableSquareSchema, { col: 3, row: 2, costDft: 10 })],
    });
    expect(reachOf(true, false, 'long', origin, 160, 300, options)).toBeNull();
    expect(reachOf(false, true, 'high', origin, 160, 300, options)).toBeNull();
    expect(reachOf(false, true, 'long', origin, 160, 300, options)).toMatchObject({ leftDft: 160 });
    expect(reachOf(false, false, 'long', origin, 160, 300, options)?.squares).toEqual([
      { col: 3, row: 2 },
    ]);
    expect(reachOf(false, false, 'long', origin, 160, 300, null)?.squares).toEqual([]);
  });

  it('lets the button work by what is being done', () => {
    const idle = 'idle' as const;
    expect(canMoveOf(true, false, 'long', 0, null, idle, { kind: 'ok' } as never)).toBe(false);
    expect(canMoveOf(false, true, 'high', 0, null, idle, null)).toBe(false);
    expect(canMoveOf(false, true, 'high', 30, null, idle, null)).toBe(true);
    expect(canMoveOf(false, true, 'long', 0, origin, 'ok', null)).toBe(true);
    expect(canMoveOf(false, true, 'long', 0, null, 'ok', null)).toBe(false);
    expect(canMoveOf(false, false, 'long', 0, null, idle, { kind: 'unknown' })).toBe(true);
    expect(canMoveOf(false, false, 'long', 0, null, idle, { kind: 'beyond' })).toBe(false);
  });

  it('shows a server error only for the square it came for', () => {
    expect(errorShown('', null, null)).toBe(false);
    expect(errorShown('x', null, null)).toBe(true);
    expect(errorShown('x', origin, { ...origin })).toBe(true);
    expect(errorShown('x', origin, { col: 9, row: 9 })).toBe(false);
    expect(errorShown('x', origin, null)).toBe(false);
  });

  it('builds the jump request and finds the trap', () => {
    expect(jumpRequestOf('high', 30, null, false)).toEqual({ kind: 'high', heightDft: 30 });
    expect(jumpRequestOf('long', 0, null, false)).toBeNull();
    expect(jumpRequestOf('long', 0, origin, true)).toEqual({
      kind: 'long',
      square: origin,
      disengage: true,
    });
    expect(jumpRequestOf('long', 0, origin, false)).toEqual({ kind: 'long', square: origin });
    expect(
      trapNameOf(
        false,
        'long',
        origin,
        { reachable: new Map(), refused: new Map(), known: true },
        null,
      ),
    ).toBe('');
    expect(
      trapNameOf(
        true,
        'high',
        origin,
        { reachable: new Map(), refused: new Map(), known: true },
        null,
      ),
    ).toBe('');
  });

  it('caps the movement by the live one, and scrolls the map', () => {
    expect(cappedMovement(undefined, undefined)).toBe(0);
    expect(cappedMovement(undefined, 40)).toBe(40);
    expect(cappedMovement(90, undefined)).toBe(90);
    expect(cappedMovement(90, 40)).toBe(40);
    const box = { left: 0, top: 0, width: 100, height: 100 };
    expect(centreScroll({ col: 10, row: 10 }, 30, box)).toEqual({ left: 265, top: 265 });
    expect(centreScroll({ col: 0, row: 0 }, 30, box)).toEqual({ left: 0, top: 0 });
    expect(keepInViewScroll({ col: 0, row: 0 }, 30, { ...box, left: 50, top: 50 })).toEqual({
      left: 0,
      top: 0,
    });
    expect(keepInViewScroll({ col: 10, row: 10 }, 30, box)).toEqual({ left: 275, top: 275 });
    expect(keepInViewScroll({ col: 1, row: 1 }, 30, { ...box, width: 300, height: 300 })).toEqual({
      left: 0,
      top: 0,
    });
  });
});
