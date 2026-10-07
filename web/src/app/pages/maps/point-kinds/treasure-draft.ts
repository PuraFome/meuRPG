import type { MapPoint } from '../../../../gen/meurpg/maps/v1/maps_pb';
import type { PointChanges } from '../../../core/maps/maps-client';
import { POINT_DESCRIPTION_MAX, POINT_NAME_MAX } from '../point-panel/point-draft';

export const MAX_TREASURE_PO = 1_000_000;

/** What the Tesouro form edits (E9-02, MR-041): the name, what is inside, and the value in whole PO, as typed. */
export interface TreasureDraft {
  readonly name: string;
  readonly description: string;
  readonly valuePo: string;
}

export function treasureDraftOf(
  point: Pick<MapPoint, 'name' | 'description' | 'treasureValuePo'>,
): TreasureDraft {
  return {
    name: point.name,
    description: point.description,
    valuePo: String(point.treasureValuePo),
  };
}

/** The value as a number: `.` or space as a thousands separator is ignored ("1.000" is 1000). */
export function parsePo(text: string): number | null {
  const t = text.trim().replace(/[.\s]/g, '');
  if (!/^\d{1,7}$/.test(t)) {
    return null;
  }
  const n = Number(t);
  return n <= MAX_TREASURE_PO ? n : null;
}

export interface TreasureErrors {
  readonly name?: string;
  readonly description?: string;
  readonly valuePo?: string;
}

export function treasureErrors(d: TreasureDraft): TreasureErrors {
  const e: { -readonly [K in keyof TreasureErrors]: TreasureErrors[K] } = {};
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
  if (parsePo(d.valuePo) === null) {
    e.valuePo = 'Use um número inteiro de 0 a 1.000.000.';
  }
  return e;
}

export function hasTreasureErrors(e: TreasureErrors): boolean {
  return Object.keys(e).length > 0;
}

export function isTreasureDirty(
  d: TreasureDraft,
  point: Pick<MapPoint, 'name' | 'description' | 'treasureValuePo'>,
): boolean {
  const before = treasureDraftOf(point);
  return (
    d.name !== before.name ||
    d.description !== before.description ||
    parsePo(d.valuePo) !== point.treasureValuePo
  );
}

export function treasureChangesOf(
  d: TreasureDraft,
  point: Pick<MapPoint, 'name' | 'description' | 'treasureValuePo'>,
): PointChanges | null {
  if (!isTreasureDirty(d, point)) {
    return null;
  }
  const changes: { -readonly [K in keyof PointChanges]: PointChanges[K] } = {};
  if (d.name.trim() !== point.name) {
    changes.name = d.name.trim();
  }
  if (d.description !== point.description) {
    changes.description = d.description;
  }
  const value = parsePo(d.valuePo);
  if (value !== null && value !== point.treasureValuePo) {
    changes.treasureValuePo = value;
  }
  return Object.keys(changes).length > 0 ? changes : null;
}
