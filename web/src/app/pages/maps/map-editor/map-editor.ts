import {
  ChangeDetectionStrategy,
  Component,
  HostListener,
  ElementRef,
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
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { MapPointKind } from '../../../../gen/meurpg/maps/v1/maps_pb';
import type {
  Map as MapMessage,
  MapPoint,
  SceneAction,
  SceneClue,
  SceneImage,
} from '../../../../gen/meurpg/maps/v1/maps_pb';
import { TrapPresets } from '../../../core/traps/trap-presets';
import { focusWithRing } from '../../../core/creatures/focus-ring';
import { DarkWatch } from '../../../core/maps/dark-watch';
import { LightPresets } from '../../../core/maps/light-presets';
import { MapState } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
import { paintHint } from '../../../core/maps/paint-tools';
import { mapTokenInitial } from '../../../core/maps/token-initial';
import { RosterClient, RosterEntry } from '../../../core/maps/roster-client';

import { ViewAsList, type ViewAsPerson } from '../../../shared/fog-map/view-as-list';
import { DarkMarks } from '../../../shared/fog-map/dark-marks';
import { ViewAsMapView } from '../../../shared/fog-map/view-as-map';
import { MapAsk } from '../../../shared/map-ask/map-ask';
import { MapLayersLegend } from '../../../shared/map-layers/map-layers-legend';
import { MapPinsLegend } from '../../../shared/map-pins/map-pins-legend';
import { MapLegend } from '../../../shared/map-view/map-legend/map-legend';
import { MapSelection, MapView } from '../../../shared/map-view/map-view';
import { CoverDegrees } from '../cover-degrees/cover-degrees';
import { EditorBar, type EditorMode } from '../editor-bar/editor-bar';
import { EditorOverlay } from '../../../shared/editor-overlay/editor-overlay';
import { EditorDark } from './editor-dark';
import { FogPanel } from '../fog-panel/fog-panel';
import { GridPanel } from '../grid-panel/grid-panel';
import { LayersPanel } from '../layers-panel/layers-panel';
import { PaintSurface } from '../paint-surface/paint-surface';
import { DoorPanel } from '../door-panel/door-panel';
import { DungeonImage } from '../dungeon-image/dungeon-image';
import { DungeonRooms } from '../dungeon-rooms/dungeon-rooms';
import { EditorDungeon } from './editor-dungeon';
import { EditorItems } from './editor-items';
import { EditorSelection } from './editor-selection';
import { EditorPointSaves } from './editor-point-saves';
import { EditorPainting } from './editor-painting';
import {
  cluePlayers,
  finderRows,
  loadRoster,
  notOnMap,
  occupantsOf,
  selectedPointOf,
  selectedTokenOf,
} from './editor-people';
import { EditorViewAs } from './editor-view-as';
import { LightPointPanel } from '../point-kinds/light-point-panel';
import { TrapPointPanel } from '../point-kinds/trap-point-panel';
import { TreasurePointPanel } from '../point-kinds/treasure-point-panel';
import { PointList } from '../point-list/point-list';
import { PointPanel } from '../point-panel/point-panel';
import type { PointChanges } from '../../../core/maps/maps-client';
import { factorLabel } from '../../../core/maps/calibration';

/** What each point panel offers the page: the shape `PointPanel` always had. */
interface PointPanelApi {
  changes(): PointChanges | null | undefined;
  discard(): void;
}

/**
 * The master's map editor on a computer (E5-23, E9-01, E9-02, MR-008, MR-034, MR-035, MR-036, MR-041): a bar, the map and a side
 * column, in two modes.
 *
 * - **Pontos** (the editor it always was): "Adicionar ponto" (Batalha, Submapa, Cena de RP, and now Luz, Armadilha and Tesouro),
 *   drag a point or a token, the arrows move the selected one, and the side panel edits it with "Salvar ponto" (a Luz, an
 *   Armadilha and a Tesouro have their own panels). With nothing selected the side column lists the points, and, when the fog
 *   is on, "Ver como": the map as one player gets it, closed by "Voltar à sua vista".
 * - **Pintar:** Terreno difícil, Parede, Cobertura (Meia, Três quartos), Luz (Claro, Penumbra, Escuro), "Apagar" and the brush
 *   (1×1, 3×3). A drag paints square by square, a click one, Shift erases; the arrows move the brush, Espaço paints, Esc leaves.
 *   Strokes show at once and go to the server in batches (`PaintQueue`), "Tudo salvo" says they arrived. Beside the map, "Camadas"
 *   and "Grade"; under it, "Névoa de guerra". Painting is allowed while a combat runs on the map; the grid and the image are not.
 *
 * Without a grid there is nothing to paint or to light: the paint tools say so and cannot act. The map's image says it is
 * visible to the players ("O que está desenhado na imagem, os jogadores veem."), on the page's header.
 */
@Component({
  selector: 'app-map-editor',
  imports: [
    CoverDegrees,
    DoorPanel,
    DungeonImage,
    DungeonRooms,
    EditorBar,
    EditorOverlay,
    FogPanel,
    GridPanel,
    LayersPanel,
    LightPointPanel,
    MapAsk,
    MapLayersLegend,
    MapLegend,
    MapPinsLegend,
    DarkMarks,
    EditorDark,
    MapView,
    MatButtonModule,
    MatIconModule,
    PaintSurface,
    PointList,
    PointPanel,
    TrapPointPanel,
    TreasurePointPanel,
    ViewAsList,
    ViewAsMapView,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './map-editor.html',
  styleUrl: './map-editor.scss',
})
export class MapEditor {
  private readonly api = inject(MapsClient);
  /** What every controller below reads of the page. */
  private readonly ctx = {
    api: this.api,
    state: () => this.state(),
    campaignId: () => this.campaignId(),
    mapId: () => this.mapId(),
  };
  private readonly roster = inject(RosterClient);
  protected readonly dungeonCtl = new EditorDungeon({
    campaignId: () => this.campaignId(),
    map: () => this.map(),
    view: () => this.view(),
  });
  /** What the server says of a generated dungeon: its rooms list and whether the image is still the generator's (the master only; `null` on any other map). */
  protected readonly dungeon = this.dungeonCtl.info;
  /** The room outlined on the map (chosen in the rooms list). */
  protected readonly roomOutline = this.dungeonCtl.roomOutline;
  private readonly lightPresets = inject(LightPresets);
  private readonly trapPresets = inject(TrapPresets);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly campaignId = input.required<string>();
  readonly state = input.required<MapState>();
  /** The campaign's maps, for "Leva para". */
  readonly maps = input<readonly { id: string; name: string }[]>([]);
  /** A combat that has not ended runs on this map: the grid and the image cannot change, painting can. */
  readonly combatRunning = input(false);
  /** The open session's number, when there is one: a treasure marked found counts in its summary. */
  readonly sessionNumber = input<number | null>(null);

  /** Whether a new grid or a new image would erase something (what is painted, or the fog's memory): the header asks first. */
  readonly erasesChange = output<boolean>();
  /** The page's flags may be out of date (a refusal said a combat runs, or does not): read them again. */
  readonly staleFlags = output<void>();
  /** A map was made from a Submapa point: the page reads the campaign's maps again. */
  readonly mapsChanged = output<void>();

  protected readonly map = computed(() => this.state().map());
  /** What a square of the drawing is worth, for the legend of a calibrated map: "3 m". */
  protected readonly factorText = computed(() => factorLabel(this.map()?.squareFactor || 1));
  /** "Ir para a grade": the map has no grid, so the paint tools wait; the field that sets it is in the side panel, far from the bar. */
  protected goToGrid(): void {
    const field = this.host.nativeElement.querySelector<HTMLElement>('app-grid-panel input');
    field?.scrollIntoView({ block: 'center' });
    focusWithRing(field);
  }

  protected readonly view = viewChild(MapView);
  /** Who sees nothing where they stand on this map (the master's light warning, B3): read again with the map and its tokens. */
  protected readonly dark = new DarkWatch(
    (mapId) => this.api.charactersInTheDark(this.campaignId(), mapId),
    () => {
      const map = this.map();
      return map?.fogEnabled ? map.id : null;
    },
    () => this.state().tokens(),
  );
  /** The character "Ver no mapa" chose: the one label "<Nome> · no escuro". */
  protected readonly darkChosen = signal<string | null>(null);

  /** "Ver no mapa" of the light warning: the map centers on the character. */
  protected showDark(characterId: string): void {
    this.darkChosen.set(characterId);
    const t = this.state()
      .tokens()
      .find((x) => !x.creatureId && x.characterId === characterId);
    if (t) {
      // Centers on the character at the zoom the map has (the whole map, unless the master zoomed).
      this.view()?.focusOn({ xBp: t.xBp, yBp: t.yBp }, this.view()?.scale());
    }
  }
  private readonly pointPanel = viewChild(PointPanel);
  private readonly lightPanel = viewChild(LightPointPanel);
  private readonly trapPanel = viewChild(TrapPointPanel);
  private readonly treasurePanel = viewChild(TreasurePointPanel);

  // ---- the point editor ----
  protected readonly mode = signal<EditorMode>('points');
  protected readonly placing = signal<MapPointKind | null>(null);
  protected readonly sel = new EditorSelection({
    applyMode: (mode) => this.applyMode(mode),
    discardPanel: () => this.activePanel()?.discard(),
    save: (): Promise<boolean> => this.pointSaves.save(),
  });
  protected readonly selection = this.sel.selection;
  protected readonly pending = this.sel.pending;
  /** The mode the master asked for while the point on the panel has unsaved changes: the same question as for another point. */
  protected readonly pendingMode = this.sel.pendingMode;
  protected readonly dirty = this.sel.dirty;
  protected readonly saving = signal(false);
  protected readonly panelError = this.sel.panelError;
  protected readonly message = signal('');
  protected readonly justCreated = this.sel.justCreated;
  protected readonly everyone = signal<readonly RosterEntry[]>([]);
  /** The roster came back: before that (or if it failed), "nobody plays yet" would be a guess. */
  protected readonly rosterLoaded = signal(false);
  protected readonly lightReach = this.sel.lightReach;
  protected readonly lightNames = signal<ReadonlyMap<string, string>>(new Map());

  /** Moves of points and tokens (saved one at a time, see `MoveSaves`) and the token calls. */
  protected readonly items = new EditorItems({
    ...this.ctx,
    selectedToken: () => this.selectedToken(),
    centerBp: () => this.view()?.centerBp(),
    layers: () => this.paint.painted.layers(),
    selection: this.selection,
    message: this.message,
  });
  protected readonly pointSaves = new EditorPointSaves(
    {
      ...this.ctx,
      selectedPoint: () => this.selectedPoint(),
      panelChanges: () => this.activePanel()?.changes(),
      movePending: (id) => this.items.pendingPoint(id),
      placing: this.placing,
      saving: this.saving,
      message: this.message,
    },
    this.sel,
  );

  // ---- painting ----
  /** The tool, the layers, the queue that saves the strokes and the question on leaving (`EditorPainting`). */
  protected readonly paint = new EditorPainting(
    this.campaignId,
    this.map,
    this.mode,
    () => this.staleFlags.emit(),
    computed(() => this.occupants()),
  );

  /** Who stands on which square (the tokens), for the question before a door goes over them. */
  private readonly occupants = computed(() => occupantsOf(this.map(), this.state().tokens()));

  // ---- "Ver como" ----
  protected readonly viewAsCtl = new EditorViewAs({
    ...this.ctx,
    map: () => this.map(),
    people: () => this.viewAsPeople(),
  });
  protected readonly viewAs = this.viewAsCtl.viewAs;
  protected readonly viewNote = this.viewAsCtl.viewNote;
  protected readonly viewGone = this.viewAsCtl.viewGone;
  protected readonly counts = this.viewAsCtl.counts;

  protected readonly mapId = computed(() => this.map()?.id ?? '');
  protected readonly image = computed(() => {
    const image = this.state().map()?.image;
    return image ? { url: image.url, width: image.width, height: image.height } : null;
  });
  protected readonly selectedPoint = computed(() =>
    selectedPointOf(this.state().points(), this.selection()),
  );
  protected readonly selectedToken = computed(() =>
    selectedTokenOf(this.state().tokens(), this.selection()),
  );
  protected readonly pendingName = computed(
    () =>
      this.state()
        .points()
        .find((p) => p.id === this.selection()?.id)?.name ?? '',
  );
  /** The player characters, to say who has each clue ("Todos", "Só Brisa"). */
  protected readonly players = computed(() => cluePlayers(this.everyone()));
  /** Who found a treasure: the living player characters, with their class and player. */
  protected readonly finders = computed(() => finderRows(this.everyone()));
  protected readonly viewAsPeople = computed<readonly ViewAsPerson[]>(() =>
    this.players().map((p) => ({ id: p.id, name: p.name, sub: p.playerName })),
  );
  protected readonly viewAsPerson = computed(
    () => this.viewAsPeople().find((p) => p.id === this.viewAs()) ?? null,
  );
  protected readonly available = computed(() => notOnMap(this.everyone(), this.state().tokens()));
  protected readonly hint = computed(() => {
    if (this.mode() === 'paint') {
      return this.paint.canPaint() ? paintHint(this.paint.settings()) : null;
    }
    return this.placing() === null
      ? 'Arraste para mover. As setas movem o item escolhido.'
      : 'Clique no mapa para pôr o ponto.';
  });
  protected readonly Trap = MapPointKind.TRAP;
  protected readonly Treasure = MapPointKind.TREASURE;
  protected readonly Light = MapPointKind.LIGHT;

  protected readonly initialOf = mapTokenInitial;

  constructor() {
    afterNextRender(() =>
      loadRoster(this.roster, this.lightPresets, this.campaignId(), {
        everyone: this.everyone,
        rosterLoaded: this.rosterLoaded,
        lightNames: this.lightNames,
      }),
    );

    // The light's rings go with the selection of a light.
    effect(() => {
      if (this.selectedPoint()?.kind !== MapPointKind.LIGHT) {
        untracked(() => this.lightReach.set(null));
      }
    });
    effect(() => this.erasesChange.emit(this.paint.erases()));
  }

  // ---- the mode ----

  protected setMode(mode: EditorMode): void {
    if (mode === this.mode()) {
      return;
    }
    if (mode === 'paint' && this.sel.dirtyBlocks()) {
      this.pendingMode.set('paint');
      return;
    }
    this.applyMode(mode);
  }

  private applyMode(mode: EditorMode): void {
    this.pendingMode.set(null);
    if (mode === 'paint') {
      this.placing.set(null);
      this.selection.set(null);
      this.pending.set(null);
      this.viewAsCtl.set(null);
    } else {
      this.paint.stop();
    }
    this.mode.set(mode);
  }

  /** Esc leaves the paint surface: the focus goes back to the chosen tool. */
  protected leaveSurface(): void {
    this.host.nativeElement
      .querySelector<HTMLElement>('app-editor-bar [aria-pressed="true"]')
      ?.focus();
  }

  /** For the route guard: true when it is fine to leave (see `EditorPainting.confirmLeave`). */
  confirmLeave(): Promise<boolean> {
    return this.paint.confirmLeave();
  }

  /** The tab closes with strokes unsaved: the browser's own question. */
  @HostListener('window:beforeunload', ['$event'])
  protected onBeforeUnload(event: BeforeUnloadEvent): void {
    if (this.paint.queue.busy) {
      event.preventDefault();
    }
  }

  // ---- toolbar ----

  protected toggleKind(kind: MapPointKind): void {
    this.placing.update((current) => (current === kind ? null : kind));
  }

  // ---- the map ----

  protected onEmptyClick(at: { xBp: number; yBp: number }): void {
    const kind = this.placing();
    if (kind === null) {
      this.requestSelect(null);
      return;
    }
    void this.pointSaves.create(kind, at, { lights: this.lightPresets, traps: this.trapPresets });
  }

  protected requestSelect(next: MapSelection | null): void {
    this.sel.request(next);
  }

  /** Whichever point panel is open: they all answer `changes()` and `discard()`. */
  private activePanel(): PointPanelApi | undefined {
    return this.pointPanel() ?? this.lightPanel() ?? this.trapPanel() ?? this.treasurePanel();
  }

  // ---- the point panels ----

  /** The scene actions saved on their own: the point carries the new list. */
  protected setSceneActions(point: MapPoint, actions: readonly SceneAction[]): void {
    this.pointSaves.patch(point, { sceneActions: [...actions] });
  }

  /** "Mostrar a CD aos jogadores" saved on its own: the point carries it. */
  protected setShowDc(point: MapPoint, showDc: boolean): void {
    this.pointSaves.patch(point, { showDc });
  }

  /** The clues saved on their own: the point carries the new list. */
  protected setClues(point: MapPoint, clues: readonly SceneClue[]): void {
    this.pointSaves.patch(point, { clues: [...clues] });
  }

  /** The images saved on their own: the point carries the new list. */
  protected setImages(point: MapPoint, images: readonly SceneImage[]): void {
    this.pointSaves.patch(point, { images: [...images] });
  }

  // ---- the grid and the fog ----

  /** The map after a change of the grid or the fog: the page's state takes it, the layers read again by themselves. */
  protected onMapChanged(map: MapMessage): void {
    this.state().setMap(map);
  }

  /** "Redesenhar" sends the strokes that still wait first, so the new image has the walls just painted. */
  protected readonly beforeRedraw = async (): Promise<string | null> => {
    await this.paint.queue.flush();
    return this.paint.queue.status() === 'error'
      ? 'Há traços que o servidor ainda não recebeu. Espere o aviso “Tudo salvo” e tente de novo.'
      : null;
  };
}
