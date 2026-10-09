import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { MatIconModule } from '@angular/material/icon';

import {
  AreaPlacement,
  type Combatant,
  type PreviewSpellAreaResponse,
  type TargetInReach,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { type AreaPick, samePick } from '../../core/combat/area-flow';
import {
  announcement,
  beforeHint,
  centerLabel,
  outOfRangeText,
  placedHint,
} from '../../core/combat/area-text';
import { type Square, arrowStep, squareAt, squareCenter } from '../../core/combat/combat-grid';
import { combatantInitial, isPlayer } from '../../core/combat/combat-view';
import { isCreature } from '../../core/combat/creature-names';
import {
  type AreaShape,
  areaSquares,
  beyondRange,
  cellsPath,
  outlinePath,
  rangeFt,
  rotate,
  toward,
} from '../../core/combat/spell-area';
import type { MapLayers } from '../../core/maps/layers';
import type { Vision } from '../../core/maps/vision';
import { metersFixed, metersText } from '../../core/units';
import { CombatMap, type CombatMapImage } from '../combat-map/combat-map';
import { CombatantToken } from '../combatant-token/combatant-token';
import { MapLayersLegend } from '../map-layers/map-layers-legend';

/** How far above the finger the template follows a touch, so the finger does not cover what is chosen (PM-02a). */
const FINGER_LIFT_PX = 44;
/** Sides of a square when a big map is zoomed in so that a finger hits one; the first step fits the map to the screen. */
const CELL_PX_NEAR = 22;
const CELL_PX_CLOSE = 30;
const CELL_PX_CLOSEST = 40;
const ZOOM_CELLS = [null, CELL_PX_NEAR, CELL_PX_CLOSE, CELL_PX_CLOSEST] as const;
/** A map wider than this many squares gets the zoom: its squares are too small to tap at the screen's width. */
const BIG_MAP_COLUMNS = 30;

/** The pause before a move is announced, so holding an arrow key does not flood the screen reader. */
const ANNOUNCE_DELAY_MS = 300;
/** Shift and an arrow move five squares. */
const SHIFT_STEP = 5;
/** Half a square: from a square's corner to its center, or to its side. */
const HALF = 0.5;

/** An entry of "Centrar em…": a creature the caster sees, the caster itself, or the map's own arrows. */
interface CenterOption {
  readonly id: string;
  readonly label: string;
  readonly initial: string;
  readonly npc: boolean;
  readonly creature: boolean;
  readonly detail: string;
  readonly square: Square | null;
}

let nextId = 0;

/**
 * Step 1 of an area spell on the map (PM-02a states 2 and 2b, PM-02b states 5 to 7, PM-02d state 11): the battle map with
 * the spell's template over it.
 *
 * - **Placing:** the template follows the pointer, and a finger 44 px above it; releasing (or a click) places the point:
 *   the template fills, the diamond marks the origin and a dashed line measures it from the caster. A second tap on the same
 *   point confirms. A cone, a line and a cube take the nearest of eight directions from the caster instead.
 * - **The server's word:** once a preview answers for the placed point (`preview`), its squares replace the outline and its
 *   point is the effective one; when a wall moved it, the tap is drawn as an ✕ and the origin where the server put it.
 * - **Range:** the squares beyond a point spell's range are dimmed. A player's tap there places nothing and says why; the
 *   master is not held to it, and the dimming is only a hint for him.
 * - **Keyboard:** the map is a focusable `role="application"`. Arrows move the point a square (Shift: five) or turn the
 *   direction 45°; Enter places, and on a placed point confirms; Escape cancels; C opens "Centrar em…" ("Apontar para…"), the
 *   listbox of the creatures the caster sees, the caster and "Um quadrado do mapa (use as setas)". Each move is announced
 *   politely after a 300 ms pause; a count of creatures only after a preview, never a name.
 *
 * Presentational: it reports places and confirmations; the page asks the server.
 */
@Component({
  selector: 'app-area-map',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CombatMap, CombatantToken, MapLayersLegend, MatIconModule, NgTemplateOutlet],
  templateUrl: './area-map.html',
  styleUrl: './area-map.scss',
})
export class AreaMap {
  private readonly injector = inject(Injector);

