import { Injectable, computed, signal } from '@angular/core';

import type {
  ConcentrationSaveResult,
  DiceRoll,
  ReactionResult,
} from '../../../gen/meurpg/play/v1/combat_pb';
import type { EffectSaveResult } from '../../../gen/meurpg/play/v1/lasting_effects_pb';
import { type CheckRoll, RollModeKind } from '../../../gen/meurpg/play/v1/contest_types_pb';
import { extraDiceText } from '../../core/combat/combat-dice';
import type { DieSpec } from './tray/dice-types';

/** The dice the game uses. A d100 is drawn as two d10 (the tray expands it). */
export type DieSides = 4 | 6 | 8 | 10 | 12 | 20 | 100;
// eslint-disable-next-line @typescript-eslint/no-magic-numbers -- the faces of the game's dice
const SIDES: readonly number[] = [4, 6, 8, 10, 12, 20, 100];

/** One die that came up. `counts: false` is the d20 that advantage or disadvantage left out: it lands dimmed. */
export interface ShownDie {
  readonly sides: DieSides;
  readonly face: number;
  readonly counts?: boolean;
}

/** What the overlay shows for one roll: only what the screen that rolled it already displays (RN-20), nothing more. */
export interface RollShow {
  /** "Ataque com Machado grande", "Teste de Sabedoria", "Dano", "Teste contra a morte". */
  readonly label: string;
  /** The dice, in the order they landed. Past `MAX_DICE` the rest is a "+N" chip. */
  readonly dice: readonly ShownDie[];
  /** "2 + 5 + 3 = 10": only when the caller's screen shows the total. */
  readonly line?: string;
  /** "de fogo": the damage type, only when the screen shows it. */
  readonly note?: string;
  /** "Acertou", "Errou", "Passou", "Falhou": only when the caller's screen shows it; `good` picks the colour. */
  readonly outcome?: { readonly word: string; readonly good: boolean };
  /** A natural 20 the caller marks as a critical hit: the gold glow and "Crítico!". */
  readonly critical?: boolean;
  /** A natural 1 the caller marks as a fumble: "Falha crítica". */
  readonly fumble?: boolean;
}

/** What the tray is asked to roll: the dice and a number that changes with every roll, so the same show can play twice. */
export interface TrayRequest {
  readonly id: number;
  readonly dice: readonly DieSpec[];
}

/** The result stays for the table to read; the overlay closes by itself this long after it opened at the latest. */
export const OPEN_MS = 10_000;
const D20 = 20;
const D100 = 100;
/** At most this many dice are drawn; the rest is a "+N" chip. */
export const MAX_DICE = 8;
/** What the tray holds: a d100 is two dice on it (`MAX_ON_TRAY` of `tray/dice-sim`, which the initial bundle must not import). */
export const TRAY_BODIES = 12;

/**
 * The dice roll animation (docs/design/session.md#rolls): `play` throws the dice on a tray, which land on the real
 * faces and the result, and stay on screen to be read until a tap or Esc closes them, or 10 s pass at most (the roll
 * takes under 2 s even for eight dice). A tap or Esc while they roll lands them at once; a second one closes. A new
 * roll replaces the one playing. Under reduced motion (or when the browser cannot say) nothing moves: the dice appear
 * at rest with their results. It does nothing when no overlay is mounted, so a flow never waits for it, and it only
 * ever shows what the caller passes: the screens call it for a roll made on this screen, never for a typed physical
 * die or another person's roll.
 */
@Injectable({ providedIn: 'root' })
export class RollAnimator {
  private readonly now = signal<RollShow | null>(null);
  private readonly rollId = signal(0);
  private readonly landedNow = signal(false);
  private readonly reducedNow = signal(true);
  private readonly skipNow = signal(0);
  private readonly said = signal('');
  private hosts = 0;
  private closer: ReturnType<typeof setTimeout> | undefined;

