import { Component, computed, input, output, signal, viewChild, ElementRef } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant } from '../../../gen/meurpg/play/v1/combat_pb';
import { DFT_PER_SQUARE, type Square, squareAt, squareCenter, stepSquare } from '../../core/combat/combat-grid';
import type { MapLayers } from '../../core/maps/layers';
import { conditionTags } from '../../core/combat/conditions';
import { combatantInitial, isPlayer } from '../../core/combat/combat-view';
import { CombatantToken } from '../combatant-token/combatant-token';
import { MapLayersOverlay } from '../map-layers/map-layers';

/** The map's picture: its URL and size (the frame is reserved from it). */
export interface CombatMapImage {
  readonly url: string;
  readonly width: number;
  readonly height: number;
}

/** What the server said a combatant reaches (`GetMoveOptions`): the square it
 * starts from, the radius of the circle in tenths of a foot (the movement it has
 * left, or a jump's limit) and the squares it can go to. The map only draws it. */
export interface Reach {
  readonly origin: Square;
  readonly leftDft: number;
  readonly squares: readonly Square[];
}

/** A square the person chose; `refused` when the server said it cannot be
 * moved to, and `label` the cost ("2,1 m") written beside it. */
export interface Chosen {
  readonly square: Square;
  readonly refused: boolean;
  readonly label?: string;
}

/** A reactor whose reach a pending opportunity offer is about, and the square
 * where the mover left it: drawn for whoever answers (E9-13). */
export interface OfferMark {
  readonly reactor: Square;
  readonly left: Square | null;
}

export interface TokenDrop extends Square {
  readonly id: string;
}

/**
 * The battle map (RN-21; E6-02, E6-04, E6-05, E6-10, E6-11): the map's image,
 * the grid on top of it and a token on each combatant's square. Presentational:
 * it never calls the API; the page saves what it reports.
 *
 * - **Who is drawn:** the combatants that have a square. A player never gets a
 *   hidden one from the server; the master's hidden ones are drawn dashed.
 * - **Layers (Etapa 9):** the walls, the difficult terrain and the cover, drawn
 *   by `app-map-layers` over the image.
 * - **Reach (E6-10, MAP-LANGUAGE.md):** the squares the server says a combatant
 *   can go to, tinted, inside a dashed circle of the movement left. A square the
 *   circle holds that is not tinted gets no mark of its own.
 * - **Offers (E9-13):** a dashed outline on the square of the reactor of a pending
 *   opportunity attack and a label on the square the mover left.
 * - **Moving:** the master drags any token (`masterMoves`); a player drags
 *   their own inside the reach (`ownMoveId`). With `pickSquares`, a click on
 *   a square (or an arrow key) chooses it (`choose`) and Enter confirms
 *   (`confirm`), the phone's "Mover" page. The map is one focus stop; the
 *   arrows move the choice one square.
 * - **Size:** it fills its container; with `cellPx` each square has that side
 *   and the container scrolls (the "Mover" page zooms in).
 */
@Component({
  selector: 'app-combat-map',
  imports: [CombatantToken, MapLayersOverlay, MatIconModule],
  templateUrl: './combat-map.html',
  styleUrl: './combat-map.scss',
})
export class CombatMap {
  readonly image = input.required<CombatMapImage>();
  readonly mapName = input('');
  readonly columns = input.required<number>();
  readonly rows = input.required<number>();
  readonly combatants = input<readonly Combatant[]>([]);
  readonly isMaster = input(false);
  readonly currentId = input('');
  readonly reach = input<Reach | null>(null);
  readonly layers = input<MapLayers | null>(null);
  readonly offers = input<readonly OfferMark[]>([]);
  readonly chosen = input<Chosen | null>(null);
  /** One square marked without a token (E6-02: "O quadrado marcado tem
   * 1,5 m"), to judge the grid's size by. */
  readonly mark = input<Square | null>(null);
  readonly masterMoves = input(false);
  readonly ownMoveId = input<string | null>(null);
  readonly pickSquares = input(false);
  /** Side of one square in pixels; `null` fits the map to its container. */
  readonly cellPx = input<number | null>(null);
  /** The "Vez" word above the one on turn: left out where the page is about one mover (the "Mover" page). */
  readonly showTurn = input(true);

