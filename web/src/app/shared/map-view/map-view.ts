import {
  Component,
  DestroyRef,
  ElementRef,
  afterNextRender,
  afterRenderEffect,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import {
  IDENTITY,
  ViewPoint,
  ViewToken,
  ViewTransform,
  bpToPercent,
  centroid,
  clampTransform,
  focusTransform,
  labelSide,
  squareKey,
  nudge,
  screenToBp,
  stepScale,
  tokenKey,
  tokenInitial,
  visiblePoints,
  visibleTokens,
  zoomAround,
} from './map-geometry';
import { pointHidden } from './map-labels';
import { MapGestures } from './map-gestures';
import { placeLabels } from './map-label-placer';
import { MapMarker } from './map-marker/map-marker';
import { MapToken } from './map-token/map-token';
import { RetryImage } from '../retry-image/retry-image';
import { fullscreenOfHost } from '../../core/ui/fullscreen';
import { RelayEvents } from '../relay-events/relay-events';

/** `preview`: a still picture (the session page's card); `view`: pan and
 * zoom, points open; `tokens`: also drag the tokens (the master's session
 * page); `edit`: also drag the points (the editor). */
export type MapViewMode = 'preview' | 'view' | 'tokens' | 'edit';

export interface MapImageRef {
  readonly url: string;
  readonly width: number;
  readonly height: number;
}

/** What is selected on the map: a point by its ID, a token by its
 * character's ID. */
export interface MapSelection {
  readonly kind: 'point' | 'token';
  readonly id: string;
}

export interface MapMove extends MapSelection {
  readonly xBp: number;
  readonly yBp: number;
}

/** The points on one square, named by one label. */
interface LabelGroup {
  readonly id: string;
  readonly ids: readonly string[];
  readonly text: string;
  readonly hidden: boolean;
  readonly at: { xBp: number; yBp: number };
}

/** On a map narrower than 520 px the names show from this zoom (below it they would cover the map). */
const COMPACT_LABELS_FROM_SCALE = 1.5;

/** The phone preview opens zoomed in on the party (README-A). */
const PREVIEW_SCALE = 2;

/**
 * The map: the gallery image with the points and tokens on top, shared by
 * the editor, the player's viewer and the session page (README-B, README-A).
 *
 * - **Layout.** The image is sized from `image.width/height` (an aspect-ratio
 *   box), so nothing shifts while it loads. Items sit at their basis points
 *   as percentages, so they stay on the same spot of the image at any size.
 * - **Pan and zoom** without a library: the stage gets a CSS transform.
 *   The mouse drags an empty spot to pan and the wheel zooms around the
 *   pointer; two fingers pan and pinch (one finger still scrolls the page).
 *   The overlay buttons (−, +, "Ajustar à tela", 44px) work for everyone;
 *   100 % is the whole image, up to 400 %. A parent that draws its own
 *   toolbar calls `zoomIn()`, `zoomOut()` and `fit()` and reads `scale()`.
 * - **Dragging** (`edit`: points and tokens, `tokens`: tokens). The arrow
 *   keys move the selected item 0,5 % (Shift: 5 %). The position is
 *   reported on drop or on key release (`moved`); the parent saves it.
 * - **Who sees what.** The master sees every point and token, hidden ones
 *   drawn dashed. Anyone else gets only the revealed and the visible, even
 *   if a hidden one is passed in (the server never sends one).
 * - **Stacking:** image, points, tokens, labels, the selected or hovered
 *   item, the controls. Hover and focus raise an overlapped item.
 *
 * Presentational and signal-based; it never calls the API.
 */
@Component({
  selector: 'app-map-view',
  imports: [RelayEvents, MapMarker, MapToken, MatIconModule, RetryImage],
  templateUrl: './map-view.html',
  styleUrl: './map-view.scss',
})
export class MapView {
  protected readonly fullscreen = fullscreenOfHost();
  readonly image = input.required<MapImageRef>();
  /** The map's name: the group's accessible name. */
  readonly mapName = input('');
  readonly points = input<readonly ViewPoint[]>([]);
  readonly tokens = input<readonly ViewToken[]>([]);
  readonly isMaster = input(false);
  readonly mode = input<MapViewMode>('view');
  /** Whether a click on a marker opens it. Off on the master's phone map,
   * which only pans and zooms. */
  readonly selectablePoints = input(true);
  readonly selected = input<MapSelection | null>(null);
  readonly controls = input<'overlay' | 'none'>('overlay');
  /** Opens zoomed in on the party and the points (the phone's preview). */
  readonly focusParty = input(false);
  /** A line over the map's top-left corner ("Clique no mapa para pôr o ponto."). */
  readonly hint = input<string | null>(null);
  /** A kind of point is waiting for a click: the cursor says so. */
  readonly placing = input(false);
  /** NPCs as white rounded squares and creatures with a dashed ring (the fog map, docs/design/README.md). */
  readonly kindShapes = input(false);
  /** The screen draws traps, chests and lights itself (`app-map-pins`): their markers are then only hit areas. */
  readonly pinsDrawn = input(false);
  /** Names beside the points. Off while the master paints: the labels would cover what he paints. */
  readonly labels = input(true);
  /** Markers and tokens drawn at 40 %, so what is painted shows through (the editor while painting). */
  readonly faded = input(false);
  /** The grid's columns, when the map has one: the tokens are then sized to the square (a fog map, docs/design/README.md). */
  readonly squares = input(0);
  /** Opens zoomed in on this spot, once the view has its size (the phone's fog map: the party at 2x). Read again only when it changes to another spot. */
  readonly startAt = input<{ xBp: number; yBp: number } | null>(null);
  /** The initial of a token: `tokenInitial` by default; the fog map writes an NPC's number too ("G2"). */
  readonly initialOf = input<((token: ViewToken, all: readonly ViewToken[]) => string) | null>(
    null,
  );

  readonly pointSelect = output<string>();
  readonly tokenSelect = output<string>();
  /** The same click with its modifier: Shift (or Ctrl, or Cmd) adds the token to a selection instead of choosing only it (B2). */
  readonly tokenPick = output<{ id: string; additive: boolean }>();
  /** A click on an empty spot of the map, where it fell. */
  readonly emptyClick = output<{ xBp: number; yBp: number }>();
  readonly moved = output<MapMove>();

  private readonly viewport = viewChild.required<ElementRef<HTMLElement>>('viewport');

  protected readonly transform = signal<ViewTransform>(IDENTITY);
  protected readonly size = signal({ width: 0, height: 0 });
  /** What the pointer or the keyboard is moving right now. */
  private readonly override = signal<MapMove | null>(null);
  /** The last position this view reported (`moved`), until the parent's next
   * `points`/`tokens` arrive. The parent saves the move and updates its state
   * at once, but the new input only reaches this view on the next change
   * detection; a second arrow key pressed before that would start again from
   * the old position and send the same move twice (seen in CI on #44 and
   * #47). It is also where the item is drawn meanwhile, so it doesn't jump
   * back for a frame. */
  private readonly settled = signal<MapMove | null>(null);
  protected readonly raisedKey = signal<string | null>(null);

  /** The zoom, 1 to 4 (the editor's toolbar reads it). */
  readonly scale = computed(() => this.transform().scale);

  protected readonly shownPoints = computed(() => visiblePoints(this.points(), this.isMaster()));
  protected readonly shownTokens = computed(() => visibleTokens(this.tokens(), this.isMaster()));
  /** One label per square: the names of the points that share it, side by side in one pill. */
  protected readonly labelGroups = computed<readonly LabelGroup[]>(() => {
    if (!this.labels()) {
      return [];
    }
    const groups = new Map<string, ViewPoint[]>();
    for (const p of this.shownPoints()) {
      const at = this.pointAt(p) ?? p;
      const key = squareKey(at.xBp, at.yBp, this.squares(), this.ratio());
      groups.set(key, [...(groups.get(key) ?? []), p]);
    }
    // A small map (a phone) seen whole has no room for names: the markers and the legend say what is there, and the list under the map
    // names each point. The name of the point you chose, and every name once you zoom in, still show.
    const crowded =
      this.size().width > 0 && this.size().width < 520 && this.scale() < COMPACT_LABELS_FROM_SCALE;
    const kept = crowded
      ? [...groups.values()].filter((members) =>
          members.some(
            (p) => this.selectedKey() === 'point:' + p.id || this.raisedKey() === 'point:' + p.id,
          ),
        )
      : [...groups.values()];
    return kept.map((members) => ({
      id: members[0].id,
      ids: members.map((p) => p.id),
      text: members.map((p) => p.name).join(' · '),
      hidden: members.every((p) => pointHidden(p)),
      at: this.pointAt(members[0]) ?? members[0],
    }));
  });
  protected readonly interactive = computed(() => this.mode() !== 'preview');
  protected readonly pointsOpen = computed(() => this.interactive() && this.selectablePoints());
  protected readonly tokensMovable = computed(
    () => this.mode() === 'tokens' || this.mode() === 'edit',
  );
  protected readonly ratio = computed(() => this.image().width / Math.max(1, this.image().height));
  protected readonly aspect = computed(() => `${this.image().width} / ${this.image().height}`);
  protected readonly selectedKey = computed(() => {
    const s = this.selected();
    return s ? `${s.kind}:${s.id}` : null;
  });

  private readonly gestures = new MapGestures({
    interactive: () => this.interactive(),
    viewport: () => this.viewport().nativeElement,
    transform: this.transform,
    size: () => this.size(),
    override: this.override,
    movable: (item) => this.movable(item),
    itemOf: (target) => this.itemOf(target),
    report: (move) => this.report(move),
    emptyClick: (at) => this.emptyClick.emit(at),
  });
  private nudging = false;
  private startedAt = '';

  constructor() {
    // Read here: inject() only works while the component is being built, not
    // inside the render callback below (NG0203).
    const destroyRef = inject(DestroyRef);
    afterNextRender(() => {
      const el = this.viewport().nativeElement;
      const measure = () => {
        const rect = el.getBoundingClientRect();
        this.size.set({ width: rect.width, height: rect.height });
        this.transform.update((t) => clampTransform(t, rect.width, rect.height));
      };
      measure();
      // The labels are measured again once the fonts are in: before, their
      // text has the fallback font's width.
      void document.fonts?.ready.then(() => this.size.update((sz) => ({ ...sz })));
      if (typeof ResizeObserver !== 'undefined') {
        const observer = new ResizeObserver(measure);
        observer.observe(el);
        destroyRef.onDestroy(() => observer.disconnect());
      }
    });

    // Put every label beside what it names, off the other labels, areas, markers and tokens, and on the image: measured on screen
    // after each render, written straight to the pills (the place is layout, not state).
    afterRenderEffect(() => {
      this.size();
      this.transform();
      this.labelGroups();
      this.shownTokens();
      this.settled();
      this.override();
      this.faded();
      placeLabels(this.viewport().nativeElement);
    });

    // New points or tokens from the parent replace what this view last
    // reported: either they carry the move, or the parent undid it (the
    // server refused).
    effect(() => {
      this.points();
      this.tokens();
      untracked(() => this.settled.set(null));
    });

    // The map opens on a spot the screen chose, once, until the spot changes (it never pulls the map back while it is used).
    effect(() => {
      const spot = this.startAt();
      const { width, height } = this.size();
      if (!spot || width === 0) {
        return;
      }
      untracked(() => {
        const key = `${spot.xBp}:${spot.yBp}`;
        if (this.startedAt !== key) {
          this.startedAt = key;
          this.transform.set(focusTransform(spot, PREVIEW_SCALE, width, height));
        }
      });
    });

    // The phone's preview opens on the party, zoomed in.
    effect(() => {
      const { width, height } = this.size();
      if (!this.focusParty() || width === 0) {
        return;
      }
      const around = this.shownTokens().length > 0 ? this.shownTokens() : this.shownPoints();
      const center = centroid(around);
      untracked(() =>
        this.transform.set(
          center ? focusTransform(center, PREVIEW_SCALE, width, height) : IDENTITY,
        ),
      );
    });
  }

  // ---- the toolbar's calls ----

  zoomIn(): void {
    this.zoomBy(1);
  }

  zoomOut(): void {
    this.zoomBy(-1);
  }

  /** "Ajustar à tela": the whole image. */
  fit(): void {
    this.transform.set(IDENTITY);
  }

  /** Zooms in on a spot (a basis-point position), as the chips of the party do on a phone. */
  focusOn(at: { xBp: number; yBp: number }, scale = PREVIEW_SCALE): void {
    const { width, height } = this.size();
    if (width > 0) {
      this.transform.set(focusTransform(at, scale, width, height));
    }
  }

  /** Puts focus back on an item (a point's marker after its sheet closes). */
  focusItem(kind: 'point' | 'token', id: string): void {
    this.viewport()
      .nativeElement.querySelector<HTMLElement>(`[data-item="${kind}:${id}"]`)
      ?.focus();
  }

  /** The middle of what is on screen, in basis points: where a new token
   * lands ("Adicionar token"). */
  centerBp(): { xBp: number; yBp: number } {
    const rect = this.viewport().nativeElement.getBoundingClientRect();
    return screenToBp(
      rect.left + rect.width / 2,
      rect.top + rect.height / 2,
      rect,
      this.transform(),
    );
  }

  private zoomBy(direction: 1 | -1): void {
    const { width, height } = this.size();
    const t = this.transform();
    this.transform.set(
      zoomAround(t, stepScale(t.scale, direction), width / 2, height / 2, width, height),
    );
  }

  // ---- where things are drawn ----

  protected readonly tokenKey = tokenKey;

  protected pointAt(point: ViewPoint): { xBp: number; yBp: number } | null {
    return this.movedTo('point', point.id);
  }

  protected tokenAt(token: ViewToken): { xBp: number; yBp: number } | null {
    return this.movedTo('token', tokenKey(token));
  }

  /** Where an item is while it moves, or right after, until the parent's
   * new input arrives; `null` to draw it where the input says. */
  private movedTo(kind: 'point' | 'token', id: string): MapMove | null {
    for (const m of [this.override(), this.settled()]) {
      if (m && m.kind === kind && m.id === id) {
        return m;
      }
    }
    return null;
  }

  protected labelLeft(at: { xBp: number }): number {
    return bpToPercent(at.xBp);
  }

  protected labelTop(at: { yBp: number }): number {
    return bpToPercent(at.yBp);
  }

  /** Where the label sits until it is measured (and in a test, which has no layout). */
  protected side(at: { xBp: number; yBp: number }): string {
    return labelSide(at.xBp, at.yBp);
  }

  protected groupSelected(g: LabelGroup): boolean {
    return g.ids.some((id) => this.selectedKey() === 'point:' + id);
  }

  protected groupRaised(g: LabelGroup): boolean {
    return g.ids.some((id) => this.raisedKey() === 'point:' + id);
  }

  protected initial(token: ViewToken): string {
    return (this.initialOf() ?? tokenInitial)(token, this.shownTokens());
  }

  protected transformCss(): string {
    const t = this.transform();
    return `translate(${t.x}px, ${t.y}px) scale(${t.scale})`;
  }

  // ---- pointers ----

  protected onPointerDown(event: PointerEvent): void {
    this.gestures.down(event);
  }

  protected onPointerMove(event: PointerEvent): void {
    this.gestures.move(event);
  }

  protected onPointerUp(event: PointerEvent): void {
    this.gestures.up(event);
  }

  protected onWheel(event: WheelEvent): void {
    this.gestures.wheel(event);
  }

  // ---- clicks, keys, hover ----

  protected onClick(event: MouseEvent): void {
    const item = this.itemOf(event.target);
    if (!item) {
      return;
    }
    const [kind, id] = splitItem(item);
    if (kind === 'point' && this.pointsOpen()) {
      this.pointSelect.emit(id);
    } else if (kind === 'token' && this.tokensMovable()) {
      this.tokenSelect.emit(id);
      this.tokenPick.emit({ id, additive: event.shiftKey || event.ctrlKey || event.metaKey });
    }
  }

  protected onKeydown(event: KeyboardEvent): void {
    const item = this.itemOf(event.target);
    const target = this.movable(item);
    if (!target) {
      return;
    }
    const from = this.override() ?? target;
    const to = nudge(event.key, event.shiftKey, from.xBp, from.yBp);
    if (to) {
      event.preventDefault();
      this.nudging = true;
      this.override.set({ ...target, ...to });
    }
  }

  protected onKeyup(event: KeyboardEvent): void {
    if (!this.nudging || !event.key.startsWith('Arrow')) {
      return;
    }
    this.nudging = false;
    const done = this.override();
    this.override.set(null);
    if (done) {
      this.report(done);
    }
  }

  /** Tells the parent where an item went, and remembers it until the parent's
   * new input arrives (see `settled`). */
  private report(move: MapMove): void {
    this.settled.set(move);
    this.moved.emit(move);
  }

  protected raise(event: Event): void {
    this.raisedKey.set(this.itemOf(event.target));
  }

  protected lower(): void {
    this.raisedKey.set(null);
  }

  // ---- helpers ----

  private itemOf(target: EventTarget | null): string | null {
    return (
      (target as HTMLElement | null)?.closest?.('[data-item]')?.getAttribute('data-item') ?? null
    );
  }

  /** The item as something the mode lets the person move, with its
   * current position; `null` otherwise. */
  private movable(item: string | null): MapMove | null {
    if (!item) {
      return null;
    }
    const [kind, id] = splitItem(item);
    if (kind === 'point' && this.mode() === 'edit') {
      const p = this.points().find((x) => x.id === id);
      return p ? (this.movedTo(kind, id) ?? { kind, id, xBp: p.xBp, yBp: p.yBp }) : null;
    }
    if (kind === 'token' && this.tokensMovable()) {
      const t = this.tokens().find((x) => tokenKey(x) === id);
      return t ? (this.movedTo(kind, id) ?? { kind, id, xBp: t.xBp, yBp: t.yBp }) : null;
    }
    return null;
  }
}

function splitItem(item: string): ['point' | 'token', string] {
  const index = item.indexOf(':');
  return [item.slice(0, index) as 'point' | 'token', item.slice(index + 1)];
}
