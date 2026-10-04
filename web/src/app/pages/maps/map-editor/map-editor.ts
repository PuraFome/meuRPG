import {
  Component,
  afterNextRender,
  computed,
  inject,
  input,
  signal,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';

import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { MapPointKind } from '../../../../gen/meurpg/maps/v1/maps_pb';
import type { MapPoint, SceneAction, SceneClue } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { MapsClient } from '../../../core/maps/maps-client';
import { mapErrorMessage } from '../../../core/maps/map-errors';
import { MapState } from '../../../core/maps/map-state';
import { MoveSaves } from '../../../core/maps/move-saves';
import { RosterClient, RosterEntry } from '../../../core/maps/roster-client';
import type { CluePlayer } from '../../../core/maps/scene-clues';
import { MapLegend } from '../../../shared/map-view/map-legend/map-legend';
import { scaleLabel } from '../../../shared/map-view/map-geometry';
import { pointKindIcon, pointKindLabel } from '../../../shared/map-view/map-labels';
import { MapMove, MapSelection, MapView } from '../../../shared/map-view/map-view';
import { PointPanel } from '../point-panel/point-panel';

const KINDS = [MapPointKind.BATTLE, MapPointKind.SUBMAP, MapPointKind.SCENE] as const;

/** The name a point is born with: the master renames it right away. */
function defaultName(kind: MapPointKind): string {
  switch (kind) {
    case MapPointKind.BATTLE:
      return 'Nova batalha';
    case MapPointKind.SUBMAP:
      return 'Novo submapa';
    default:
      return 'Nova cena';
  }
}

/**
 * The master's map editor on a computer (E5-23, MR-008): a toolbar, the map
 * and a side panel.
 *
 * - "Adicionar ponto": pick a kind, then click the map. The point appears
 *   there, selected and hidden, and the panel focuses "Nome".
 * - Drag a point or a token, or use the arrow keys on the selected one
 *   (0,5 %, Shift 5 %); the position saves on drop or key release. A failed
 *   save puts the item back and says so.
 * - "Adicionar token" lists the characters that aren't on this map yet; the
 *   token lands in the middle of what is on screen.
 * - The side panel edits the selected point and saves it with "Salvar
 *   ponto"; selecting another point with unsaved changes asks, in place,
 *   whether to save or discard.
 */
@Component({
  selector: 'app-map-editor',
  imports: [MapLegend, MapView, MatButtonModule, MatIconModule, MatMenuModule, PointPanel],
  templateUrl: './map-editor.html',
  styleUrl: './map-editor.scss',
})
export class MapEditor {
  private readonly api = inject(MapsClient);
  /** Saves each point's and token's moves one at a time (see `MoveSaves`). */
  private readonly moves = new MoveSaves();
  private readonly roster = inject(RosterClient);

  readonly campaignId = input.required<string>();
  readonly state = input.required<MapState>();
  /** The campaign's maps, for "Leva para". */
  readonly maps = input<readonly { id: string; name: string }[]>([]);

  protected readonly view = viewChild(MapView);
  private readonly panel = viewChild(PointPanel);

  protected readonly kinds = KINDS;
  protected readonly kindLabel = pointKindLabel;
  protected readonly kindIcon = pointKindIcon;
  protected readonly scaleLabel = scaleLabel;

  protected readonly placing = signal<MapPointKind | null>(null);
  protected readonly selection = signal<MapSelection | null>(null);
  protected readonly pending = signal<MapSelection | null>(null);
  protected readonly dirty = signal(false);
  protected readonly saving = signal(false);
  protected readonly panelError = signal<string | null>(null);
  protected readonly message = signal('');
  protected readonly justCreated = signal(false);
  protected readonly everyone = signal<readonly RosterEntry[]>([]);

  protected readonly image = computed(() => {
    const image = this.state().map()?.image;
    return image ? { url: image.url, width: image.width, height: image.height } : null;
  });
  protected readonly selectedPoint = computed(() => {
    const s = this.selection();
    return s?.kind === 'point' ? (this.state().points().find((p) => p.id === s.id) ?? null) : null;
  });
  protected readonly selectedToken = computed(() => {
    const s = this.selection();
    return s?.kind === 'token'
      ? (this.state().tokens().find((t) => t.characterId === s.id) ?? null)
      : null;
  });
  protected readonly pendingName = computed(() => {
    const s = this.selection();
    return this.state().points().find((p) => p.id === s?.id)?.name ?? '';
  });
  /** The player characters, to say who has each clue ("Todos", "Só Brisa"). */
  protected readonly players = computed<readonly CluePlayer[]>(() =>
    this.everyone()
      .filter((c) => c.kind === CharacterKind.PLAYER && c.playerUserId !== '')
      .map((c) => ({ id: c.id, name: c.name, playerName: c.playerName ?? '' })),
  );
  protected readonly available = computed(() => {
    const onMap = new Set(this.state().tokens().map((t) => t.characterId));
    return this.everyone().filter((c) => !onMap.has(c.id));
  });
  protected readonly hint = computed(() =>
    this.placing() === null
      ? 'Arraste para mover. As setas movem o item escolhido.'
      : 'Clique no mapa para pôr o ponto.',
  );

  constructor() {
    afterNextRender(() => {
      this.roster.list(this.campaignId()).then(
        (list) => this.everyone.set(list),
        () => undefined,
      );
    });
  }

  // ---- toolbar ----

  protected toggleTool(kind: MapPointKind): void {
    this.placing.update((current) => (current === kind ? null : kind));
  }

  protected async addToken(entry: RosterEntry): Promise<void> {
    const mapId = this.state().map()?.id;
    if (!mapId) {
      return;
    }
    const at = this.view()?.centerBp() ?? { xBp: 5000, yBp: 5000 };
    try {
      const token = await this.api.placeToken(this.campaignId(), mapId, entry.id, at.xBp, at.yBp);
      this.state().upsertToken(token);
      this.selection.set({ kind: 'token', id: token.characterId });
      this.message.set(`${entry.name} está no mapa.`);
    } catch (err) {
      this.message.set(mapErrorMessage(err, 'pôr o token'));
    }
  }

  // ---- the map ----

  protected onEmptyClick(at: { xBp: number; yBp: number }): void {
    const kind = this.placing();
    if (kind === null) {
      this.requestSelect(null);
      return;
    }
    void this.createPoint(kind, at);
  }

  private async createPoint(kind: MapPointKind, at: { xBp: number; yBp: number }): Promise<void> {
    const mapId = this.state().map()?.id;
    if (!mapId) {
      return;
    }
    if (this.dirtyBlocks()) {
      this.message.set('Salve ou descarte as mudanças do ponto escolhido antes de pôr outro.');
      return;
    }
    this.placing.set(null);
    try {
      const point = await this.api.createPoint(this.campaignId(), mapId, {
        kind,
        name: defaultName(kind),
        description: '',
        xBp: at.xBp,
        yBp: at.yBp,
      });
      this.state().upsertPoint(point);
      this.panelError.set(null);
      this.justCreated.set(true);
      this.selection.set({ kind: 'point', id: point.id });
      this.message.set(`Ponto criado, escondido dos jogadores. Dê um nome a ele.`);
    } catch (err) {
      this.message.set(mapErrorMessage(err, 'criar o ponto'));
    }
  }

  /** Unsaved changes: the page asks before moving on. */
  private dirtyBlocks(): boolean {
    return this.dirty() && this.selection()?.kind === 'point';
  }

  protected requestSelect(next: MapSelection | null): void {
    const current = this.selection();
    if (current?.kind === next?.kind && current?.id === next?.id) {
      return;
    }
    if (this.dirtyBlocks()) {
      this.pending.set(next ?? { kind: 'point', id: '' });
      return;
    }
    this.moveSelection(next);
  }

  private moveSelection(next: MapSelection | null): void {
    this.pending.set(null);
    this.justCreated.set(false);
    this.panelError.set(null);
    this.selection.set(next && next.id !== '' ? next : null);
  }

  protected async saveAndContinue(): Promise<void> {
    const next = this.pending();
    if (await this.save()) {
      this.moveSelection(next);
    }
  }

  protected discardAndContinue(): void {
    const next = this.pending();
    this.panel()?.discard();
    this.moveSelection(next);
  }

  protected async onMoved(move: MapMove): Promise<void> {
    const state = this.state();
    const mapId = state.map()?.id;
    if (!mapId) {
      return;
    }
    if (move.kind === 'point') {
      const before = state.points().find((p) => p.id === move.id);
      if (!before) {
        return;
      }
      state.upsertPoint({ ...before, xBp: move.xBp, yBp: move.yBp });
      await this.moves.move(
        `${mapId}/point/${move.id}`,
        before,
        { xBp: move.xBp, yBp: move.yBp },
        {
          save: (to) =>
            this.api.updatePoint(this.campaignId(), mapId, move.id, { xBp: to.xBp, yBp: to.yBp }),
          failed: (saved, err) => {
            const now = state.points().find((p) => p.id === move.id);
            if (now) {
              state.upsertPoint({ ...now, xBp: saved.xBp, yBp: saved.yBp });
            }
            this.message.set(mapErrorMessage(err, 'mover o ponto'));
          },
        },
      );
      return;
    }
    const before = state.tokens().find((t) => t.characterId === move.id);
    if (!before) {
      return;
    }
    state.upsertToken({ ...before, xBp: move.xBp, yBp: move.yBp });
    await this.moves.move(
      `${mapId}/token/${move.id}`,
      before,
      { xBp: move.xBp, yBp: move.yBp },
      {
        save: (to) => this.api.placeToken(this.campaignId(), mapId, move.id, to.xBp, to.yBp),
        failed: (saved, err) => {
          const now = state.tokens().find((t) => t.characterId === move.id);
          if (now) {
            state.upsertToken({ ...now, xBp: saved.xBp, yBp: saved.yBp });
          }
          this.message.set(mapErrorMessage(err, 'mover o token'));
        },
      },
    );
  }

  // ---- the point panel ----

  protected async save(): Promise<boolean> {
    const point = this.selectedPoint();
    const mapId = this.state().map()?.id;
    const changes = this.panel()?.changes();
    if (!point || !mapId) {
      return false;
    }
    if (changes === undefined) {
      return false;
    }
    if (changes === null) {
      // Nothing changed, or the draft breaks a rule (the fields say which).
      return !this.dirty();
    }
    this.saving.set(true);
    this.panelError.set(null);
    try {
      const saved = await this.api.updatePoint(this.campaignId(), mapId, point.id, changes);
      this.state().upsertPoint(saved);
      this.message.set(`${saved.name} salvo.`);
      return true;
    } catch (err) {
      this.panelError.set(mapErrorMessage(err, 'salvar o ponto'));
      return false;
    } finally {
      this.saving.set(false);
    }
  }

  /** The scene actions saved on their own: the point carries the new list. */
  protected setSceneActions(point: MapPoint, actions: readonly SceneAction[]): void {
    const now = this.state().points().find((p) => p.id === point.id) ?? point;
    this.state().upsertPoint({ ...now, sceneActions: [...actions] });
  }

  /** The clues saved on their own: the point carries the new list. */
  protected setClues(point: MapPoint, clues: readonly SceneClue[]): void {
    const now = this.state().points().find((p) => p.id === point.id) ?? point;
    this.state().upsertPoint({ ...now, clues: [...clues] });
  }

  protected async remove(): Promise<void> {
    const point = this.selectedPoint();
    const mapId = this.state().map()?.id;
    if (!point || !mapId) {
      return;
    }
    try {
      await this.api.deletePoint(this.campaignId(), mapId, point.id);
      this.state().removePoint(point.id);
      this.dirty.set(false);
      this.moveSelection(null);
      this.message.set(`${point.name} foi apagado.`);
    } catch (err) {
      this.panelError.set(mapErrorMessage(err, 'apagar o ponto'));
    }
  }

  // ---- the token panel ----

  protected async toggleTokenHidden(): Promise<void> {
    const token = this.selectedToken();
    const mapId = this.state().map()?.id;
    if (!token || !mapId) {
      return;
    }
    try {
      const saved = await this.api.setTokenHidden(
        this.campaignId(),
        mapId,
        token.characterId,
        !token.hidden,
      );
      this.state().upsertToken(saved);
    } catch (err) {
      this.message.set(mapErrorMessage(err, 'mudar o token'));
    }
  }

  protected async removeToken(): Promise<void> {
    const token = this.selectedToken();
    const mapId = this.state().map()?.id;
    if (!token || !mapId) {
      return;
    }
    try {
      await this.api.removeToken(this.campaignId(), mapId, token.characterId);
      this.state().removeToken(token.characterId);
      this.selection.set(null);
      this.message.set(`${token.name} saiu do mapa.`);
    } catch (err) {
      this.message.set(mapErrorMessage(err, 'tirar o token'));
    }
  }
}
