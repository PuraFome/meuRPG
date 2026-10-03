import {
  labelBounds,
  IDENTITY,
  bpToPercent,
  centroid,
  clampBp,
  clampTransform,
  focusTransform,
  labelShift,
  labelSide,
  nudge,
  scaleLabel,
  screenToBp,
  stepScale,
  tokenInitial,
  visiblePoints,
  visibleTokens,
  zoomAround,
} from './map-geometry';

const rect = { left: 100, top: 50, width: 800, height: 400 };
const token = (characterId: string, name: string, hidden = false) => ({
  characterId,
  name,
  mine: false,
  xBp: 0,
  yBp: 0,
  hidden,
});

describe('positions', () => {
  it('turns basis points into percentages', () => {
    expect(bpToPercent(5000)).toBe(50);
    expect(bpToPercent(3750)).toBe(37.5);
  });

  it('keeps a position inside the image, in whole basis points', () => {
    expect(clampBp(-20)).toBe(0);
    expect(clampBp(10500)).toBe(10000);
    expect(clampBp(1234.6)).toBe(1235);
  });

  it('reads a click as basis points of the image, with no pan or zoom', () => {
    expect(screenToBp(500, 250, rect, IDENTITY)).toEqual({ xBp: 5000, yBp: 5000 });
    expect(screenToBp(100, 50, rect, IDENTITY)).toEqual({ xBp: 0, yBp: 0 });
    expect(screenToBp(2000, 2000, rect, IDENTITY)).toEqual({ xBp: 10000, yBp: 10000 });
  });

  it('reads a click through a pan and a zoom', () => {
    // At 2x, with the stage shifted 400px left, the viewport's left edge
    // shows the image's 400/800/2 = 25 % mark.
    const t = { scale: 2, x: -400, y: 0 };
    expect(screenToBp(100, 50, rect, t)).toEqual({ xBp: 2500, yBp: 0 });
    expect(screenToBp(900, 450, rect, t)).toEqual({ xBp: 7500, yBp: 5000 });
  });
});

describe('zoom and pan', () => {
  it('keeps the image covering the viewport', () => {
    expect(clampTransform({ scale: 2, x: 50, y: 50 }, 800, 400)).toEqual({ scale: 2, x: 0, y: 0 });
    expect(clampTransform({ scale: 2, x: -5000, y: -5000 }, 800, 400)).toEqual({
      scale: 2,
      x: -800,
      y: -400,
    });
    expect(clampTransform({ scale: 1, x: -30, y: 10 }, 800, 400)).toEqual(IDENTITY);
  });

  it('zooms around the point under the pointer', () => {
    const t = zoomAround(IDENTITY, 2, 400, 200, 800, 400);
    expect(t.scale).toBe(2);
    // The image point under (400, 200) is still under it.
    expect(screenToBp(500, 250, rect, t)).toEqual({ xBp: 5000, yBp: 5000 });
  });

  it('never zooms out of 100 % to 400 %', () => {
    expect(zoomAround(IDENTITY, 0.5, 0, 0, 800, 400).scale).toBe(1);
    expect(zoomAround(IDENTITY, 9, 0, 0, 800, 400).scale).toBe(4);
  });

  it('steps in 25 % from 100 % to 400 %', () => {
    expect(stepScale(1, 1)).toBe(1.25);
    expect(stepScale(1, -1)).toBe(1);
    expect(stepScale(3.9, 1)).toBe(4);
    expect(stepScale(4, 1)).toBe(4);
    expect(stepScale(1.25, -1)).toBe(1);
  });

  it('says the zoom as a percentage', () => {
    expect(scaleLabel(1.25).replace(' ', ' ')).toBe('125 %');
  });

  it('puts a centre in the middle of the viewport', () => {
    const t = focusTransform({ xBp: 5000, yBp: 5000 }, 2, 800, 400);
    expect(t).toEqual({ scale: 2, x: -400, y: -200 });
    // Near a corner it stops at the edge.
    expect(focusTransform({ xBp: 0, yBp: 0 }, 2, 800, 400)).toEqual({ scale: 2, x: 0, y: 0 });
  });

  it('centres on the average of what is shown', () => {
    expect(centroid([])).toBeNull();
    expect(
      centroid([
        { xBp: 1000, yBp: 2000 },
        { xBp: 3000, yBp: 4000 },
      ]),
    ).toEqual({ xBp: 2000, yBp: 3000 });
  });
});

