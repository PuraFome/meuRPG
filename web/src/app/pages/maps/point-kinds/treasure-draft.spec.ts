import { describe, expect, it } from 'vitest';

import {
  hasTreasureErrors,
  isTreasureDirty,
  parsePo,
  treasureChangesOf,
  treasureDraftOf,
  treasureErrors,
} from './treasure-draft';

const point = {
  name: 'Baú de moedas',
  description: '250 PO e uma adaga de prata.',
  treasureValuePo: 250,
};

describe('the Tesouro form', () => {
  it('shows the saved value as a number of PO', () => {
    expect(treasureDraftOf(point)).toEqual({
      name: 'Baú de moedas',
      description: '250 PO e uma adaga de prata.',
      valuePo: '250',
    });
  });

  it('reads whole PO, with or without the thousands dot, 0 to 1.000.000', () => {
    expect(parsePo('250')).toBe(250);
    expect(parsePo('1.000')).toBe(1000);
    expect(parsePo('1000000')).toBe(1_000_000);
    expect(parsePo('1000001')).toBeNull();
    expect(parsePo('-1')).toBeNull();
    expect(parsePo('2,5')).toBeNull();
    expect(parsePo('')).toBeNull();
  });

  it('says what is wrong', () => {
    const e = treasureErrors({ name: '', description: '', valuePo: 'dez' });
    expect(e.name).toBe('Dê um nome ao ponto.');
    expect(e.valuePo).toContain('0 a 1.000.000');
    expect(hasTreasureErrors(e)).toBe(true);
    expect(hasTreasureErrors(treasureErrors(treasureDraftOf(point)))).toBe(false);
  });

  it('saves only what changed: the value as a number', () => {
    const d = { ...treasureDraftOf(point), valuePo: '300' };
    expect(isTreasureDirty(d, point)).toBe(true);
    expect(treasureChangesOf(d, point)).toEqual({ treasureValuePo: 300 });
    expect(treasureChangesOf(treasureDraftOf(point), point)).toBeNull();
    expect(treasureChangesOf({ ...d, name: ' Baú ' }, point)).toEqual({
      name: 'Baú',
      treasureValuePo: 300,
    });
  });
});
