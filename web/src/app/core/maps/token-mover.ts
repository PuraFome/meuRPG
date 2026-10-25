import { computed, signal } from '@angular/core';

import type { MapToken } from '../../../gen/meurpg/maps/v1/maps_pb';
import { moveErrorMessage } from './map-errors';
import type { MapState } from './map-state';
import {
  DIRECTIONS,
  type Direction,
  type Grid,
  MAX_STEP,
  MIN_STEP,
  type MoveNotice,
  type MoveOutcome,
  type Point,
  type TokenMoveResult,
  askedLine,
  centerOf,
  metersOf,
  moveNotice,
  squareAt,
  squaresBetween,
  stepsWords,
  stopLabel,
  targetAfter,
} from './token-move';

/** The one call the mover makes (`MapsClient.moveTokens`). */
export interface MoveApi {
  moveTokens(
    campaignId: string,
    mapId: string,
    characterIds: readonly string[],
    xBp: number,
    yBp: number,
  ): Promise<MoveOutcome>;
}

/** What the map draws for a move: where a token goes (the plan) or where it got (the result). */
export interface MoveTrail {
  readonly from: Point;
  readonly to: Point;
  /** After a move that stopped early: the label at the end of the line and the X where it was meant to go. */
  readonly stop?: { readonly label: string; readonly at: Point };
}

/**
 * The master's moving of tokens outside combat (B2, MoveMapTokens): the selection, the direction and the
 * "Quadrados por clique", the destination, the call and what it said. Pure TypeScript over signals, so the
 * rules are tested without a DOM:
 *
 * - a token walks toward the destination and may stop before it (a wall, a marked square, a locked door); the
 *   server decides and says why, and nothing moves on a guess: the tokens change only with the answer;
 * - several tokens move together around the first (the anchor, which a drag makes the dragged one);
 * - a creature's token is not a character's and never joins a selection (it moves by drag only);
 * - one move at a time.
 */
export class TokenMover {
  /** The characters chosen, the anchor first. */
  readonly selected = signal<readonly string[]>([]);
  /** "Selecionar": a tap on a token adds or takes it, instead of choosing only it. */
  readonly selecting = signal(false);
  readonly steps = signal(MIN_STEP);
  readonly direction = signal<Direction | null>(null);
  /** The square the move heads for, when one was chosen (a tap on the map or a direction). */
  readonly target = signal<Point | null>(null);
  readonly busy = signal(false);
  readonly error = signal('');
  /** What the last move said, until the next choice. */
  readonly last = signal<{
    readonly notice: MoveNotice;
    readonly lines: readonly string[];
    readonly trail: MoveTrail | null;
  } | null>(null);
  /** What a screen reader hears: who moved and how far. */
  readonly status = signal('');

  private readonly names = computed(
    () => new Map(this.state.tokens().map((t) => [t.characterId, t.name])),
  );
  /** The characters' tokens on the map, in the list's order; a creature's is not one. */
  readonly characters = computed(() => this.state.tokens().filter((t) => !t.creatureId));
  /** The tokens chosen, the anchor first. */
  readonly chosen = computed(() =>
    this.selected()
      .map((id) => this.characters().find((t) => t.characterId === id))
      .filter((t): t is MapToken => t !== undefined),
  );
  readonly anchor = computed<MapToken | null>(() => this.chosen()[0] ?? null);
  readonly grid = computed<Grid>(() => ({
    columns: this.state.map()?.gridColumns ?? 0,
    rows: this.state.map()?.gridRows ?? 0,
  }));
  /** Moving needs a grid: the server counts squares. */
  readonly hasGrid = computed(() => this.grid().columns > 0 && this.grid().rows > 0);

  /** "Tor vai 3 quadrados para a direita" / "Destino: 3 quadrados à direita", and the distance. */
  readonly summary = computed(() => {
    const anchor = this.anchor();
    const target = this.target();
    if (!anchor || !target) {
      return null;
    }
    const grid = this.grid();
    const squares = squaresBetween(anchor, target, grid);
    const words = stepsWords(anchor, target, grid);
    const several = this.chosen().length > 1;
    return {
      title: several ? `Destino: ${words}` : `${anchor.name} vai ${words}`,
      detail: several
        ? `${metersOf(squares)} · toque num quadrado do mapa para mudar`
        : `Distância: ${metersOf(squares)}`,
    };
  });

  /** What the map draws: a ring on each chosen token, the plan, the last move's line. */
  readonly rings = computed<readonly Point[]>(() =>
    this.chosen().map((t) => ({ xBp: t.xBp, yBp: t.yBp })),
  );
  readonly plan = computed<MoveTrail | null>(() => {
    const anchor = this.anchor();
    const target = this.target();
    return anchor && target ? { from: anchor, to: target } : null;
  });
  readonly trail = computed<MoveTrail | null>(() => this.last()?.trail ?? null);

