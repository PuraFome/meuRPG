import { computed } from '@angular/core';

import type { Combatant } from '../../../gen/meurpg/play/v1/combat_pb';

/** How near a side edge (a share of the map's width) a chip leans away from it. */
const EDGE_LEAN = 0.2;

/** Chips within this many columns of each other, in the same zone, would touch. */
const NEAR_CHIPS = 2;

/** How near a side edge (a share of the map's width) the "Vez" word leans inward. */
const TURN_LEAN = 0.08;

/** The middle of a square. */
const HALF = 0.5;

/** A creature of the map while the player chooses a target (C7-04): ringed and tappable, or dimmed with the reason a tap tells. */
export interface PickMark {
  readonly id: string;
  readonly label: string;
  /** Why it cannot be the target ("cobertura total"); `''` for a ringed one. */
  readonly reason: string;
  /** A valid target with a catch, written over it ("Longe: desvantagem"). */
  readonly note: string;
  /** The marks it has: the chosen target is 1, a missile's darts are as many as it took. */
  readonly count: number;
  /** What the marks are: the one target ("Alvo") or darts ("×2"). */
  readonly mode: 'target' | 'darts';
}

/** The chip over the map that says why a creature cannot be the target. */
export interface PickNote {
  readonly id: string;
  readonly text: string;
}

/** What the map reads to draw the choice of a target: the marks by creature, the dimmed ones and the chip beside its token. */
export class PickLayer {
  private readonly byId = computed(() => new Map((this.marks() ?? []).map((m) => [m.id, m])));
  /** The chip's text and the creature it sits beside. */
  readonly note = computed(() => {
    const note = this.chip();
    const c = note ? this.shown().find((x) => x.id === note.id) : undefined;
    return note && c ? { text: note.text, c } : null;
  });

  constructor(
    private readonly marks: () => readonly PickMark[] | null,
    private readonly chip: () => PickNote | null,
    private readonly shown: () => readonly Combatant[],
    private readonly grid: () => { readonly columns: number; readonly rows: number },
  ) {}

  of(id: string): PickMark | undefined {
    return this.byId().get(id);
  }

  /** Dimmed while the player chooses: a creature the action cannot aim at. */
  dim(id: string): boolean {
    return !!this.byId().get(id)?.reason;
  }

  /** The button's name: "Escolher Goblin 1 como alvo", or why the creature cannot be one. */
  name(m: PickMark): string {
    if (m.reason) {
      return `${m.label}: ${m.reason}`;
    }
    const base =
      m.mode === 'darts' && m.count > 0
        ? `Pôr outro dardo em ${m.label} (${m.count} agora)`
        : `Escolher ${m.label} como alvo`;
    return m.note ? `${base}. ${m.note}` : base;
  }

  /** The chip opens toward the middle of the map, below the token unless it is at the foot. */
  side(c: Combatant): string {
    const { columns, rows } = this.grid();
    const right = c.col * 2 > columns ? ' cm__why--right' : '';
    return `cm__why${right}${c.row + 2 >= rows ? ' cm__why--above' : ''}`;
  }

  /** Where each "Longe" chip goes: below its token unless a creature stands there or another chip is, then above; never two chips in one zone. */
  private readonly farPlan = computed(() => {
    const occupied = new Set(this.shown().map((c) => `${c.col},${c.row}`));
    const free = (c: Combatant, dr: number) =>
      [-1, 0, 1].every((dx) => !occupied.has(`${c.col + dx},${c.row + dr}`));
    const zones: { col: number; zone: number }[] = [];
    const plan = new Map<string, 'below' | 'above'>();
    const { rows } = this.grid();
    for (const c of this.shown().filter((x) => this.byId().get(x.id)?.note)) {
      const order: ('below' | 'above')[] =
        c.row + 2 >= rows ? ['above', 'below'] : ['below', 'above'];
      const zoneOf = (side: 'below' | 'above') => (side === 'below' ? c.row + 1 : c.row - 1);
      const clash = (side: 'below' | 'above') =>
        zones.some((z) => z.zone === zoneOf(side) && Math.abs(z.col - c.col) <= NEAR_CHIPS);
      const room = (side: 'below' | 'above') =>
        zoneOf(side) >= 0 && zoneOf(side) < rows && free(c, zoneOf(side) - c.row);
      const side =
        order.find((o) => room(o) && !clash(o)) ?? order.find((o) => !clash(o)) ?? order[0];
      plan.set(c.id, side);
      zones.push({ col: c.col, zone: zoneOf(side) });
    }
    return plan;
  });
  private readonly noted = computed(() => (this.marks() ?? []).filter((m) => m.note).length);

  /** The chip's words: whole when it is the only one, "Longe" when several share the map (the button's name has the whole). */
  far(m: PickMark): string {
    return m.note.split(':')[0];
  }

  /** The rest of a lone chip (": desvantagem"); a narrow map hides it by its width, and several chips never have it. */
  farRest(m: PickMark): string {
    return this.noted() > 1 ? '' : m.note.slice(this.far(m).length);
  }

  /** The "Longe" chip's place: below its token, above when something is there or at the foot, leaning away from a side edge. */
  farSide(c: Combatant): string {
    const { columns } = this.grid();
    const x = (c.col + HALF) / columns;
    const lean = x < EDGE_LEAN ? ' cm__far--left' : x > 1 - EDGE_LEAN ? ' cm__far--right' : '';
    return `cm__far${lean}${this.farPlan().get(c.id) === 'above' ? ' cm__far--above' : ''}`;
  }

  /** A "Vez" word above or below a token at a side edge leans inward, so the map does not cut it. */
  turnLean(c: Combatant): string {
    const { columns } = this.grid();
    const x = (c.col + HALF) / columns;
    return x < TURN_LEAN
      ? ' cm__turn--lean-left'
      : x > 1 - TURN_LEAN
        ? ' cm__turn--lean-right'
        : '';
  }
}