  /** A square was chosen (a click, or an arrow key) in `pickSquares` mode. */
  readonly choose = output<Square>();
  /** Enter on the chosen square in `pickSquares` mode. */
  readonly confirm = output<void>();
  /** A token was dropped on a square (a drag, or Enter after the arrows). */
  readonly tokenDrop = output<TokenDrop>();

  private readonly surface = viewChild.required<ElementRef<HTMLElement>>('surface');

  /** The token under the pointer while it is dragged, and where. */
  protected readonly drag = signal<TokenDrop | null>(null);
  /** The token the keyboard moves (the master picks one with a click). */
  private readonly picked = signal<string | null>(null);
  /** The square the arrow keys moved to, outside `pickSquares` mode. */
  protected readonly cursor = signal<Square | null>(null);
  private dragging: { id: string; moved: boolean; pointer: number } | null = null;

  protected readonly shown = computed(() =>
    this.combatants().filter((c) => c.placed && (this.isMaster() || !c.hidden)),
  );
  protected readonly aspect = computed(() => `${this.image().width} / ${this.image().height}`);
  protected readonly widthPx = computed(() => {
    const cell = this.cellPx();
    return cell ? cell * this.columns() : null;
  });
  protected readonly gridPath = computed(() => {
    const parts: string[] = [];
    for (let c = 0; c <= this.columns(); c++) {
      parts.push(`M${c} 0V${this.rows()}`);
    }
    for (let r = 0; r <= this.rows(); r++) {
      parts.push(`M0 ${r}H${this.columns()}`);
    }
    return parts.join('');
  });
  /** The squares the reach tints (the server's list). */
  protected readonly reachCells = computed<readonly Square[]>(() => this.reach()?.squares ?? []);
  /** The circle of the movement left, in squares (a square straight is 50 tenths of a foot). */
  protected readonly ringRadius = computed(() => {
    const reach = this.reach();
    return reach ? reach.leftDft / DFT_PER_SQUARE : 0;
  });
  /** The square of each reactor, outlined: the server does not say how far its reach goes, so none is drawn. */
  protected readonly reactorBoxes = computed(() =>
    this.offers().map((o) => ({ col: o.reactor.col, row: o.reactor.row, left: o.left })),
  );

  /** The frame on the chosen square (a tap, or the arrow keys' cursor). */
  protected readonly frame = computed<Chosen | null>(() => {
    const chosen = this.chosen();
    if (chosen) {
      return chosen;
    }
    const cursor = this.cursor();
    return cursor ? { square: cursor, refused: false } : null;
  });
  protected readonly framedToken = computed(() => this.tokenOf(this.pickSquares() ? (this.ownMoveId() ?? '') : (this.picked() ?? this.ownMoveId() ?? '')));

  /** The dashed line from the token to the chosen square (E6-10). */
  protected readonly path = computed(() => {
    const frame = this.frame();
    const token = this.pickSquares() ? this.originToken() : this.framedToken();
    if (!frame || !token) {
      return null;
    }
    const from = squareCenter(token, this.columns(), this.rows());
    const to = squareCenter(frame.square, this.columns(), this.rows());
    return { x1: from.x, y1: from.y, x2: to.x, y2: to.y };
  });

  protected readonly listed = computed(() =>
    this.shown().map((c) => ({
      id: c.id,
      text: `${c.label}, coluna ${c.col + 1}, linha ${c.row + 1}${c.conditions.length ? `, ${conditionTags(c).join(', ')}` : ''}`,
    })),
  );

  protected readonly canMove = computed(
    () => this.masterMoves() || this.ownMoveId() !== null || this.pickSquares(),
  );

