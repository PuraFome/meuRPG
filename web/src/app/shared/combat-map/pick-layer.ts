import { computed } from '@angular/core';

import type { Combatant } from '../../../gen/meurpg/play/v1/combat_pb';

/** How near a side edge (a share of the map's width) a chip leans away from it. */
const EDGE_LEAN = 0.2;

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
    return m.mode === 'darts' && m.count > 0
      ? `Pôr outro dardo em ${m.label} (${m.count} agora)`
      : `Escolher ${m.label} como alvo`;
  }

  /** The chip opens toward the middle of the map, below the token unless it is at the foot. */
  side(c: Combatant): string {
    const { columns, rows } = this.grid();
    const right = c.col * 2 > columns ? ' cm__why--right' : '';
    return `cm__why${right}${c.row + 2 >= rows ? ' cm__why--above' : ''}`;
  }

  /** The "Longe" chip's place: below its token, above at the foot, leaning away from a side edge. */
  farSide(c: Combatant): string {
    const { columns, rows } = this.grid();
    const x = (c.col + HALF) / columns;
    const lean = x < EDGE_LEAN ? ' cm__far--left' : x > 1 - EDGE_LEAN ? ' cm__far--right' : '';
    return `cm__far${lean}${c.row + 2 >= rows ? ' cm__far--above' : ''}`;
  }
}
