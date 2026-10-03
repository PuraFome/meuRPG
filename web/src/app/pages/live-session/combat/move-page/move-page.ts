import { Component, ElementRef, afterNextRender, computed, input, output, signal, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  type Square,
  SQUARE_FT,
  canReach,
  distance,
  feetToMeters,
  formatMeters,
  moveDetail,
  reachSquares,
  squaresToMeters,
} from '../../../../core/combat/combat-grid';
import { ownCombatant, roundLabel } from '../../../../core/combat/combat-view';
import { CombatMap, type CombatMapImage } from '../../../../shared/combat-map/combat-map';
import { PHONE_QUERY, mediaQuery } from '../../../../shared/map-view/media-query';
import { LivePill } from '../../../../shared/live-pill/live-pill';

/** Side of a square on this page: the map is zoomed in so a thumb hits one. */
const CELL_PX = 32;

/**
 * "Mover" (E6-10, RN-21): a full page where the map is the control. The
 * squares the combatant reaches are tinted (a king's move, occupied squares
 * left out); the player taps one, reads how far it is, and confirms with
 * "Mover para cá". A square out of reach or taken is framed as refused, with
 * the reason, and the button stays blocked: the player is never moved past
 * the limit. The line under the map is a live region that says the distance
 * in words, so it works without seeing the map; the map is one focus stop,
 * with the arrows moving the choice and Enter confirming.
 */
@Component({
  selector: 'app-move-page',
  imports: [CombatMap, LivePill, MatButtonModule, MatIconModule],
  templateUrl: './move-page.html',
  styleUrl: './move-page.scss',
})
export class MovePage {
  readonly encounter = input.required<Encounter>();
  readonly image = input.required<CombatMapImage>();
  readonly mapName = input('');
  readonly sessionNumber = input(0);
  /** "Ver mapa": the same page without the reach and the buttons. */
  readonly readOnly = input(false);
  /** The master's "Abrir mapa": hidden combatants are drawn too. */
  readonly isMaster = input(false);
  readonly busy = input(false);
  /** A refusal from the server ("Longe demais…"), shown like a local one. */
  readonly serverError = input('');

  /** "Mover para cá": move the combatant to this square. */
  readonly confirm = output<Square>();
  /** "Cancelar", or the back arrow. */
  readonly back = output<void>();

  protected readonly chosen = signal<Square | null>(null);
  private readonly scroller = viewChild.required<ElementRef<HTMLElement>>('scroller');

  protected readonly cell = CELL_PX;
  /** From 1024px the map fits the left column and the controls sit beside it. */
  protected readonly wide = mediaQuery('(min-width: 1024px)');
  protected readonly round = computed(() => roundLabel(this.encounter().round));
  protected readonly own = computed(() => ownCombatant(this.encounter()));
  protected readonly origin = computed<Square>(() => {
    const own = this.own();
    return own ? { col: own.col, row: own.row } : { col: 0, row: 0 };
  });
  protected readonly squares = computed(() => reachSquares(this.own()?.movementLeftFt ?? 0));
  protected readonly reach = computed(() => ({ origin: this.origin(), squares: this.squares() }));
  protected readonly leftMeters = computed(() => formatMeters(feetToMeters(this.own()?.movementLeftFt ?? 0)));
  protected readonly totalMeters = computed(() => {
    const own = this.own();
    return formatMeters(feetToMeters((own?.speedFt ?? 0) * (own?.dashed ? 2 : 1)));
  });
  protected readonly occupied = computed<Square[]>(() =>
    this.encounter()
      .combatants.filter((c) => c.placed && c.id !== this.own()?.id)
      .map((c) => ({ col: c.col, row: c.row })),
  );

  /** Whether the chosen square is a legal move, and what to say if not. */
  protected readonly verdict = computed(() => {
    const to = this.chosen();
    if (!to) {
      return null;
    }
    const e = this.encounter();
    if (
      this.occupied().some((o) => o.col === to.col && o.row === to.row) ||
      (to.col === this.origin().col && to.row === this.origin().row)
    ) {
      return { ok: false as const, title: 'Ocupado', detail: 'Há alguém nesse quadrado. Escolha um quadrado destacado.' };
    }
    if (canReach(this.origin(), to, this.squares(), e.gridColumns, e.gridRows, this.occupied())) {
      return { ok: true as const, detail: moveDetail(this.origin(), to, this.own()?.movementLeftFt ?? 0), cost: formatMeters(squaresToMeters(distance(this.origin(), to))) };
    }
    const away = distance(this.origin(), to) * SQUARE_FT;
    const missing = away - (this.own()?.movementLeftFt ?? 0);
    return {
      ok: false as const,
      title: `Longe demais: faltam ${formatMeters(feetToMeters(missing))}`,
      detail: `Esse quadrado fica a ${formatMeters(feetToMeters(away))} e você tem ${this.leftMeters()}. Escolha um quadrado destacado.`,
    };
  });
  protected readonly frame = computed(() => {
    const square = this.chosen();
    return square ? { square, refused: !this.verdict()?.ok } : null;
  });
  protected readonly canMove = computed(() => this.verdict()?.ok === true && !this.busy());

  constructor() {
    // The map is wider than the screen: open with the token in view.
    afterNextRender(() => {
      const el = this.scroller().nativeElement;
      const x = (this.origin().col + 0.5) * CELL_PX - el.clientWidth / 2;
      el.scrollLeft = Math.max(0, x);
    });
  }

  protected choose(square: Square): void {
    this.chosen.set(square);
  }

  protected go(): void {
    const to = this.chosen();
    if (this.canMove() && to) {
      this.confirm.emit(to);
    }
  }
}
