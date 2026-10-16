import { describe, expect, it } from 'vitest';

import { MapPointKind } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { defaultName, newPointBody } from './new-point';

describe('new point', () => {
  const at = { xBp: 10, yBp: 20 };
  const presets = {
    lights: { list: async () => [{ key: 'light:torch' }] },
    traps: { list: async () => ({ presets: [{ findDc: 14 }], severities: [] }) },
  } as never;
  const none = {
    lights: { list: async () => [] },
    traps: { list: async () => ({ presets: [], severities: [] }) },
  } as never;

  it('names each kind', () => {
    expect(
      [
        MapPointKind.BATTLE,
        MapPointKind.SUBMAP,
        MapPointKind.LIGHT,
        MapPointKind.TRAP,
        MapPointKind.TREASURE,
        MapPointKind.SCENE,
      ].map(defaultName),
    ).toEqual([
      'Nova batalha',
      'Novo submapa',
      'Nova luz',
      'Nova armadilha',
      'Novo tesouro',
      'Nova cena',
    ]);
  });

  it('gives a light, a trap and a treasure the start the server takes', async () => {
    expect(await newPointBody(MapPointKind.LIGHT, at, 'c', presets)).toMatchObject({
      light: { presetKey: 'light:torch' },
    });
    expect(await newPointBody(MapPointKind.TRAP, at, 'c', presets)).toMatchObject({
      trap: { findDc: 14, areaSize: 1 },
    });
    expect(await newPointBody(MapPointKind.TREASURE, at, 'c', presets)).toMatchObject({
      treasureValuePo: 0,
    });
    const scene = await newPointBody(MapPointKind.SCENE, at, 'c', presets);
    expect(scene).toEqual({
      kind: MapPointKind.SCENE,
      name: 'Nova cena',
      description: '',
      xBp: 10,
      yBp: 20,
    });
  });

  it('falls back to an empty preset and a DC of 10 when the server lists none', async () => {
    expect(await newPointBody(MapPointKind.LIGHT, at, 'c', none)).toMatchObject({
      light: { presetKey: '' },
    });
    expect(await newPointBody(MapPointKind.TRAP, at, 'c', none)).toMatchObject({
      trap: { findDc: 10 },
    });
  });
});
