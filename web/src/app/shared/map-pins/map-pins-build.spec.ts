import { describe, expect, it } from 'vitest';

import { MapPointKind, TrapState } from '../../../gen/meurpg/maps/v1/maps_pb';
import { mapPoint } from '../../core/maps/maps-testing';
import { buildPins } from './map-pins-build';

describe('buildPins', () => {
  const view = { isMaster: true, columns: 10, rows: 10 };
  const trap = (over: Parameters<typeof mapPoint>[2] = {}) =>
    mapPoint('t', 'Fosso', {
      kind: MapPointKind.TRAP,
      revealed: true,
      xBp: 5000,
      yBp: 5000,
      ...over,
    });

  it('names the state of a trap: armed by default, fired or disarmed as saved', () => {
    const state = (s?: TrapState) =>
      buildPins([trap({ trap: s === undefined ? undefined : ({ state: s } as never) })], view)[0]
        .state;
    expect(state()).toBe('armed');
    expect(state(TrapState.TRIGGERED)).toBe('fired');
    expect(state(TrapState.DISARMED)).toBe('disarmed');
  });

  it('marks a hidden armed trap as secret for the master only, and gives a player nothing hidden', () => {
    const hidden = trap({ revealed: false });
    expect(buildPins([hidden], view)[0].secret).toBe(true);
    expect(buildPins([hidden], { ...view, isMaster: false })).toEqual([]);
  });

  it('draws a treasure by whether it was found, a light for the master only, and no other kind', () => {
    const chest = mapPoint('c', 'Baú', { kind: MapPointKind.TREASURE, revealed: true });
    const lamp = mapPoint('l', 'Luz', { kind: MapPointKind.LIGHT, revealed: true });
    const scene = mapPoint('s', 'Cena', { kind: MapPointKind.SCENE, revealed: true });
    expect(buildPins([chest, lamp, scene], view).map((p) => [p.kind, p.state])).toEqual([
      ['treasure', 'hidden'],
      ['light', ''],
    ]);
    expect(
      buildPins([chest, lamp, scene], { ...view, isMaster: false }).map((p) => p.kind),
    ).toEqual(['treasure']);
  });

  it('numbers the marks that share a square, even past a light a player does not see', () => {
    const at = { revealed: true, xBp: 2000, yBp: 2000 };
    const points = [
      mapPoint('a', 'A', { kind: MapPointKind.TREASURE, ...at }),
      mapPoint('b', 'B', { kind: MapPointKind.LIGHT, ...at }),
      mapPoint('c', 'C', { kind: MapPointKind.TREASURE, ...at }),
    ];
    expect(buildPins(points, { ...view, isMaster: false }).map((p) => p.n)).toEqual([0, 2]);
    expect(buildPins(points, view).map((p) => p.n)).toEqual([0, 1, 2]);
  });
});