describe('keyboard moves', () => {
  it('moves 0,5 % per arrow, and 5 % with Shift', () => {
    expect(nudge('ArrowRight', false, 5000, 5000)).toEqual({ xBp: 5050, yBp: 5000 });
    expect(nudge('ArrowLeft', false, 5000, 5000)).toEqual({ xBp: 4950, yBp: 5000 });
    expect(nudge('ArrowDown', true, 5000, 5000)).toEqual({ xBp: 5000, yBp: 5500 });
    expect(nudge('ArrowUp', true, 5000, 5000)).toEqual({ xBp: 5000, yBp: 4500 });
  });

  it('stops at the edges and ignores other keys', () => {
    expect(nudge('ArrowLeft', true, 200, 0)).toEqual({ xBp: 0, yBp: 0 });
    expect(nudge('ArrowDown', true, 0, 9800)).toEqual({ xBp: 0, yBp: 10000 });
    expect(nudge('a', false, 5000, 5000)).toBeNull();
  });
});

describe('labels and initials', () => {
  it('uses the first letter, and two when two tokens share it', () => {
    const all = [token('1', 'Pensantus'), token('2', 'Brisa'), token('3', 'Toren')];
    expect(tokenInitial(all[0], all)).toBe('P');
    const clash = [token('1', 'Brisa'), token('2', 'Boris'), token('3', 'Toren')];
    expect(tokenInitial(clash[0], clash)).toBe('Br');
    expect(tokenInitial(clash[1], clash)).toBe('Bo');
    expect(tokenInitial(clash[2], clash)).toBe('T');
  });

  it('keeps a lone letter when the name has one', () => {
    const all = [token('1', 'B'), token('2', 'Brisa')];
    expect(tokenInitial(all[0], all)).toBe('B');
  });

  it('puts the label under the marker, or beside it at the edges', () => {
    expect(labelSide(5000, 5000)).toBe('below');
    expect(labelSide(9200, 5000)).toBe('left');
    expect(labelSide(500, 5000)).toBe('right');
    expect(labelSide(5000, 9500)).toBe('above');
  });

  it('slides a label back onto the image, and leaves one that fits alone', () => {
    // A 170px label centred 77px from the left of a 310px map (E5-29 on a phone).
    expect(labelShift(-8, 162, 4, 306)).toBe(12);
    expect(labelShift(200, 330, 4, 306)).toBe(-24);
    expect(labelShift(40, 210, 4, 306)).toBe(0);
    // Wider than the map: it starts at the left edge.
    expect(labelShift(-50, 350, 4, 306)).toBe(54);
  });
});

describe('what each person sees', () => {
  const points = [
    { id: 'a', name: 'A', kind: 1, xBp: 0, yBp: 0, revealed: true },
    { id: 'b', name: 'B', kind: 1, xBp: 0, yBp: 0, revealed: false },
  ];
  const tokens = [token('1', 'Pensantus'), token('2', 'Capitão Goblin', true)];

  it('gives the master everything', () => {
    expect(visiblePoints(points, true)).toHaveLength(2);
    expect(visibleTokens(tokens, true)).toHaveLength(2);
  });

  it('gives a player only the revealed points and the visible tokens', () => {
    expect(visiblePoints(points, false).map((p) => p.id)).toEqual(['a']);
    expect(visibleTokens(tokens, false).map((t) => t.characterId)).toEqual(['1']);
  });
});

describe('labelBounds', () => {
  it('keeps a label inside the image and inside what the viewport shows of it', () => {
    // A zoomed preview: the image runs past both sides of the viewport.
    expect(labelBounds({ left: -120, right: 700 }, { left: 0, right: 350 })).toEqual({ minLeft: 4, maxRight: 346 });
    // A small map in a big viewport: the image's own edges.
    expect(labelBounds({ left: 40, right: 300 }, { left: 0, right: 500 })).toEqual({ minLeft: 44, maxRight: 296 });
  });

  it('slides a label that sticks out of the preview back into it', () => {
    const { minLeft, maxRight } = labelBounds({ left: -120, right: 700 }, { left: 0, right: 350 });
    // "A carroça tombada" 110px wide, starting 300px from the left: it would be cut at 350.
    expect(labelShift(300, 410, minLeft, maxRight)).toBe(-64);
  });
});
