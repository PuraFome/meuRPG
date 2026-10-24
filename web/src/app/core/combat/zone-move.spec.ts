import { SpellAreaShape } from '../../../gen/meurpg/rules/v1/rules_pb';
import { spellZone } from './combat-testing';
import { ZONE_MOVE_RANGE_FT, zoneMovePreview, zoneRangeText, zoneSquareSpoken } from './zone-move';

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
    expect(p.text.replace(/\u00a0/g, ' ')).toBe('4,5 m · 3 quadrados');
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

describe('the words of a zone move', () => {
  const zone = spellZone({ id: 'z', sizeFt: 5, origin: { col: 8, row: 6 } } as never);
  const plain = (t: string) => t.replace(/\u00a0/g, ' ');

  it('says "dentro do alcance" for a player in range and the refusal wording beyond it', () => {
    const near = zoneMovePreview(zone, { col: 10, row: 6 }, false);
    expect(plain(zoneRangeText(near, false))).toBe('Dentro do alcance, até 18 m.');
    const far = zoneMovePreview(zone, { col: 23, row: 6 }, false);
    expect(plain(zoneRangeText(far, false))).toBe(
      'Longe demais: a área anda até 18 m do ponto atual. Faltam 4,5 m.',
    );
  });

  it('has no range sentence for the master', () => {
    expect(zoneRangeText(zoneMovePreview(zone, { col: 40, row: 6 }, true), true)).toBe('');
  });

  it('speaks the square, the distance and whether it can be used', () => {
    const near = zoneMovePreview(zone, { col: 10, row: 6 }, false);
    expect(plain(zoneSquareSpoken({ col: 10, row: 6 }, near))).toBe(
      'Coluna 11, linha 7, a 3,0 m. Escolhido.',
    );
    const far = zoneMovePreview(zone, { col: 23, row: 6 }, false);
    expect(plain(zoneSquareSpoken({ col: 23, row: 6 }, far))).toContain('Longe demais');
  });
});