  readonly image = input.required<CombatMapImage>();
  readonly mapName = input('');
  readonly columns = input.required<number>();
  readonly rows = input.required<number>();
  readonly combatants = input<readonly Combatant[]>([]);
  readonly layers = input<MapLayers | null>(null);
  readonly fog = input<Vision | null>(null);
  /** The master: hidden creatures are drawn, and the range does not hold him. */
  readonly master = input(false);
  readonly casterId = input.required<string>();
  /** The spell's Portuguese name ("Bola de Fogo"). */
  readonly spell = input.required<string>();
  readonly placement = input.required<AreaPlacement>();
  readonly area = input.required<AreaShape>();
  /** How far a point may be from the caster, in feet; 0 for a shape that comes out of the caster. */
  readonly rangeFt = input(0);
  readonly placed = input<AreaPick | null>(null);
  readonly preview = input<PreviewSpellAreaResponse | null>(null);
  /** Step 2: the map only shows the area, and takes no tap. */
  readonly readOnly = input(false);
  /** The creatures the caster sees, with their distances (`SpellTargets.targets`): the list of "Centrar em…". */
  readonly seen = input<readonly TargetInReach[]>([]);
  /** The focus goes to the map when the step opens. */
  readonly autofocus = input(false);

  /** A point or a direction placed; `null` when a tap outside the range took the old one away. */
  readonly place = output<AreaPick | null>();
  /** A second tap on the same place, or Enter on it. */
  readonly confirm = output<void>();
  /** Escape on the map: back to the step before. */
  readonly dismiss = output<void>();

  protected readonly id = `area-map-${nextId++}`;
  private readonly app = viewChild<ElementRef<HTMLElement>>('app');
  private readonly listbox = viewChild<ElementRef<HTMLElement>>('listbox');
  private readonly centerButton = viewChild<ElementRef<HTMLButtonElement>>('center');

  /** The template under the pointer, the finger or the arrow keys, not placed yet. */
  protected readonly hover = signal<AreaPick | null>(null);
  /** The zoom step of a big map (0 fits it to the screen). */
  protected readonly zoom = signal(0);
  protected readonly lastZoom = ZOOM_CELLS.length - 1;
  protected readonly zoomable = computed(
    () => !this.readOnly() && this.columns() > BIG_MAP_COLUMNS,
  );
  protected readonly cell = computed(() => (this.zoomable() ? ZOOM_CELLS[this.zoom()] : null));
  /** The hover came from the keyboard, so the hint follows it (a mouse that only passes over does not rewrite it). */
  private readonly hoverByKey = signal(false);
  /** Where the finger is, in percent of the map, while it drags: the dotted line to the template above it. */
  protected readonly finger = signal<{ x: number; y: number } | null>(null);
  /** "Fora do alcance." and why, after a tap outside the range. */
  protected readonly refusal = signal('');
  protected readonly listOpen = signal(false);
  protected readonly active = signal(0);
  /** What the live region says, a pause after the last move. */
  protected readonly spoken = signal('');
  private announceTimer: ReturnType<typeof setTimeout> | null = null;
  private touching: number | null = null;

  protected readonly Placement = AreaPlacement;
  protected readonly isPoint = computed(() => this.placement() === AreaPlacement.POINT);
  protected readonly isDirection = computed(() => this.placement() === AreaPlacement.DIRECTION);

  protected readonly caster = computed<Square | null>(() => {
    const c = this.combatants().find((x) => x.id === this.casterId() && x.placed);
    return c ? { col: c.col, row: c.row } : null;
  });

  /** What the template shows: the hover, or what is placed. */
  protected readonly shown = computed(
    () => (this.readOnly() ? null : this.hover()) ?? this.placed(),
  );
  /** The template is the placed one (filled), not one that moves (dashed). */
  protected readonly filled = computed(() => {
    const placed = this.placed();
    return !!placed && samePick(this.shown(), placed);
  });

  /** The squares drawn: the server's, for the placed point it answered for; otherwise the browser's outline. */
  protected readonly squares = computed<readonly Square[]>(() => {
    const p = this.preview();
    if (p && this.filled()) {
      return p.squares.map((q) => ({ col: q.col, row: q.row }));
    }
    const pick = this.shown();
    const caster = this.caster();
    if (!pick) {
      return [];
    }
    if (pick.kind === 'point') {
      return areaSquares(this.area(), { origin: pick.square }, this.columns(), this.rows());
    }
    return caster
      ? areaSquares(this.area(), { caster, direction: pick.direction }, this.columns(), this.rows())
      : [];
  });
  protected readonly fillPath = computed(() => cellsPath(this.squares()));
  protected readonly edgePath = computed(() => outlinePath(this.squares()));