  /** The roll on show, or null. */
  readonly current = this.now.asReadonly();
  /** The dice for the tray, and how many dice are left out ("+N"). */
  readonly request = computed<TrayRequest>(() => {
    const id = this.rollId();
    return { id, dice: trayDice(this.now()).dice };
  });
  readonly hidden = computed(() => trayDice(this.now()).hidden);
  /** True once every die has landed: the result shows. */
  readonly done = this.landedNow.asReadonly();
  /** True when nothing may move: the tray draws the dice at rest. */
  readonly reduced = this.reducedNow.asReadonly();
  /** Counts the taps that ask the dice to land at once; the tray watches it. */
  readonly skips = this.skipNow.asReadonly();
  /** The one sentence a screen reader hears, set when the dice land. */
  readonly announcement = this.said.asReadonly();

  /** The overlay in the shell announces itself: with none mounted (a unit test of a sheet), `play` does nothing and starts no timer. */
  attach(): () => void {
    this.hosts += 1;
    return () => {
      this.hosts -= 1;
      if (this.hosts <= 0) {
        this.hosts = 0;
        this.dismiss();
      }
    };
  }

  play(roll: RollShow): void {
    if (this.hosts === 0 || !validDice(roll.dice)) {
      return;
    }
    this.stop();
    this.reducedNow.set(!motionAllowed());
    this.landedNow.set(false);
    this.said.set('');
    this.rollId.update((n) => n + 1);
    this.now.set(roll);
    this.closer = setTimeout(() => this.dismiss(), OPEN_MS);
  }

  /** The tray reports that every die has landed: the result and the sentence show. */
  land(): void {
    const roll = this.now();
    if (roll && !this.landedNow()) {
      this.landedNow.set(true);
      this.said.set(sentence(roll));
    }
  }

  /** A tap or Esc: while the dice roll it lands them at once, then it closes. */
  tap(): void {
    if (this.now() && !this.landedNow()) {
      this.skipNow.update((n) => n + 1);
    } else {
      this.dismiss();
    }
  }

  /** Closes at once. */
  dismiss(): void {
    if (this.now()) {
      this.stop();
      this.now.set(null);
    }
  }

  private stop(): void {
    if (this.closer !== undefined) {
      clearTimeout(this.closer);
      this.closer = undefined;
    }
  }
}

/** False when the browser asks for reduced motion, and when it cannot tell (jsdom): the dice are then never moved. */
function motionAllowed(): boolean {
  if (typeof matchMedia !== 'function') {
    return false;
  }
  try {
    return !matchMedia('(prefers-reduced-motion: reduce)').matches;
  } catch {
    return false;
  }
}

function validDice(dice: readonly ShownDie[]): boolean {
  return (
    dice.length > 0 &&
    dice.every(
      (d) =>
        SIDES.includes(d.sides) && Number.isInteger(d.face) && d.face >= 1 && d.face <= d.sides,
    )
  );
}

/**
 * The dice the tray rolls for a show: `sides`, `result: face`, `discarded` for the d20 that advantage or disadvantage
 * left out, and the roll's critical or fumble on the d20 that counts. At most `MAX_DICE` are drawn (and as many as
 * fit the tray, a d100 being two); the rest is the "+N" chip.
 */
export function trayDice(roll: RollShow | null): { dice: DieSpec[]; hidden: number } {
  if (!roll) {
    return { dice: [], hidden: 0 };
  }
  let bodies = 0;
  const drawn: ShownDie[] = [];
  for (const d of roll.dice.slice(0, MAX_DICE)) {
    bodies += d.sides === D100 ? 2 : 1;
    if (bodies > TRAY_BODIES) {
      break;
    }
    drawn.push(d);
  }
  const counted = drawn.findIndex((d) => d.sides === D20 && d.counts !== false);
  return {
    dice: drawn.map((d, i) => ({
      sides: d.sides,
      result: d.face,
      discarded: d.counts === false,
      critical: !!roll.critical && i === counted,
      fumble: !!roll.fumble && i === counted,
    })),
    hidden: roll.dice.length - drawn.length,
  };
}

