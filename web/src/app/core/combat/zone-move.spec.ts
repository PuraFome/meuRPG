import { SpellAreaShape } from '../../../gen/meurpg/rules/v1/rules_pb';
import { spellZone } from './combat-testing';
import { ZONE_MOVE_RANGE_FT, zoneMovePreview } from './zone-move';

describe('the preview of moving a zone', () => {
  const moonbeam = spellZone({ id: 'z', sizeFt: 5, origin: { col: 8, row: 6 } } as never);

  it('keeps the server range: 60 ft, 18 m', () => {
    expect(ZONE_MOVE_RANGE_FT).toBe(60);
  });

  it('draws the new area around the chosen square, the square included', () => {
    const p = zoneMovePreview(moonbeam, { col: 10, row: 6 }, false);
    expect(p.squares).toHaveLength(5);
    expect(p.squares).toContainEqual({ col: 10, row: 6 });
    expect(p.squares).toContainEqual({ col: 11, row: 6 });
  });

  it('measures the distance from the current origin in feet and squares', () => {
    const p = zoneMovePreview(moonbeam, { col: 11, row: 6 }, false);
    expect(p.distanceFt).toBe(15);
    expect(p.text).toBe('4,5 m · 3 quadrados');
  });

  it('is in range up to 60 ft and out of range beyond, for a player', () => {
    expect(zoneMovePreview(moonbeam, { col: 20, row: 6 }, false).inRange).toBe(true);
    const far = zoneMovePreview(moonbeam, { col: 21, row: 6 }, false);
    expect(far.inRange).toBe(false);
    expect(far.missingFt).toBe(5);
  });

  it('has no limit for the master', () => {
    const far = zoneMovePreview(moonbeam, { col: 40, row: 6 }, true);
    expect(far.inRange).toBe(true);
    expect(far.missingFt).toBe(0);
  });

  it('draws a sphere like a cylinder, and nothing for a shape that is not centered', () => {
    const sphere = spellZone({ id: 's', shape: SpellAreaShape.SPHERE, sizeFt: 10 } as never);
    expect(zoneMovePreview(sphere, { col: 5, row: 5 }, true).squares).toHaveLength(13);
    const cube = spellZone({ id: 'c', shape: SpellAreaShape.CUBE, sizeFt: 20 } as never);
    expect(zoneMovePreview(cube, { col: 5, row: 5 }, true).squares).toEqual([]);
  });

  it('says it does not cut the area at walls (the server does on commit)', () => {
    expect(zoneMovePreview(moonbeam, { col: 9, row: 6 }, true).wallCut).toBe(false);
  });
});