  constructor(
    private readonly api: MoveApi,
    private readonly state: MapState,
    private readonly campaignId: () => string,
  ) {}

  isSelected(characterId: string): boolean {
    return this.selected().includes(characterId);
  }

  /** A tap on a token or its row: only it, or, when `additive` (Shift, or "Selecionar" is on), added or taken from the selection. */
  pick(characterId: string, additive = false): void {
    if (!this.characters().some((t) => t.characterId === characterId)) {
      return;
    }
    this.forgetMove();
    if (additive || this.selecting()) {
      this.selected.update((list) =>
        list.includes(characterId)
          ? list.filter((id) => id !== characterId)
          : [...list, characterId],
      );
    } else {
      this.selected.set(
        this.selected().length === 1 && this.isSelected(characterId) ? [] : [characterId],
      );
    }
    this.afterSelection();
  }

  clear(): void {
    this.selected.set([]);
    this.afterSelection();
  }

  setSelecting(on: boolean): void {
    this.selecting.set(on);
  }

  /** A pad button: the direction, `steps` squares from the anchor. */
  choose(direction: Direction): void {
    this.forgetMove();
    this.direction.set(direction);
    this.retarget();
  }

  setSteps(steps: number): void {
    this.steps.set(Math.max(MIN_STEP, Math.min(MAX_STEP, Math.round(steps))));
    this.retarget();
  }

  /** A tap on a square of the map: the destination. */
  tap(point: Point): void {
    if (!this.anchor() || !this.hasGrid()) {
      return;
    }
    this.forgetMove();
    const at = squareAt(point, this.grid());
    this.direction.set(null);
    this.target.set(centerOf(at.col, at.row, this.grid()));
  }

  /** "Mover Tor" / "Mover os 3 juntos": the move the panel set up. */
  async move(): Promise<boolean> {
    const target = this.target();
    if (!target || this.chosen().length === 0) {
      return false;
    }
    return this.run(
      this.chosen().map((t) => t.characterId),
      target,
    );
  }

  /** A token dragged on the map (it keeps its group when it is chosen with others). */
  async drag(characterId: string, to: Point): Promise<boolean> {
    const group = this.isSelected(characterId) ? this.selected() : [characterId];
    return this.run([characterId, ...group.filter((id) => id !== characterId)], to);
  }

  private retarget(): void {
    const anchor = this.anchor();
    const dir = DIRECTIONS.find((d) => d.id === this.direction());
    this.target.set(
      anchor && dir && this.hasGrid() ? targetAfter(anchor, dir, this.steps(), this.grid()) : null,
    );
  }

  private afterSelection(): void {
    this.target.set(null);
    this.direction.set(null);
  }

  private forgetMove(): void {
    this.last.set(null);
    this.error.set('');
  }

  private async run(ids: readonly string[], to: Point): Promise<boolean> {
    const mapId = this.state.map()?.id;
    if (this.busy() || !mapId) {
      return false;
    }
    const anchor = this.characters().find((t) => t.characterId === ids[0]);
    const who = anchor?.name ?? 'o token';
    this.busy.set(true);
    this.error.set('');
    this.last.set(null);
    try {
      const outcome = await this.api.moveTokens(this.campaignId(), mapId, ids, to.xBp, to.yBp);
      this.apply(outcome);
      this.say(outcome, anchor, to);
      return outcome.moved;
    } catch (err) {
      this.error.set(moveErrorMessage(err, who));
      void this.state.refresh();
      return false;
    } finally {
      this.busy.set(false);
    }
  }

  /** The tokens as they are now: always a new object, so a map that showed the drop spot lets go of it. */
  private apply(outcome: MoveOutcome): void {
    for (const r of outcome.results) {
      const token = this.state
        .tokens()
        .find((t) => !t.creatureId && t.characterId === r.characterId);
      if (token) {
        this.state.upsertToken({ ...token, xBp: r.xBp, yBp: r.yBp });
      }
    }
  }

  private say(outcome: MoveOutcome, anchor: MapToken | undefined, to: Point): void {
    const notice = moveNotice(outcome, this.names());
    const first: TokenMoveResult | undefined = outcome.results[0];
    const short = outcome.results.find((r) => r.moved < r.asked);
    const from = anchor ? { xBp: anchor.xBp, yBp: anchor.yBp } : to;
    const trail: MoveTrail | null =
      first && outcome.moved
        ? {
            from,
            to: { xBp: first.xBp, yBp: first.yBp },
            ...(short ? { stop: { label: stopLabel(short), at: to } } : {}),
          }
        : null;
    const lines = outcome.moved && first && outcome.results.length === 1 ? [askedLine(first)] : [];
    this.last.set({ notice, lines, trail });
    this.target.set(null);
    this.direction.set(null);
    this.status.set(`${notice.lead} ${notice.text}`.trim());
  }
}
