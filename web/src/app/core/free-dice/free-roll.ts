import { timestampDate } from '@bufbuild/protobuf/wkt';

import { type FreeRoll, FreeRollMode } from '../../../gen/meurpg/play/v1/free_dice_pb';

/** How the d20 is rolled: one die, or two with the higher (advantage) or the lower (disadvantage) counting. */
export type FreeMode = 'normal' | 'advantage' | 'disadvantage';

/** One die of a free roll: the face, and whether it is the second d20 that did not count. */
export interface FreeDieVm {
  readonly face: number;
  readonly discarded: boolean;
}

/** A roll of the free dice roller (MR-052), as the screens read it. `logText` and `formula` come ready from the server. */
export interface FreeRollVm {
  readonly id: string;
  readonly rollerName: string;
  readonly byMaster: boolean;
  /** Only the master saw it: it never reached a player. */
  readonly secret: boolean;
  readonly sides: number;
  readonly count: number;
  readonly modifier: number;
  readonly mode: FreeMode;
  readonly dice: readonly FreeDieVm[];
  readonly total: number;
  /** "2d6 + 3", "1d20 - 2", "1d20 + 5 com vantagem". */
  readonly formula: string;
  /** "Rolagem livre · Brisa: 2d6 + 3 = 11 (4, 4)". */
  readonly logText: string;
  readonly createdAt: Date;
}

/** What the person asked for in the form. */
export interface FreeForm {
  readonly sides: number;
  readonly count: number;
  readonly modifier: number;
  readonly mode: FreeMode;
  /** "Só eu": the master's roll that no player sees. */
  readonly secret: boolean;
}

/** The dice the server rolls (the board's chips). */
const D4 = 4;
const D6 = 6;
const D8 = 8;
const D10 = 10;
const D12 = 12;
const D20 = 20;
const D100 = 100;
export const FREE_DICE: readonly number[] = [D4, D6, D8, D10, D12, D20, D100];
export const MAX_COUNT = 20;
export const MAX_MODIFIER = 20;
/** The newest rolls the lists hold: the table's log and the caller's own. */
export const TABLE_LIMIT = 50;
export const MINE_LIMIT = 10;

const SECONDS_PER_MINUTE = 60;
const MINUTES_PER_HOUR = 60;

/** "Atalhos (preenchem o formulário)": the dice most rolls are. */
export interface FreeShortcut {
  readonly label: string;
  readonly sides: number;
  readonly count: number;
}
export const SHORTCUTS: readonly FreeShortcut[] = [
  { label: '1d20', sides: 20, count: 1 },
  { label: '1d6', sides: 6, count: 1 },
  { label: '2d6', sides: 6, count: 2 },
  { label: '1d100', sides: 100, count: 1 },
];

export const DEFAULT_FORM: FreeForm = {
  sides: D20,
  count: 1,
  modifier: 0,
  mode: 'normal',
  secret: false,
};

const MODES: Record<FreeRollMode, FreeMode> = {
  [FreeRollMode.UNSPECIFIED]: 'normal',
  [FreeRollMode.ADVANTAGE]: 'advantage',
  [FreeRollMode.DISADVANTAGE]: 'disadvantage',
};

export const WIRE_MODE: Record<FreeMode, FreeRollMode> = {
  normal: FreeRollMode.UNSPECIFIED,
  advantage: FreeRollMode.ADVANTAGE,
  disadvantage: FreeRollMode.DISADVANTAGE,
};

export function toFreeRollVm(m: FreeRoll): FreeRollVm {
  return {
    id: m.id,
    rollerName: m.rollerName,
    byMaster: m.byMaster,
    secret: m.secret,
    sides: m.sides,
    count: m.count,
    modifier: m.modifier,
    mode: MODES[m.mode] ?? 'normal',
    dice: m.dice.map((d) => ({ face: d.face, discarded: d.discarded })),
    total: m.total,
    formula: m.formula,
    logText: m.logText,
    createdAt: m.createdAt ? timestampDate(m.createdAt) : new Date(0),
  };
}

function clamp(n: number, min: number, max: number): number {
  const whole = Number.isFinite(n) ? Math.trunc(n) : min;
  return Math.min(max, Math.max(min, whole));
}

/** Advantage and disadvantage stand for one d20 only. */
export function canAdvantage(form: Pick<FreeForm, 'sides' | 'count'>): boolean {
  return form.sides === D20 && form.count === 1;
}

/** The form with every value in the server's range; a mode that does not fit the dice goes back to normal. */
export function normalizeForm(form: FreeForm): FreeForm {
  const next = {
    ...form,
    sides: FREE_DICE.includes(form.sides) ? form.sides : D20,
    count: clamp(form.count, 1, MAX_COUNT),
    modifier: clamp(form.modifier, -MAX_MODIFIER, MAX_MODIFIER),
  };
  return canAdvantage(next) ? next : { ...next, mode: 'normal' };
}

const MODE_WORDS: Record<FreeMode, string> = {
  normal: '',
  advantage: ' com vantagem',
  disadvantage: ' com desvantagem',
};

