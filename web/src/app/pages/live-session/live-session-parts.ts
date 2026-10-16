import { CombatState } from '../../core/combat/combat-state';
import { FogView } from '../../core/maps/fog-view';
import { MapState } from '../../core/maps/map-state';
import { NotesState } from '../../core/notes/notes-state';
import { SceneState } from '../../core/play/scene-state';
import { PuzzleSessionState } from '../../core/puzzles/puzzle-session';
import { ToastQueue } from '../../core/traps/toast-queue';
import { TrapBoard } from '../../core/traps/trap-board';
import { TreasureWatch } from '../../core/traps/treasure-text';
import type { LiveSessionServices } from './live-session-services';
import type { LiveSessionState } from './live-session-state';

/** The state holders the page builds around its services: each one is a plain class with its own tests. */
export class LiveSessionParts {
  readonly mapState: MapState;
  /** What traps did and the damage that waits (MR-035): the master's panel and the players' "Registro". */
  readonly trapBoard: TrapBoard;
  /** The player's toasts: a trap noticed, a treasure found (8 s each). */
  readonly toasts = new ToastQueue();
  readonly treasureWatch = new TreasureWatch();
  /** What the viewer sees of the current map when it has the fog of war on (MR-036): the vision with its tiles and
   * the layers, read again on `vision_changed` and `map_changed`. The combat map draws the same one. */
  readonly fog: FogView;
  /** The session's combat (MR-013): read on every `ready` and after each
   * `encounter_changed`; `turn_changed` and `combatant_moved` apply in place. */
  readonly combat = new CombatState();
  /** The RP scene open in the session (MR-015): read on every `ready` and
   * after each `scene_changed` or `scene_check_rolled`. */
  readonly scene: SceneState;
  /** The puzzles of the session (MR-038, E10-06): the master's menu with each one's live state, or what the players are shown.
   * `puzzle_changed` reads one again; `ready` reads the list. */
  readonly puzzles: PuzzleSessionState;
  /** The player's notes and the clues the master revealed (MR-030): read after
   * each `ready` and each `notes_changed`. The master has none. */
  readonly notes: NotesState;

  constructor(services: LiveSessionServices, state: LiveSessionState) {
    const { mapsApi } = services;
    this.mapState = new MapState((mapId) => mapsApi.get(state.campaignId(), mapId));
    this.trapBoard = new TrapBoard(
      services.trapsApi,
      mapsApi,
      () => state.campaignId(),
      () => state.currentMapId(),
      () => state.isMaster(),
    );
    this.fog = new FogView(
      (mapId, as) => mapsApi.vision(state.campaignId(), mapId, as ?? ''),
      (mapId, as) => mapsApi.layers(state.campaignId(), mapId, as ?? ''),
      () => !state.isMaster(),
    );
    this.scene = new SceneState(
      () => services.sceneApi.get(state.campaignId()),
      () => state.isMaster(),
    );
    this.puzzles = new PuzzleSessionState(
      services.puzzlesApi,
      () => state.campaignId(),
      () => state.isMaster(),
    );
    this.notes = new NotesState(services.notesApi, () => state.campaignId());
  }
}
