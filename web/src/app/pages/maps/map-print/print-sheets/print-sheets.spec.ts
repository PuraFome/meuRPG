import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import type { MapImage } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import { PAPERS, planFor } from '../print-math';
import { PrintSheets } from './print-sheets';

interface SheetView {
  key: string;
  label: string;
  imageLeft: number;
  imageTop: number;
  grid: string;
  bands: { x: number; y: number; w: number; h: number }[];
  marks: string;
  crosses: string;
  above: string | null;
  below: string | null;
  left: string | null;
  right: string | null;
}

describe('PrintSheets, the tiles it draws', () => {
  function sheetsOf(width: number, height: number, squareCm = 2.54): SheetView[] {
    const fixture = TestBed.createComponent(PrintSheets);
    const size = { width, height };
    fixture.componentRef.setInput('plan', planFor(PAPERS[0], 'portrait', size));
    fixture.componentRef.setInput('image', { id: 'i1' } as MapImage);
    fixture.componentRef.setInput('mapSize', size);
    fixture.componentRef.setInput('squareCm', squareCm);
    return (fixture.componentInstance as unknown as { sheets(): SheetView[] }).sheets();
  }

  it('is one sheet with no band, no neighbour and no cross when the map fits the paper', () => {
    const [only, ...rest] = sheetsOf(10, 10);
    expect(rest).toEqual([]);
    expect(only).toMatchObject({
      key: 'A1',
      imageLeft: -0,
      imageTop: -0,
      bands: [],
      marks: '',
      crosses: '',
      above: null,
      below: null,
      left: null,
      right: null,
    });
    expect(only.grid).toMatch(/^M0 0V10/);
  });

  it('tints the 1 cm it shares with each neighbour, and names the neighbours', () => {
    const sheets = sheetsOf(60, 80);
    const columns = new Set(sheets.map((s) => s.key[1])).size;
    const rows = new Set(sheets.map((s) => s.key[0])).size;
    expect(columns).toBeGreaterThan(2);
    expect(rows).toBeGreaterThan(2);
    const middle = sheets.find((s) => s.key === 'B2');
    expect(middle).toBeDefined();
    expect(middle).toMatchObject({ above: 'A2', below: 'C2', left: 'B1', right: 'B3' });
    expect(middle?.bands).toHaveLength(4);
    // One dashed edge per band, and a cross at each of the four inner corners.
    expect(middle?.marks.split('M').length).toBe(5);
    expect((middle?.crosses.split('M').length ?? 0) - 1).toBe(8);
    const corner = sheets[0];
    expect(corner).toMatchObject({ above: null, left: null, below: 'B1', right: 'A2' });
    expect(corner.bands).toHaveLength(2);
    const last = sheets[sheets.length - 1];
    expect(last).toMatchObject({ below: null, right: null });
    expect(last.imageLeft).toBeLessThan(0);
    expect(last.imageTop).toBeLessThan(0);
  });

  it('labels each sheet with its place and the grid at the square size, trimmed to 3 decimals', () => {
    const sheets = sheetsOf(30, 10, 2.5);
    expect(sheets.map((s) => s.label).length).toBe(sheets.length);
    expect(sheets[0].grid).toContain('M2.5 0V');
    expect(sheets[0].bands.length).toBe(1);
    expect(sheets[0].right).toBe('A2');
    expect(sheets[1].left).toBe('A1');
  });
});