  /** The effective point of origin: the server's when it answered, else the point placed (or hovered). */
  protected readonly origin = computed<Square | null>(() => {
    const pick = this.shown();
    if (pick?.kind !== 'point') {
      return null;
    }
    const o = this.preview()?.origin;
    return this.filled() && o ? { col: o.col, row: o.row } : pick.square;
  });
  /** The point tapped when a wall moved it: drawn as an ✕ beside the real origin. */
  protected readonly tapped = computed<Square | null>(() => {
    const pick = this.placed();
    return this.filled() && this.preview()?.moved && pick?.kind === 'point' ? pick.square : null;
  });
  /** The diamond of a shape out of the caster: on the caster's side the area leaves from. */
  protected readonly edgeMark = computed(() => {
    const pick = this.shown();
    const caster = this.caster();
    if (pick?.kind !== 'direction' || !caster) {
      return null;
    }
    return {
      x: ((caster.col + HALF + pick.direction.dx * HALF) / this.columns()) * 100,
      y: ((caster.row + HALF + pick.direction.dy * HALF) / this.rows()) * 100,
    };
  });
  /** The dashed line from the caster to the point, with its length ("7,5 m"). */
  protected readonly line = computed(() => {
    const caster = this.caster();
    const origin = this.origin();
    if (!caster || !origin) {
      return null;
    }
    const a = squareCenter(caster, this.columns(), this.rows());
    const b = squareCenter(origin, this.columns(), this.rows());
    return {
      x1: a.x,
      y1: a.y,
      x2: b.x,
      y2: b.y,
      label: metersFixed(rangeFt(caster, origin)),
      mx: (a.x + b.x) / 2,
      my: (a.y + b.y) / 2,
    };
  });
  /** The squares beyond the range, dimmed (a point spell with a range). */
  protected readonly dimPath = computed(() => {
    const caster = this.caster();
    const range = this.rangeFt();
    return caster && this.isPoint() && range > 0
      ? cellsPath(beyondRange(caster, range, this.columns(), this.rows()))
      : '';
  });
  protected readonly rangeText = computed(() => metersText(this.rangeFt()));
  protected readonly hasHidden = computed(
    () => this.master() && this.combatants().some((c) => c.placed && c.hidden),
  );

  protected readonly hint = computed(() => {
    const pick = (this.hoverByKey() ? this.hover() : null) ?? this.placed();
    if (!pick) {
      return beforeHint(this.placement(), this.area());
    }
    return placedHint(
      this.placement(),
      this.distanceOf(pick),
      pick.kind === 'direction' ? pick.direction : null,
    );
  });
  protected readonly mapLabel = computed(() =>
    this.isDirection()
      ? `Mapa: escolha a direção da ${this.spell()}`
      : `Mapa: escolha o ponto da ${this.spell()}`,
  );
  protected readonly centerText = computed(() => centerLabel(this.placement()));

  /** "Centrar em…": the creatures the caster sees, nearest list order as the server gave it, then the caster and the arrows. */
  protected readonly options = computed<readonly CenterOption[]>(() => {
    const byId = new Map(this.combatants().map((c) => [c.id, c]));
    const out: CenterOption[] = [];
    for (const t of this.seen()) {
      const c = byId.get(t.combatantId);
      if (!c?.placed || t.combatantId === this.casterId()) {
        continue;
      }
      out.push({
        id: t.combatantId,
        label: t.label,
        initial: combatantInitial(t.label),
        npc: !isPlayer(c) && !isCreature(c),
        creature: isCreature(c),
        detail: t.distanceFt !== undefined ? `a ${metersFixed(t.distanceFt)}` : '',
        square: { col: c.col, row: c.row },
      });
    }
    const caster = byId.get(this.casterId());
    if (caster?.placed && this.isPoint()) {
      out.push({
        id: caster.id,
        label: `${caster.label} (você)`,
        initial: combatantInitial(caster.label),
        npc: !isPlayer(caster) && !isCreature(caster),
        creature: isCreature(caster),
        detail: 'aqui',
        square: { col: caster.col, row: caster.row },
      });
    }
    out.push({
      id: 'grid',
      label: 'Um quadrado do mapa (use as setas)',
      initial: '',
      npc: false,
      creature: false,
      detail: '',
      square: null,
    });
    return out;
  });

  constructor() {
    afterNextRender(() => {
      if (this.autofocus()) {
        this.app()?.nativeElement.focus();
      }
    });
    inject(DestroyRef).onDestroy(() => this.clearTimer());
    // The count of creatures is said once the server answered for the place on the map.
    effect(() => {
      const p = this.preview();
      if (p && untracked(() => this.filled())) {
        untracked(() => this.say(this.placed()));
      }
    });
  }

  // ---- what each place means ----

  private distanceOf(pick: AreaPick): number {
    const caster = this.caster();
    return pick.kind === 'point' && caster ? rangeFt(caster, pick.square) : 0;
  }