/** The word under the dice for a natural 20 or 1 the caller marked, or the caller's own outcome. */
export function outcomeLine(roll: RollShow): string {
  if (roll.critical) {
    return 'Crítico!';
  }
  if (roll.fumble) {
    return 'Falha crítica';
  }
  return roll.outcome?.word ?? '';
}

/** "Dano: 2 + 5 + 3 = 10 de fogo." */
function sentence(roll: RollShow): string {
  const faces = roll.dice
    .filter((d) => d.counts !== false)
    .map((d) => d.face)
    .join(', ');
  const text = [
    roll.line ? `${roll.line}${roll.note ? ` ${roll.note}` : ''}` : roll.note,
    outcomeLine(roll),
  ].filter((p) => !!p);
  return text.length > 0
    ? `${roll.label}: ${faces}. ${text.join('. ')}.`
    : `${roll.label}: ${faces}.`;
}

interface ShowMore extends Pick<RollShow, 'outcome' | 'critical' | 'fumble' | 'note'> {
  /** The screen shows the total: "14 + 5 = 19". */
  readonly withTotal?: boolean;
}

/** A roll the app made with dice the overlay draws. A die with no faces (a partial message) shows nothing: the
 * animation never stands in the way of the roll. */
function animatable(dice: DiceRoll | undefined): dice is DiceRoll {
  return !!dice && !dice.physical && !!dice.faces?.length && SIDES.includes(dice.diceSides);
}

/**
 * A `RollShow` from a roll the server answered (a d20, or the dice of a damage or hit die roll). A d20 pair shows both,
 * the one that counts highlighted. A physical roll is never animated: the player already has the dice in hand. A d20 a
 * feature raised (Talento Confiável), and a total the faces do not add up to (a maximum critical die), show only the total.
 */
export function showOfDice(
  label: string,
  dice: DiceRoll | undefined,
  more: ShowMore = {},
): RollShow | null {
  if (!animatable(dice)) {
    return null;
  }
  const sides = dice.diceSides as DieSides;
  const pair = sides === D20 && dice.faces.length > 1;
  const counted = pair ? dice.countedIndex : -1;
  const shown: ShownDie[] = dice.faces.map((face, i) => ({
    sides,
    face,
    counts: pair ? i === counted : true,
  }));
  const kept = pair ? [dice.faces[counted] ?? dice.faces[0]] : dice.faces;
  const face = kept[0];
  return {
    label,
    dice: shown,
    line: more.withTotal ? totalLine(dice, kept) : undefined,
    note: more.note,
    outcome: more.outcome,
    critical: more.critical && sides === D20 && face === D20,
    fumble: more.fumble && sides === D20 && face === 1,
  };
}

/** A bare die (the Bardic Inspiration die, a hit die): no roll message, just the number that came up. */
export function showOfDie(
  label: string,
  sides: number,
  face: number,
  line?: string,
): RollShow | null {
  if (!SIDES.includes(sides) || face < 1 || face > sides) {
    return null;
  }
  return { label, dice: [{ sides: sides as DieSides, face }], line };
}

function totalLine(dice: DiceRoll, kept: readonly number[]): string {
  const extra = extraDiceText(dice);
  const sum = kept.reduce((a, b) => a + b, 0) + dice.modifier + extra.sum;
  if (dice.treatedAs !== undefined || sum !== dice.total) {
    return `Total ${dice.total}`;
  }
  const mod =
    dice.modifier === 0 ? '' : ` ${dice.modifier < 0 ? '−' : '+'} ${Math.abs(dice.modifier)}`;
  return `${kept.join(' + ')}${mod}${extra.text} = ${dice.total}`;
}