/** "2d6 + 3", written the way the server writes it. */
export function formulaOf(form: FreeForm): string {
  const sign = form.modifier < 0 ? '-' : '+';
  const mod = form.modifier === 0 ? '' : ` ${sign} ${Math.abs(form.modifier)}`;
  return `${form.count}d${form.sides}${mod}${MODE_WORDS[form.mode]}`;
}

/** The form that asks for the same dice again. */
export function formFromRoll(roll: FreeRollVm): FreeForm {
  return normalizeForm({
    sides: roll.sides,
    count: roll.count,
    modifier: roll.modifier,
    mode: roll.mode,
    secret: roll.secret,
  });
}

/** Who rolls and what the table plays with decide the big button's words. */
export interface ButtonContext {
  readonly master: boolean;
  /** The table plays the game's dice with physical dice (RN-18): this roll is still the app's. */
  readonly physical: boolean;
}

/**
 * "Rolar 2d6 + 3", "Rolar 2d6 + 3 para a mesa" (the master), "Rolar 1d20 com vantagem, só eu", "Rolar no app".
 * `again` is the result card's button: "Rolar 2d6 + 3 de novo".
 */
export function rollButtonLabel(form: FreeForm, ctx: ButtonContext, again = false): string {
  const repeat = again ? ' de novo' : '';
  if (ctx.physical) {
    return `Rolar no app${repeat}`;
  }
  if (!ctx.master) {
    return `Rolar ${formulaOf(form)}${repeat}`;
  }
  // "Rolar 2d6 + 3 para a mesa", but "Rolar 2d6 + 3 de novo, para a mesa".
  const who = form.secret ? ', só eu' : again ? ', para a mesa' : ' para a mesa';
  return `Rolar ${formulaOf(form)}${repeat}${who}`;
}

/** The accessible name of the big button when its words leave the formula out. */
export function rollButtonName(form: FreeForm, ctx: ButtonContext, again = false): string {
  return ctx.physical
    ? `${rollButtonLabel(form, ctx, again)}: ${formulaOf(form)}`
    : rollButtonLabel(form, ctx, again);
}

/** "Repetir 2d6 + 3". */
export function repeatLabel(roll: FreeRollVm): string {
  return `Repetir ${roll.formula}`;
}

/** The die that counts, for the natural 20 and the natural 1 (one d20 only). */
function countedFace(roll: FreeRollVm): number | null {
  if (roll.sides !== D20 || roll.count !== 1) {
    return null;
  }
  return roll.dice.find((d) => !d.discarded)?.face ?? null;
}

/** "20 natural" or "1 natural", in words so the colour is never alone; `null` for any other roll. */
export function naturalWord(roll: FreeRollVm): string | null {
  const face = countedFace(roll);
  return face === D20 ? '20 natural' : face === 1 ? '1 natural' : null;
}

/** "2d6 + 3 = 11", with " · 20 natural" after it when it is one. */
export function resultLine(roll: FreeRollVm): string {
  const natural = naturalWord(roll);
  return `${roll.formula} = ${roll.total}${natural ? ` · ${natural}` : ''}`;
}

/** "2d6 + 3 = 11 (4, 4)" and "1d20 + 5 com vantagem = 19 (14, 9 descartado)". */
export function summaryOf(roll: FreeRollVm): string {
  const faces = roll.dice.map((d) => (d.discarded ? `${d.face} descartado` : String(d.face)));
  return `${roll.formula} = ${roll.total} (${faces.join(', ')})`;
}

/** "para a mesa" or "só você". */
export function audienceText(roll: FreeRollVm): string {
  return roll.secret ? 'só você' : 'para a mesa';
}

/** The log line cut at the roller's name, so the screen can set it in bold: "Rolagem livre · Brisa:" and the rest. */
export function logParts(roll: FreeRollVm): { who: string; rest: string } {
  const at = roll.logText.indexOf(': ');
  return at < 0
    ? { who: '', rest: roll.logText }
    : { who: roll.logText.slice(0, at + 1), rest: roll.logText.slice(at + 2) };
}

/** "agora", "há 3 min", "há 2 h". A clock a little behind never reads in the future. */
export function whenText(at: Date, now: Date): string {
  const seconds = Math.max(0, Math.floor((now.getTime() - at.getTime()) / 1000));
  const minutes = Math.floor(seconds / SECONDS_PER_MINUTE);
  if (minutes < 1) {
    return 'agora';
  }
  return minutes < MINUTES_PER_HOUR
    ? `há ${minutes} min`
    : `há ${Math.floor(minutes / MINUTES_PER_HOUR)} h`;
}

/** The two lists merged by id, newest first, cut to `limit` (a roll made while a read was on its way is kept). */
export function mergeRolls(
  read: readonly FreeRollVm[],
  local: readonly FreeRollVm[],
  limit: number,
): FreeRollVm[] {
  const byId = new Map<string, FreeRollVm>();
  for (const roll of [...local, ...read]) {
    byId.set(roll.id, roll);
  }
  return [...byId.values()]
    .sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime())
    .slice(0, limit);
}