  /** The pick a square makes: the square itself for a point, the direction toward it for a shape out of the caster. */
  private pickAt(square: Square): AreaPick | null {
    if (this.isPoint()) {
      return { kind: 'point', square };
    }
    const caster = this.caster();
    const direction = caster ? toward(caster, square) : null;
    return direction ? { kind: 'direction', direction } : null;
  }

  /** A tap (or Enter, or a pick in the list) on a place: places it, confirms it when it is the placed one, or refuses it. */
  private tap(pick: AreaPick | null): void {
    if (!pick || this.readOnly()) {
      return;
    }
    const caster = this.caster();
    if (
      pick.kind === 'point' &&
      !this.master() &&
      caster &&
      this.rangeFt() > 0 &&
      rangeFt(caster, pick.square) > this.rangeFt()
    ) {
      const text = outOfRangeText(this.spell(), this.rangeFt(), rangeFt(caster, pick.square));
      this.refusal.set(text);
      this.hover.set(null);
      this.place.emit(null);
      this.speakNow(`Fora do alcance. ${text}`);
      return;
    }
    this.refusal.set('');
    this.hover.set(null);
    if (samePick(pick, this.placed())) {
      this.confirm.emit();
      return;
    }
    this.place.emit(pick);
    this.say(pick);
  }

  // ---- pointer ----

  private squareUnder(event: PointerEvent, lift: number): Square {
    const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
    return squareAt(
      (event.clientX - rect.left) / Math.max(rect.width, 1),
      (event.clientY - lift - rect.top) / Math.max(rect.height, 1),
      this.columns(),
      this.rows(),
    );
  }

  private fingerAt(event: PointerEvent): { x: number; y: number } {
    const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
    return {
      x: ((event.clientX - rect.left) / Math.max(rect.width, 1)) * 100,
      y: ((event.clientY - rect.top) / Math.max(rect.height, 1)) * 100,
    };
  }

  private static touchLike(event: PointerEvent): boolean {
    return event.pointerType === 'touch' || event.pointerType === 'pen';
  }

  protected onPointerDown(event: PointerEvent): void {
    if (this.readOnly() || !AreaMap.touchLike(event)) {
      return;
    }
    this.touching = event.pointerId;
    (event.currentTarget as HTMLElement).setPointerCapture?.(event.pointerId);
    this.setHover(this.pickAt(this.squareUnder(event, FINGER_LIFT_PX)), false);
    this.finger.set(this.fingerAt(event));
  }

  protected onPointerMove(event: PointerEvent): void {
    if (this.readOnly()) {
      return;
    }
    if (AreaMap.touchLike(event)) {
      if (this.touching === event.pointerId) {
        this.setHover(this.pickAt(this.squareUnder(event, FINGER_LIFT_PX)), false);
        this.finger.set(this.fingerAt(event));
      }
      return;
    }
    const pick = this.pickAt(this.squareUnder(event, 0));
    if (!samePick(pick, this.hover())) {
      this.setHover(pick, false);
    }
  }

  protected onPointerUp(event: PointerEvent): void {
    if (this.readOnly()) {
      return;
    }
    if (AreaMap.touchLike(event)) {
      if (this.touching !== event.pointerId) {
        return;
      }
      this.touching = null;
      this.finger.set(null);
      this.tap(this.pickAt(this.squareUnder(event, FINGER_LIFT_PX)));
      return;
    }
    if (event.button > 0) {
      return;
    }
    this.tap(this.pickAt(this.squareUnder(event, 0)));
  }

  protected onPointerLeave(event: PointerEvent): void {
    if (!AreaMap.touchLike(event)) {
      this.hover.set(null);
    }
  }

  protected onPointerCancel(): void {
    this.touching = null;
    this.finger.set(null);
    this.hover.set(null);
  }

  // ---- keyboard ----