/**
 * A `RollShow` from the d20 of a check the server answered as a `CheckRoll` (a contest, Hide, a group check): the pair of
 * an advantage or disadvantage roll shows both dice with the one that counts highlighted. Physical rolls are never animated.
 */
export function showOfCheck(
  label: string,
  roll: CheckRoll | undefined,
  more: ShowMore = {},
): RollShow | null {
  if (!roll || roll.physical || roll.faces.length === 0) {
    return null;
  }
  const pair = roll.faces.length > 1;
  const counted =
    roll.mode === RollModeKind.ADVANTAGE
      ? Math.max(...roll.faces)
      : roll.mode === RollModeKind.DISADVANTAGE
        ? Math.min(...roll.faces)
        : roll.faces[0];
  const countedAt = Math.max(0, roll.faces.indexOf(counted));
  const mod =
    roll.modifier === 0 ? '' : ` ${roll.modifier < 0 ? '−' : '+'} ${Math.abs(roll.modifier)}`;
  const line =
    counted + roll.modifier === roll.total
      ? `${counted}${mod} = ${roll.total}`
      : `Total ${roll.total}`;
  return {
    label,
    dice: roll.faces.map((face, i) => ({
      sides: D20 as DieSides,
      face,
      counts: pair ? i === countedAt : true,
    })),
    line: more.withTotal ? line : undefined,
    note: more.note,
    outcome: more.outcome,
    critical: more.critical && counted === D20,
    fumble: more.fumble && counted === 1,
  };
}

/**
 * The reaction a player (or the master, for an NPC) just answered, tumbling the dice its result sheet shows: the aggressor's
 * save of Repreensão Infernal, the check of Contramágica (none when the slot was high enough), the die of Palavras Cortantes
 * (only when it changed the roll; never the total or the armor class) and the reduction of Defletir Projéteis.
 */
export function showOfReaction(res: ReactionResult | undefined): RollShow | null {
  const r = res?.result;
  switch (r?.case) {
    // Repreensão Infernal's saving throw is the attacker's roll, not the reactor's: never animated on the reactor's screen.
    case 'counterspell':
      return showOfDice('Contramágica', r.value.check, {
        withTotal: true,
        outcome: { word: r.value.countered ? 'Anulada' : 'Não anulada', good: r.value.countered },
      });
    case 'cuttingWords':
      return r.value.effective ? showOfDice('Palavras Cortantes', r.value.die) : null;
    case 'deflectMissiles':
      return showOfDice('Defletir Projéteis', r.value.reduction, { withTotal: true });
    default:
      return null;
  }
}

/** The concentration saving throw, with "Passou" or "Falhou" (the sheet says whether it was kept). */
export function showOfConcentration(res: ConcentrationSaveResult | undefined): RollShow | null {
  return showOfDice('Teste de concentração', res?.save, {
    withTotal: true,
    outcome: res ? { word: res.kept ? 'Passou' : 'Falhou', good: res.kept } : undefined,
  });
}

/** An effect's saving throw the app rolled (`label` names the ability): the d20 pair, the total and "Passou" or "Falhou". */
export function showOfEffectSave(label: string, r: EffectSaveResult | undefined): RollShow | null {
  if (!r || r.autoFail || r.skipped || r.physical) {
    return null;
  }
  const faces = r.d20Faces.length === 2 ? r.d20Faces : [r.d20];
  const counted = Math.max(0, faces.indexOf(r.d20));
  const mod = r.modifier === 0 ? '' : ` ${r.modifier < 0 ? '−' : '+'} ${Math.abs(r.modifier)}`;
  return {
    label,
    dice: faces.map((face, i) => ({
      sides: D20 as DieSides,
      face,
      counts: faces.length === 1 || i === counted,
    })),
    line: r.extraDice.length > 0 ? `Total ${r.total}` : `${r.d20}${mod} = ${r.total}`,
    outcome: { word: r.saved ? 'Passou' : 'Falhou', good: r.saved },
  };
}
