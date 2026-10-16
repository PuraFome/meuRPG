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
import { type Square, arrowStep } from '../../core/combat/combat-grid';
import {
  type AreaShape,
  areaSquares,
  beyondRange,
  cellsPath,
  outlinePath,
  rangeFt,
  toward,
} from '../../core/combat/spell-area';
import type { MapLayers } from '../../core/maps/layers';
import type { Vision } from '../../core/maps/vision';
import { metersText } from '../../core/units';
import { CombatMap, type CombatMapImage } from '../combat-map/combat-map';
import { CombatantToken } from '../combatant-token/combatant-token';
import { MapLayersLegend } from '../map-layers/map-layers-legend';
import { AreaAnnouncer } from './area-announcer';
import {
  BIG_MAP_COLUMNS,
  ZOOM_CELLS,
  areaLine,
  clampSquare,
  edgeMarkOf,
  listMove,
  movedPoint,
  squareUnder,
  turnedDirection,
} from './area-moves';
import { AreaPointer } from './area-pointer';
import { type CenterOption, centerOptions } from './center-options';

/** Shift and an arrow move five squares. */
const SHIFT_STEP = 5;

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
  private readonly pointer = new AreaPointer({
    readOnly: () => this.readOnly(),
    pickUnder: (event, lift) => this.pickAt(squareUnder(event, lift, this.columns(), this.rows())),
    hover: this.hover,
    setHover: (pick, byKey) => this.setHover(pick, byKey),
    tap: (pick) => this.tap(pick),
  });
  /** Where the finger is, in percent of the map, while it drags: the dotted line to the template above it. */
  protected readonly finger = this.pointer.finger;
  /** "Fora do alcance." and why, after a tap outside the range. */
  protected readonly refusal = signal('');
  protected readonly listOpen = signal(false);
  protected readonly active = signal(0);
  private readonly announcer = new AreaAnnouncer();
  /** What the live region says, a pause after the last move. */
  protected readonly spoken = this.announcer.spoken;

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
  protected readonly edgeMark = computed(() =>
    edgeMarkOf(this.shown(), this.caster(), this.columns(), this.rows()),
  );
  /** The dashed line from the caster to the point, with its length ("7,5 m"). */
  protected readonly line = computed(() =>
    areaLine(this.caster(), this.origin(), this.columns(), this.rows()),
  );
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
  protected readonly options = computed<readonly CenterOption[]>(() =>
    centerOptions(this.combatants(), this.seen(), this.casterId(), this.isPoint()),
  );

  constructor() {
    afterNextRender(() => {
      if (this.autofocus()) {
        this.app()?.nativeElement.focus();
      }
    });
    inject(DestroyRef).onDestroy(() => this.announcer.clear());
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
      this.announcer.now(`Fora do alcance. ${text}`);
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

  protected onPointerDown(event: PointerEvent): void {
    this.pointer.down(event);
  }

  protected onPointerMove(event: PointerEvent): void {
    this.pointer.move(event);
  }

  protected onPointerUp(event: PointerEvent): void {
    this.pointer.up(event);
  }

  protected onPointerLeave(event: PointerEvent): void {
    this.pointer.leave(event);
  }

  protected onPointerCancel(): void {
    this.pointer.cancel();
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
    const from = this.hover() ?? this.placed();
    const next = this.isPoint()
      ? movedPoint(
          from,
          this.caster(),
          step,
          event.shiftKey ? SHIFT_STEP : 1,
          this.columns(),
          this.rows(),
        )
      : turnedDirection(from, step.dc + step.dr);
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

  /** "Ajustar o ponto, um quadrado por toque": moves the placed point one square, as a tap there. */
  protected nudge(dc: number, dr: number): void {
    const pick = this.placed();
    if (pick?.kind !== 'point') {
      return;
    }
    const square = clampSquare(
      { col: pick.square.col + dc, row: pick.square.row + dr },
      this.columns(),
      this.rows(),
    );
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
    const move = listMove(event.key, this.active(), this.options().length);
    if (move === null) {
      return;
    }
    event.preventDefault();
    if (move === 'choose') {
      this.choose(this.options()[this.active()]);
    } else if (move === 'close') {
      event.stopPropagation();
      this.closeList(true);
    } else {
      this.active.set(move);
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
    this.announcer.say(
      announcement(
        this.placement(),
        this.distanceOf(pick),
        pick.kind === 'direction' ? pick.direction : null,
        seen,
      ),
    );
  }

  // ---- drawing ----

  protected pct(n: number, of: number): number {
    return (n / of) * 100;
  }
}
