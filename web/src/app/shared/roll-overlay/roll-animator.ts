import { Injectable, computed, signal } from '@angular/core';

import type { DiceRoll } from '../../../gen/meurpg/play/v1/combat_pb';
import { extraDiceText } from '../../core/combat/combat-dice';

/** The dice the game uses. A d100 is drawn as two d10 (see `shapesOf`). */
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

/** What the overlay draws: one silhouette per shape. */
export interface DieShape {
  /** The silhouette: a d100 is two d10. */
  // eslint-disable-next-line @typescript-eslint/no-magic-numbers -- the faces of the game's dice are the type
  readonly shape: 4 | 6 | 8 | 10 | 12 | 20;
  /** The number it lands on ("00" to "90" for the tens die of a d100). */
  readonly text: string;
  readonly counts: boolean;
  /** How many numbers it cycles through while it tumbles. */
  readonly spin: number;
  /** The index of the die in the roll, to tell the critical d20. */
  readonly source: number;
  /** Which of the two percentile d10 of a d100 this is: the tens ("00" to "90") or the units ("0" to "9"). */
  readonly part?: 'tens' | 'units';
}

/** Timings (milliseconds): the tumble, the pause between one die landing and the next, and the longest the overlay stays open. */
export const TUMBLE_MS = 700;
export const STAGGER_MS = 80;
/** The result stays for the table to read; the overlay closes by itself this long after it opened at the latest. */
export const OPEN_MS = 10_000;
const CYCLE_MS = 60;
/** Steps of the tumbling number: any odd numbers will do, they only make the digits look random. */
const SPIN_TICK = 7;
const SPIN_DIE = 13;
const D20 = 20;
const D100 = 100;
const TENS = 10;
/** At most this many dice are drawn; the rest is a "+N" chip. */
export const MAX_DICE = 8;

/**
 * The dice roll animation (docs/design.md#roll-animation): `play` shows the dice tumbling together, landing one after
 * another on the real faces and the result, which stay on screen to be read and close by themselves after 10 s at most (the landing takes under 2 s even for eight dice). A new roll replaces
 * the one playing; a tap or Esc closes it. It does nothing when the browser asks for reduced motion (or cannot say) or
 * when no overlay is mounted, so a flow never waits for it, and it only ever shows what the caller passes: the screens
 * call it for a roll made on this screen, never for a typed physical die or another person's roll.
 */
@Injectable({ providedIn: 'root' })
export class RollAnimator {
  private readonly now = signal<RollShow | null>(null);
  private readonly landedNow = signal(0);
  private readonly tickNow = signal(0);
  private readonly said = signal('');
  private hosts = 0;
  private cycle: ReturnType<typeof setInterval> | undefined;
  private timer: ReturnType<typeof setTimeout> | undefined;
  private closer: ReturnType<typeof setTimeout> | undefined;

  /** The roll on show, or null. */
  readonly current = this.now.asReadonly();
  /** The silhouettes to draw, and how many dice are left out ("+N"). */
  readonly shapes = computed(() => {
    const r = this.now();
    return r ? shapesOf(r.dice.slice(0, MAX_DICE)) : [];
  });
  readonly hidden = computed(() => Math.max(0, (this.now()?.dice.length ?? 0) - MAX_DICE));
  /** How many silhouettes have landed. */
  readonly landed = this.landedNow.asReadonly();
  /** True once every die has landed: the result shows. */
  readonly done = computed(() => this.now() !== null && this.landedNow() >= this.shapes().length);
  /** The one sentence a screen reader hears, set when the dice land. */
  readonly announcement = this.said.asReadonly();

  /** The number a die shows while it tumbles (display only). */
  spinning(index: number, spin: number): number {
    return 1 + ((this.tickNow() * SPIN_TICK + index * SPIN_DIE) % spin);
  }

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
    if (this.hosts === 0 || !motionAllowed() || !validDice(roll.dice)) {
      return;
    }
    this.stop();
    this.now.set(roll);
    this.landedNow.set(0);
    this.said.set('');
    this.cycle = setInterval(() => this.tickNow.update((t) => t + 1), CYCLE_MS);
    this.timer = setTimeout(() => this.land(roll), TUMBLE_MS);
    this.closer = setTimeout(() => this.dismiss(), OPEN_MS);
  }

  /** A tap or Esc: closes at once. */
  dismiss(): void {
    if (this.now()) {
      this.stop();
      this.now.set(null);
    }
  }

  private land(roll: RollShow): void {
    const total = this.shapes().length;
    this.landedNow.update((n) => n + 1);
    if (this.landedNow() < total) {
      this.timer = setTimeout(() => this.land(roll), STAGGER_MS);
      return;
    }
    this.stopCycle();
    this.said.set(sentence(roll));
  }

  private stop(): void {
    this.stopCycle();
    if (this.timer !== undefined) {
      clearTimeout(this.timer);
      this.timer = undefined;
    }
    if (this.closer !== undefined) {
      clearTimeout(this.closer);
      this.closer = undefined;
    }
  }

  private stopCycle(): void {
    if (this.cycle !== undefined) {
      clearInterval(this.cycle);
      this.cycle = undefined;
    }
  }
}

/** False when the browser asks for reduced motion, and when it cannot tell (jsdom): the animation is then never drawn. */
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
 * The silhouettes of the dice. A d100 is the two percentile d10 side by side, as at the table (SRD): the tens die shows
 * "00" to "90" and the units die "0" to "9", and the face 100 shows "00" and "0".
 */
export function shapesOf(dice: readonly ShownDie[]): DieShape[] {
  return dice.flatMap((d, source): DieShape[] => {
    const counts = d.counts !== false;
    if (d.sides === D100) {
      const tens = Math.floor((d.face % D100) / TENS) * TENS;
      return [
        {
          shape: 10,
          text: String(tens).padStart(2, '0'),
          counts,
          spin: TENS,
          source,
          part: 'tens',
        },
        { shape: 10, text: String(d.face % TENS), counts, spin: TENS, source, part: 'units' },
      ];
    }
    return [
      {
        shape: d.sides as DieShape['shape'],
        text: String(d.face),
        counts,
        spin: d.sides,
        source,
      },
    ];
  });
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
  if (!dice || dice.physical || dice.faces.length === 0 || !SIDES.includes(dice.diceSides)) {
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