  protected onKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      this.hover.set(null);
      this.dismiss.emit();
      return;
    }
    if (this.readOnly()) {
      return;
    }
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      this.tap(this.hover() ?? this.placed());
      return;
    }
    if (event.key === 'c' || event.key === 'C') {
      event.preventDefault();
      this.openList();
      return;
    }
    const step = arrowStep(event.key);
    if (!step) {
      return;
    }
    event.preventDefault();
    const next = this.isPoint()
      ? this.movedPoint(step, event.shiftKey ? SHIFT_STEP : 1)
      : this.turned(step.dc + step.dr);
    if (next) {
      this.setHover(next, true);
      this.say(next);
    }
  }

  protected zoomBy(delta: 1 | -1): void {
    this.zoom.update((z) => Math.min(this.lastZoom, Math.max(0, z + delta)));
  }

  private setHover(pick: AreaPick | null, byKey: boolean): void {
    this.hoverByKey.set(byKey);
    this.hover.set(pick);
  }

  /** The point the arrows move to: from the hover, the placed point or the caster, kept on the map. */
  private movedPoint(step: { dc: number; dr: number }, n: number): AreaPick | null {
    const from = this.hover() ?? this.placed();
    const base = from?.kind === 'point' ? from.square : (this.caster() ?? { col: 0, row: 0 });
    return {
      kind: 'point',
      square: {
        col: Math.min(this.columns() - 1, Math.max(0, base.col + step.dc * n)),
        row: Math.min(this.rows() - 1, Math.max(0, base.row + step.dr * n)),
      },
    };
  }

  /** The direction turned 45° (right and down clockwise, left and up the other way). */
  private turned(turn: number): AreaPick {
    const from = this.hover() ?? this.placed();
    const base = from?.kind === 'direction' ? from.direction : null;
    return {
      kind: 'direction',
      direction: base ? rotate(base, turn) : { dx: 1, dy: 0 },
    };
  }

  /** "Ajustar o ponto, um quadrado por toque": moves the placed point one square, as a tap there. */
  protected nudge(dc: number, dr: number): void {
    const pick = this.placed();
    if (pick?.kind !== 'point') {
      return;
    }
    const square = {
      col: Math.min(this.columns() - 1, Math.max(0, pick.square.col + dc)),
      row: Math.min(this.rows() - 1, Math.max(0, pick.square.row + dr)),
    };
    this.tap({ kind: 'point', square });
  }

  // ---- "Centrar em…" ----

  protected openList(): void {
    if (this.readOnly()) {
      return;
    }
    this.listOpen.set(true);
    this.active.set(0);
    afterNextRender(() => this.listbox()?.nativeElement.focus(), { injector: this.injector });
  }

  protected toggleList(): void {
    if (this.listOpen()) {
      this.closeList(true);
    } else {
      this.openList();
    }
  }

  private closeList(toButton: boolean): void {
    this.listOpen.set(false);
    afterNextRender(
      () => (toButton ? this.centerButton()?.nativeElement : this.app()?.nativeElement)?.focus(),
      { injector: this.injector },
    );
  }

  protected onListKeydown(event: KeyboardEvent): void {
    const n = this.options().length;
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        this.active.set((this.active() + 1) % n);
        break;
      case 'ArrowUp':
        event.preventDefault();
        this.active.set((this.active() - 1 + n) % n);
        break;
      case 'Home':
        event.preventDefault();
        this.active.set(0);
        break;
      case 'End':
        event.preventDefault();
        this.active.set(n - 1);
        break;
      case 'Enter':
      case ' ':
        event.preventDefault();
        this.choose(this.options()[this.active()]);
        break;
      case 'Escape':
        event.preventDefault();
        event.stopPropagation();
        this.closeList(true);
        break;
      default:
    }
  }

  /** A click on an option of the list (the listbox handles the pointer for its options, which take no focus of their own). */
  protected onListClick(event: MouseEvent): void {
    const i = Number((event.target as HTMLElement).closest<HTMLElement>('[data-i]')?.dataset['i']);
    const option = Number.isInteger(i) ? this.options()[i] : undefined;
    if (option) {
      this.choose(option);
    }
  }

  /** A pick in the list places exactly like a tap on that square; "Um quadrado do mapa" hands the arrows the map. */
  protected choose(option: CenterOption): void {
    this.closeList(false);
    if (option.square) {
      this.tap(this.pickAt(option.square));
    }
  }

  // ---- the live region ----

  private say(pick: AreaPick | null): void {
    if (!pick) {
      return;
    }
    const p = this.preview();
    const seen = p && samePick(pick, this.placed()) ? p.targets.length : null;
    this.clearTimer();
    const text = announcement(
      this.placement(),
      this.distanceOf(pick),
      pick.kind === 'direction' ? pick.direction : null,
      seen,
    );
    this.announceTimer = setTimeout(() => this.spoken.set(text), ANNOUNCE_DELAY_MS);
  }

  private speakNow(text: string): void {
    this.clearTimer();
    this.spoken.set(text);
  }

  private clearTimer(): void {
    if (this.announceTimer !== null) {
      clearTimeout(this.announceTimer);
      this.announceTimer = null;
    }
  }

  // ---- drawing ----

  protected pct(n: number, of: number): number {
    return (n / of) * 100;
  }
}
