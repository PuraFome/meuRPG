import {
  DungeonCorridorStyle,
  DungeonDoorMix,
  DungeonMask,
} from '../../../gen/meurpg/maps/v1/dungeons_pb';
import {
  DEFAULT_FORM,
  type DungeonForm,
  formProblems,
  optionsOf,
  parseSeed,
  shortSideOf,
  sideOf,
  wholeNumber,
} from './dungeon-options';

const form = (partial: Partial<DungeonForm>): DungeonForm => ({ ...DEFAULT_FORM, ...partial });

describe('the dungeon options form', () => {
  it('starts as the artboard does: Pequena, no shape, rooms of 3 to 9, winding corridors, common doors, 60 % dead ends, 2 stairs', () => {
    expect(optionsOf(DEFAULT_FORM)).toEqual({
      width: 31,
      height: 21,
      mask: DungeonMask.NONE,
      roomSideMin: 3,
      roomSideMax: 9,
      corridorStyle: DungeonCorridorStyle.MEANDERING,
      doorMix: DungeonDoorMix.TYPICAL,
      deadendRemoval: 60,
      stairs: 2,
    });
    expect(formProblems(DEFAULT_FORM)).toEqual({});
  });

  it('makes the shorter side two thirds of the longer one, odd (31 gives 31 × 21)', () => {
    expect([21, 31, 51, 81, 121].map(shortSideOf)).toEqual([15, 21, 35, 55, 81]);
  });

  it('reads the typed size for "Outro"', () => {
    expect(sideOf(form({ preset: null, sizeText: '45' }))).toBe(45);
    expect(sideOf(form({ preset: null, sizeText: 'abc' }))).toBeNull();
    expect(optionsOf(form({ preset: null, sizeText: '45' }))).toMatchObject({
      width: 45,
      height: 31,
    });
  });

  it('refuses a size out of 21 to 121 before it asks the server', () => {
    expect(formProblems(form({ preset: null, sizeText: '130' })).size).toBe(
      'O tamanho vai de 21 a 121 quadrados.',
    );
    expect(formProblems(form({ preset: null, sizeText: '20' })).size).toBeDefined();
    expect(formProblems(form({ preset: null, sizeText: '' })).size).toBeDefined();
    expect(formProblems(form({ preset: null, sizeText: '121' })).size).toBeUndefined();
  });

  it('refuses a smaller room side above the larger one, and a side out of range', () => {
    expect(formProblems(form({ roomMinText: '9', roomMaxText: '5' })).room_side_min).toBe(
      'O menor lado das salas não pode passar do maior.',
    );
    expect(formProblems(form({ roomMinText: '2' })).room_side_min).toBeDefined();
    expect(formProblems(form({ roomMinText: '16' })).room_side_min).toBeDefined();
    expect(formProblems(form({ roomMaxText: '32' })).room_side_max).toBeDefined();
    expect(formProblems(form({ roomMaxText: '' })).room_side_max).toBeDefined();
    expect(formProblems(form({ roomMinText: '3', roomMaxText: '31' }))).toEqual({});
  });

  it('reads whole numbers and seeds: digits only, a seed up to a uint64', () => {
    expect(wholeNumber(' 12 ')).toBe(12);
    expect(wholeNumber('1.5')).toBeNull();
    expect(parseSeed('48213')).toBe(48213n);
    expect(parseSeed('18446744073709551615')).toBe(18446744073709551615n);
    expect(parseSeed('18446744073709551616')).toBeNull();
    expect(parseSeed('-1')).toBeNull();
    expect(parseSeed('4 8')).toBeNull();
    expect(parseSeed('')).toBeNull();
  });
});
