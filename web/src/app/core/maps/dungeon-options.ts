import {
  DungeonCorridorStyle,
  DungeonDoorMix,
  DungeonMask,
} from '../../../gen/meurpg/maps/v1/dungeons_pb';
import type { DungeonOptionsInit } from './dungeons-client';
import type { OptionField } from './dungeon-errors';

/** What the "Gerar masmorra" page offers (E10-05 1); the generator's other options keep their defaults. */
export const SIZE_PRESETS: readonly { readonly side: number; readonly label: string }[] = [
  { side: 21, label: 'Mínima (21)' },
  { side: 31, label: 'Pequena (31)' },
  { side: 51, label: 'Média (51)' },
  { side: 81, label: 'Grande (81)' },
  { side: 121, label: 'Enorme (121)' },
];

/** The page's size range, in squares on the longer side (MR-010). The server takes 15 to 199; the page offers 21 to 121. */
export const SIZE_MIN = 21;
export const SIZE_MAX = 121;
export const ROOM_SIDE_MIN_RANGE = { min: 3, max: 15 } as const;
export const ROOM_SIDE_MAX_LIMIT = 31;
export const STAIRS_MAX = 4;

export const MASKS: readonly { readonly value: DungeonMask; readonly label: string }[] = [
  { value: DungeonMask.NONE, label: 'Sem forma' },
  { value: DungeonMask.DONUT, label: 'Anel' },
  { value: DungeonMask.PLUS, label: 'Cruz' },
  { value: DungeonMask.L_SHAPE, label: 'Em L' },
  { value: DungeonMask.ELLIPSE, label: 'Elipse' },
  { value: DungeonMask.DIAMOND, label: 'Losango' },
];

export const CORRIDORS: readonly {
  readonly value: DungeonCorridorStyle;
  readonly label: string;
}[] = [
  { value: DungeonCorridorStyle.TWISTY, label: 'Labirinto' },
  { value: DungeonCorridorStyle.MEANDERING, label: 'Sinuosos' },
  { value: DungeonCorridorStyle.LONG_RUNS, label: 'Retos' },
];

/** The door mixes, each with the one line that says what it means (my wording; the generator spec decides the numbers). */
export const DOOR_MIXES: readonly {
  readonly value: DungeonDoorMix;
  readonly label: string;
  readonly about: string;
}[] = [
  {
    value: DungeonDoorMix.OPEN,
    label: 'Só passagens',
    about: 'Só passagens: vãos sem porta, quase nenhuma porta de verdade.',
  },
  {
    value: DungeonDoorMix.TYPICAL,
    label: 'Comuns',
    about:
      'Comuns: a maioria fechada, umas trancadas, uma grade, uma porta secreta e algumas passagens sem porta.',
  },
  {
    value: DungeonDoorMix.SECURED,
    label: 'Seguras',
    about: 'Seguras: mais portas trancadas, grades e portas secretas.',
  },
  {
    value: DungeonDoorMix.PARANOID,
    label: 'Paranoicas',
    about: 'Paranoicas: muitas trancadas, grades e portas secretas.',
  },
];

/** What the form holds. A number field holds its text, so a half-typed value is never lost. */
export interface DungeonForm {
  /** The longer side in squares, or `null` for "Outro" with its own text. */
  readonly preset: number | null;
  /** The typed size, for "Outro". */
  readonly sizeText: string;
  readonly mask: DungeonMask;
  readonly roomMinText: string;
  readonly roomMaxText: string;
  readonly corridor: DungeonCorridorStyle;
  readonly doors: DungeonDoorMix;
  readonly deadends: number;
  readonly stairs: number;
}

export const DEFAULT_FORM: DungeonForm = {
  preset: 31,
  sizeText: '',
  mask: DungeonMask.NONE,
  roomMinText: '3',
  roomMaxText: '9',
  corridor: DungeonCorridorStyle.MEANDERING,
  doors: DungeonDoorMix.TYPICAL,
  deadends: 60,
  stairs: 2,
};

/** A whole number typed in a field: digits only, or `null`. */
export function wholeNumber(text: string): number | null {
  return /^\d{1,9}$/.test(text.trim()) ? Number(text.trim()) : null;
}

/** The longer side in squares: the preset's, or the typed one (`null` when it is not a number). */
export function sideOf(form: DungeonForm): number | null {
  return form.preset ?? wholeNumber(form.sizeText);
}

/** The shorter side: two thirds of the longer one, made odd (the generator keeps odd sizes: 31 gives 31 × 21). */
export function shortSideOf(side: number): number {
  const two = Math.round((side * 2) / 3);
  return Math.max(15, two % 2 === 0 ? two + 1 : two);
}

/** The reasons the form cannot be sent yet, by field (the page shows them in place; the server's refusals come on top of these). */
export function formProblems(form: DungeonForm): Partial<Record<OptionField, string>> {
  const out: Partial<Record<OptionField, string>> = {};
  const side = sideOf(form);
  if (side === null || side < SIZE_MIN || side > SIZE_MAX) {
    out.size = `O tamanho vai de ${SIZE_MIN} a ${SIZE_MAX} quadrados.`;
  }
  const min = wholeNumber(form.roomMinText);
  const max = wholeNumber(form.roomMaxText);
  if (min === null || min < ROOM_SIDE_MIN_RANGE.min || min > ROOM_SIDE_MIN_RANGE.max) {
    out.room_side_min = `O menor lado das salas vai de ${ROOM_SIDE_MIN_RANGE.min} a ${ROOM_SIDE_MIN_RANGE.max} quadrados.`;
  } else if (max !== null && min > max) {
    out.room_side_min = 'O menor lado das salas não pode passar do maior.';
  }
  if (max === null || max > ROOM_SIDE_MAX_LIMIT || max < ROOM_SIDE_MIN_RANGE.min) {
    out.room_side_max = `O maior lado das salas vai até ${ROOM_SIDE_MAX_LIMIT} quadrados.`;
  }
  return out;
}

/** The request's options for a form with no problems (call `formProblems` first). */
export function optionsOf(form: DungeonForm): DungeonOptionsInit {
  const side = sideOf(form) ?? 31;
  return {
    width: side,
    height: shortSideOf(side),
    mask: form.mask,
    roomSideMin: wholeNumber(form.roomMinText) ?? 3,
    roomSideMax: wholeNumber(form.roomMaxText) ?? 9,
    corridorStyle: form.corridor,
    doorMix: form.doors,
    deadendRemoval: form.deadends,
    stairs: form.stairs,
  };
}

/** A seed the master typed: digits, up to 18446744073709551615 (a uint64). `null` when it is not one. */
export function parseSeed(text: string): bigint | null {
  const t = text.trim();
  if (!/^\d{1,20}$/.test(t)) {
    return null;
  }
  const n = BigInt(t);
  return n <= 18446744073709551615n ? n : null;
}
