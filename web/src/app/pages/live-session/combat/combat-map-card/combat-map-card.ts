import { Component, computed, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { CombatantKind, type Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { PHONE_QUERY, mediaQuery } from '../../../../shared/map-view/media-query';
import { ownCombatant } from '../../../../core/combat/combat-view';
import { metersFixed } from '../../../../core/units';
import { type DoorSquare, type MapLayers, NO_LAYERS } from '../../../../core/maps/layers';
import {
  CombatMap,
  type CombatMapImage,
  type OfferMark,
  type Reach,
  type TokenDrop,
} from '../../../../shared/combat-map/combat-map';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';
import { AreaOverlay } from '../../../../shared/area-picker/area-overlay';
import { lastArea } from '../../../../core/combat/hidden-reveal';
import { MapLayersLegend } from '../../../../shared/map-layers/map-layers-legend';
import {
  type Vision,
  knownWindow,
  tileProgress,
  tileRects,
  visionLegend,
} from '../../../../core/maps/vision';

/**
 * The map panel of a running combat or of its setup (E6-04, E6-05, E6-11):
 * the battle map with its legend. The master drags any token ("O mestre anda
 * sem limite"); a player drags their own on their turn, which opens the "Mover" page on that square (the page asks, with its warnings; a drop never moves).
 * The legend names every shape the map uses, so no meaning is colour alone: the
 * layer marks the map has (`app-map-layers-legend`), then the tokens and, when
 * they are drawn, the movement marks and the square of an opportunity attack's
 * reactor. The master can turn the reach of whoever is on turn on and off
 * ("Mostrar o alcance do Toren no mapa", E9-05): the server's `GetMoveOptions`
 * answer, drawn as it comes. "Movimento forçado" (a teleport, a push or a pull)
 * is a box under it that makes the next drag skip the opportunity attacks.
 */
/** The squares kept around what the player knows when the card crops the map, and the share of the grid above which it does not. */
const CROP_MARGIN = 2;
const CROP_MAX_SHARE = 0.6;

@Component({
  selector: 'app-combat-map-card',
  imports: [
    AreaOverlay,
    CombatMap,
    CombatantToken,
    MapLayersLegend,
    MatButtonModule,
    MatIconModule,
  ],
  templateUrl: './combat-map-card.html',
  styleUrl: './combat-map-card.scss',
})
export class CombatMapCard {
  readonly encounter = input.required<Encounter>();
  readonly image = input.required<CombatMapImage>();
  readonly mapName = input('');
  readonly isMaster = input(false);
  /** The panel's title: "Mapa" for the master, the map's name for a player. */
  readonly title = input('Mapa');
  readonly reach = input<Reach | null>(null);
  /** The map's painted layers, once read. */
  readonly layers = input<MapLayers | null>(null);
  /** What the player sees of the map with the fog on (MR-036): the same drawing as the session's, with the combatants over it. */
  readonly fog = input<Vision | null>(null);
  /** The reactors of pending opportunity offers the caller answers or waits on (E9-13). */
  readonly offers = input<readonly OfferMark[]>([]);
  /** The master's switch for the reach of whoever is on turn: its name, and whether it is on. */
  readonly reachSwitch = input<{ readonly name: string; readonly on: boolean } | null>(null);
  /** The master's "Movimento forçado" box is offered (a running combat on a map): the next drag skips the opportunity attacks. */
  readonly forcedSwitch = input(false);
  /** The box is on: the next drag is forced. */
  readonly forced = input(false);
  /** What the last forced drag did, in words, for the live region. */
  readonly forcedNote = input('');
  /** The player's own combatant may be dragged to pick a square (their turn, desktop): the drop opens "Mover" there. */
  readonly ownMovable = input(false);
  readonly hint = input(true);
  /** The legend of the shapes; the setup's small map says it in a note. */
  readonly legend = input(true);
  /** A line under the map instead of the legend. */
  readonly note = input('');
  /** On a phone the master gets a zoomed, cropped preview centered on whoever
   * is on turn, and "Abrir mapa" for the whole map (E6-12). */
  readonly cropOnPhone = input(false);

  /** The master taps a door of the map (E10-05 10): the page opens the door's sheet. Not on the phone's cropped preview. */
  readonly doorPick = output<DoorSquare>();
  readonly doorTaps = input(false);
  readonly tokenDrop = output<TokenDrop>();
  /** The master turned the reach on or off. */
  readonly reachChange = output<boolean>();
  /** The master turned "Movimento forçado" on or off. */
  readonly forcedChange = output<boolean>();
  /** "Ver mapa": the player's full-screen, read-only map. */
  readonly openMap = output<void>();

  private readonly phone = mediaQuery(PHONE_QUERY);
  private readonly settled = signal<ReadonlySet<string>>(new Set());
  protected readonly fogProgress = computed(() => {
    const v = this.fog();
    return v ? tileProgress(tileRects(v), this.settled()) : { total: 0, done: 0 };
  });
  protected readonly fogLoading = signal(false);
  protected readonly fogLegend = computed(() => {
    const v = this.fog();
    return v ? visionLegend(v) : null;
  });
  protected onFogLoading(loading: boolean): void {
    this.fogLoading.set(loading);
  }

  protected onFogSettled(set: ReadonlySet<string>): void {
    this.settled.set(set);
  }
  /** With the fog on, the map card shows the part of the grid the player knows (two squares around it), so the tokens are
   * readable when the explored area is a small part of the grid; "Ver mapa" has the whole map. */
  protected readonly crop = computed(() => {
    const vision = this.fog();
    if (!vision || this.isMaster()) {
      return null;
    }
    const e = this.encounter();
    const known = knownWindow(
      vision,
      e.combatants.filter((c) => c.placed),
      CROP_MARGIN,
    );
    if (!known || known.cols * known.rows >= CROP_MAX_SHARE * vision.columns * vision.rows) {
      return null;
    }
    const { width, height } = this.image();
    // The window is as wide as it is tall at least (a tall strip of corridor would be a tall card), and never wider than the grid.
    const cols = Math.min(vision.columns, Math.max(known.cols, known.rows));
    const col = Math.min(
      Math.max(0, known.col - Math.floor((cols - known.cols) / 2)),
      vision.columns - cols,
    );
    const rows = known.rows;
    return {
      // The inner map is `columns / cols` times the card's width; the shift is a share of the inner map.
      width: (vision.columns / cols) * 100,
      shift: `translate(${(-col / vision.columns) * 100}%, ${(-known.row / vision.rows) * 100}%)`,
      ratio: `${(cols * width) / vision.columns} / ${(rows * height) / vision.rows}`,
    };
  });

  protected readonly preview = computed(
    () => this.cropOnPhone() && this.isMaster() && this.phone(),
  );

  /** Where the 2x map sits inside the preview frame, so that the one on turn
   * is in the middle (clamped, so no empty edge shows). The frame is 326 x 224
   * like the artboard: its height over the map's is `frameOverMap`. */
  protected readonly shift = computed(() => {
    const e = this.encounter();
    const focus =
      e.combatants.find((c) => c.id === e.currentCombatantId && c.placed) ??
      e.combatants.find((c) => c.placed);
    const fx = focus ? (focus.col + 0.5) / e.gridColumns : 0.5;
    const fy = focus ? (focus.row + 0.5) / e.gridRows : 0.5;
    const ratio = this.image().width / Math.max(1, this.image().height);
    const frameOverMap = ratio / 2 / (326 / 224);
    const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v));
    const x = clamp(-(fx - 0.25), -0.5, 0);
    const y = frameOverMap >= 1 ? 0 : clamp(-(fy - frameOverMap / 2), -(1 - frameOverMap), 0);
    return `translate(${x * 100}%, ${y * 100}%)`;
  });

  protected readonly own = computed(() => ownCombatant(this.encounter()));
  protected readonly size = computed(() => {
    const e = this.encounter();
    return `${e.gridColumns} × ${e.gridRows} quadrados de 1,5 m`;
  });
  /** "Área da última magia": the area a player's spell reached, while its question waits (the master's map only). */
  protected readonly lastArea = computed(() =>
    this.isMaster() ? lastArea(this.encounter()) : null,
  );
  protected readonly emptyLayers = NO_LAYERS;
  protected readonly reachMeters = computed(() => metersFixed((this.reach()?.leftDft ?? 0) / 10));
  protected readonly hasHidden = computed(() => this.encounter().combatants.some((c) => c.hidden));
  /** A player's creature has a square on the map: the legend names the dashed round token. */
  protected readonly hasCreatures = computed(() =>
    this.encounter().combatants.some((c) => c.kind === CombatantKind.CREATURE && c.placed),
  );
  protected readonly hasDefeated = computed(() =>
    this.encounter().combatants.some((c) => c.defeated),
  );
}