  protected initial(c: Combatant): string {
    return combatantInitial(c.label);
  }

  protected npc(c: Combatant): boolean {
    return !isPlayer(c);
  }

  protected place(c: Combatant): Square {
    const drag = this.drag();
    return drag && drag.id === c.id ? drag : { col: c.col, row: c.row };
  }

  protected left(s: Square): number {
    return squareCenter(s, this.columns(), this.rows()).x;
  }

  protected top(s: Square): number {
    return squareCenter(s, this.columns(), this.rows()).y;
  }

  protected movable(c: Combatant): boolean {
    return this.masterMoves() || c.id === this.ownMoveId();
  }

  private tokenOf(id: string): Square | null {
    const c = this.shown().find((x) => x.id === id);
    return c ? { col: c.col, row: c.row } : null;
  }

  /** The square the reach starts from (the mover's own token). */
  private originToken(): Square | null {
    return this.reach()?.origin ?? this.tokenOf(this.ownMoveId() ?? '');
  }

  // ---- pointer ----

  private squareUnder(event: PointerEvent | MouseEvent): Square {
    const rect = this.surface().nativeElement.getBoundingClientRect();
    return squareAt(
      (event.clientX - rect.left) / rect.width,
      (event.clientY - rect.top) / rect.height,
      this.columns(),
      this.rows(),
    );
  }

  protected onPointerDown(event: PointerEvent): void {
    const id = (event.target as HTMLElement).closest<HTMLElement>('[data-id]')?.dataset['id'];
    const token = id ? this.shown().find((c) => c.id === id) : undefined;
    if (!token || !this.movable(token) || event.button > 0) {
      return;
    }
    this.picked.set(token.id);
    this.dragging = { id: token.id, moved: false, pointer: event.pointerId };
    this.surface().nativeElement.setPointerCapture?.(event.pointerId);
  }

  protected onPointerMove(event: PointerEvent): void {
    const d = this.dragging;
    if (!d || d.pointer !== event.pointerId) {
      return;
    }
    d.moved = true;
    this.drag.set({ id: d.id, ...this.squareUnder(event) });
  }

  protected onPointerUp(event: PointerEvent): void {
    const d = this.dragging;
    this.dragging = null;
    if (!d || d.pointer !== event.pointerId) {
      return;
    }
    const drop = this.drag();
    this.drag.set(null);
    if (event.type === 'pointerup' && d.moved && drop) {
      this.tokenDrop.emit(drop);
    }
  }

  protected onClick(event: MouseEvent): void {
    if (!this.pickSquares() || (event.target as HTMLElement).closest('[data-id]')) {
      return;
    }
    this.choose.emit(this.squareUnder(event));
  }

  // ---- keyboard ----

  protected onKeydown(event: KeyboardEvent): void {
    if (!this.canMove()) {
      return;
    }
    if (event.key === 'Enter') {
      event.preventDefault();
      if (this.pickSquares()) {
        this.confirm.emit();
        return;
      }
      const cursor = this.cursor();
      const id = this.picked() ?? this.ownMoveId();
      if (cursor && id) {
        this.tokenDrop.emit({ id, ...cursor });
        this.cursor.set(null);
      }
      return;
    }
    if (event.key === 'Escape') {
      this.cursor.set(null);
      return;
    }
    if (!event.key.startsWith('Arrow')) {
      return;
    }
    event.preventDefault();
    const base = this.pickSquares()
      ? (this.chosen()?.square ?? this.reach()?.origin ?? null)
      : (this.cursor() ?? this.tokenOf(this.picked() ?? this.ownMoveId() ?? this.currentId()));
    if (!base) {
      return;
    }
    const next = stepSquare(base, event.key, this.columns(), this.rows());
    if (this.pickSquares()) {
      this.choose.emit(next);
    } else {
      this.cursor.set(next);
    }
  }

  protected onBlur(): void {
    this.cursor.set(null);
  }
}
