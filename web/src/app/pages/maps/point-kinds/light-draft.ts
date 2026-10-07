import type { MapPoint } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { metersNumber } from '../../../core/units';
import type { LightOption } from '../../../core/maps/light-presets';
import type { PointChanges } from '../../../core/maps/maps-client';
import { POINT_DESCRIPTION_MAX, POINT_NAME_MAX } from '../point-panel/point-draft';

/** The custom light's radii are metres in whole squares of 1,5 m (5 ft), up to 120 ft (36 m): the server's limit. */
export const CUSTOM_KEY = '';
export const MAX_LIGHT_FT = 120;

/**
 * What the Luz form edits (E9-02, MR-036): the name, the description and the light, a preset of the SRD or
 * "Personalizada" with its two radii in metres (what the master types; feet and squares come from the metres).
 * Pure functions, tested without a DOM.
 */
export interface LightDraft {
  readonly name: string;
  readonly description: string;
  /** A preset's key ("light:torch"), or `''` for "Personalizada". */
  readonly presetKey: string;
  readonly brightM: string;
  readonly dimM: string;
}

/** The metres a person typed ("4,5", "4.5"), or `null` when it is not a number. */
export function parseMetres(text: string): number | null {
  const t = text.trim().replace(',', '.');
  return /^\d{1,3}(\.\d{1,2})?$/.test(t) ? Number(t) : null;
}

/** Feet for metres, when the metres are a whole number of squares (1,5 m each); else `null`. */
export function metresToFt(m: number | null): number | null {
  if (m === null) {
    return null;
  }
  const squares = m / 1.5;
  return Math.abs(squares - Math.round(squares)) < 1e-9 ? Math.round(squares) * 5 : null;
}

export function lightDraftOf(point: Pick<MapPoint, 'name' | 'description' | 'light'>, presets: readonly LightOption[]): LightDraft {
  const l = point.light;
  const known = l && presets.some((p) => p.key === l.presetKey);
  return {
    name: point.name,
    description: point.description,
    presetKey: known ? l.presetKey : CUSTOM_KEY,
    brightM: l ? metersNumber(l.brightFt) : '',
    dimM: l ? metersNumber(l.dimFt) : '',
  };
}

/** Choosing a preset fills the radii; "Personalizada" keeps what is typed. */
export function withPreset(d: LightDraft, key: string, presets: readonly LightOption[]): LightDraft {
  const p = presets.find((x) => x.key === key);
  if (!p || p.brightFt === undefined || p.dimFt === undefined) {
    return { ...d, presetKey: CUSTOM_KEY };
  }
  return { ...d, presetKey: key, brightM: metersNumber(p.brightFt), dimM: metersNumber(p.dimFt) };
}

export interface LightErrors {
  readonly name?: string;
  readonly description?: string;
  readonly brightM?: string;
  readonly dimM?: string;
  readonly both?: string;
}

const RADIUS = 'Use múltiplos de 1,5 m (um quadrado), de 0 a 36 m.';

export function lightErrors(d: LightDraft): LightErrors {
  const e: { -readonly [K in keyof LightErrors]: LightErrors[K] } = {};
  const name = d.name.trim();
  if (name === '') {
    e.name = 'Dê um nome ao ponto.';
  } else if ([...name].length > POINT_NAME_MAX) {
    e.name = `Use até ${POINT_NAME_MAX} caracteres.`;
  } else if (/\p{Cc}/u.test(name)) {
    e.name = 'Use um nome numa linha só.';
  }
  if ([...d.description].length > POINT_DESCRIPTION_MAX) {
    e.description = 'Use até 2.000 caracteres.';
  }
  const bright = metresToFt(parseMetres(d.brightM));
  const dim = metresToFt(parseMetres(d.dimM));
  if (bright === null || bright > MAX_LIGHT_FT) {
    e.brightM = RADIUS;
  }
  if (dim === null || dim > MAX_LIGHT_FT) {
    e.dimM = RADIUS;
  }
  if (bright === 0 && dim === 0) {
    e.both = 'Os dois raios não podem ser 0.';
  }
  return e;
}

export function hasLightErrors(e: LightErrors): boolean {
  return Object.keys(e).length > 0;
}

/** The radii in feet, when the form is valid. */
export function lightFt(d: LightDraft): { brightFt: number; dimFt: number } | null {
  const bright = metresToFt(parseMetres(d.brightM));
  const dim = metresToFt(parseMetres(d.dimM));
  return bright === null || dim === null ? null : { brightFt: bright, dimFt: dim };
}

/** The radii in whole squares, for the line under the fields ("4 quadrados de luz clara e mais 4 de penumbra"). */
export function lightSquares(d: LightDraft): { bright: number; dim: number } | null {
  const ft = lightFt(d);
  return ft ? { bright: ft.brightFt / 5, dim: ft.dimFt / 5 } : null;
}

export function isLightDirty(d: LightDraft, point: Pick<MapPoint, 'name' | 'description' | 'light'>, presets: readonly LightOption[]): boolean {
  return JSON.stringify(d) !== JSON.stringify(lightDraftOf(point, presets));
}

export function lightChangesOf(
  d: LightDraft,
  point: Pick<MapPoint, 'name' | 'description' | 'light'>,
  presets: readonly LightOption[],
): PointChanges | null {
  if (!isLightDirty(d, point, presets)) {
    return null;
  }
  const ft = lightFt(d);
  if (!ft) {
    return null;
  }
  const changes: { -readonly [K in keyof PointChanges]: PointChanges[K] } = {
    light: { presetKey: d.presetKey, brightFt: ft.brightFt, dimFt: ft.dimFt },
  };
  if (d.name.trim() !== point.name) {
    changes.name = d.name.trim();
  }
  if (d.description !== point.description) {
    changes.description = d.description;
  }
  return changes;
}
