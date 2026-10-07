import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import { MapPointSchema } from '../../../../gen/meurpg/maps/v1/maps_pb';
import type { LightOption } from '../../../core/maps/light-presets';
import {
  CUSTOM_KEY,
  hasLightErrors,
  isLightDirty,
  lightChangesOf,
  lightDraftOf,
  lightErrors,
  lightFt,
  lightSquares,
  metresToFt,
  parseMetres,
  withPreset,
} from './light-draft';

const presets: LightOption[] = [
  {
    key: 'light:torch',
    name: 'Tocha',
    radii: '6 m claro + 6 m de penumbra',
    brightFt: 20,
    dimFt: 20,
  },
  {
    key: 'light:daylight',
    name: 'Luz do Dia',
    radii: '18 m claro + 18 m de penumbra',
    brightFt: 60,
    dimFt: 60,
  },
];

const point = (light: { presetKey: string; brightFt: number; dimFt: number }) =>
  create(MapPointSchema, { id: 'l1', name: 'Tocha da guarita', light });

describe('metres', () => {
  it('reads a comma or a dot, and only whole squares of 1,5 m turn into feet (5 ft each)', () => {
    expect(parseMetres('4,5')).toBe(4.5);
    expect(parseMetres('4.5')).toBe(4.5);
    expect(parseMetres('abc')).toBeNull();
    expect(metresToFt(4.5)).toBe(15);
    expect(metresToFt(6)).toBe(20);
    expect(metresToFt(0)).toBe(0);
    expect(metresToFt(1)).toBeNull();
    expect(metresToFt(null)).toBeNull();
  });
});

describe('the light form', () => {
  it('shows a saved preset as its radio, with the radii in metres', () => {
    const d = lightDraftOf(point({ presetKey: 'light:torch', brightFt: 20, dimFt: 20 }), presets);
    expect(d).toMatchObject({ presetKey: 'light:torch', brightM: '6', dimM: '6' });
    expect(
      isLightDirty(d, point({ presetKey: 'light:torch', brightFt: 20, dimFt: 20 }), presets),
    ).toBe(false);
  });

  it('shows a custom light as "Personalizada"', () => {
    const d = lightDraftOf(point({ presetKey: '', brightFt: 15, dimFt: 15 }), presets);
    expect(d).toMatchObject({ presetKey: CUSTOM_KEY, brightM: '4,5', dimM: '4,5' });
  });

  it('choosing a preset fills the radii; "Personalizada" keeps what was typed', () => {
    const d = lightDraftOf(point({ presetKey: 'light:torch', brightFt: 20, dimFt: 20 }), presets);
    const day = withPreset(d, 'light:daylight', presets);
    expect(day).toMatchObject({ presetKey: 'light:daylight', brightM: '18', dimM: '18' });
    expect(withPreset(day, '', presets)).toMatchObject({
      presetKey: CUSTOM_KEY,
      brightM: '18',
      dimM: '18',
    });
  });

  it('says how many squares the radii are', () => {
    const d = lightDraftOf(point({ presetKey: 'light:torch', brightFt: 20, dimFt: 20 }), presets);
    expect(lightSquares(d)).toEqual({ bright: 4, dim: 4 });
    expect(lightFt(d)).toEqual({ brightFt: 20, dimFt: 20 });
  });

  it('refuses a radius that is not a whole square, over 36 m, or both radii at 0', () => {
    const base = lightDraftOf(point({ presetKey: '', brightFt: 15, dimFt: 15 }), presets);
    expect(hasLightErrors(lightErrors(base))).toBe(false);
    expect(lightErrors({ ...base, brightM: '4' }).brightM).toContain('múltiplos de 1,5 m');
    expect(lightErrors({ ...base, dimM: '37,5' }).dimM).toContain('0 a 36 m');
    expect(lightErrors({ ...base, brightM: '0', dimM: '0' }).both).toBe(
      'Os dois raios não podem ser 0.',
    );
    expect(lightErrors({ ...base, name: ' ' }).name).toBe('Dê um nome ao ponto.');
  });

  it('saves the preset key with the radii in feet, and the name only when it changed', () => {
    const p = point({ presetKey: 'light:torch', brightFt: 20, dimFt: 20 });
    const d = withPreset(lightDraftOf(p, presets), 'light:daylight', presets);
    expect(lightChangesOf(d, p, presets)).toEqual({
      light: { presetKey: 'light:daylight', brightFt: 60, dimFt: 60 },
    });
    expect(lightChangesOf({ ...d, name: ' Sol ' }, p, presets)?.name).toBe('Sol');
    expect(lightChangesOf(lightDraftOf(p, presets), p, presets)).toBeNull();
  });
});
