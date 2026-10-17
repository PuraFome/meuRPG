import type { WritableSignal } from '@angular/core';

import type { MapPoint, MapPointKind } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { newKey } from '../../../core/connect/idempotency';
import { editorErrorMessage } from '../../../core/maps/map-errors';
import type { MapState } from '../../../core/maps/map-state';
import type { MapsClient, PointChanges } from '../../../core/maps/maps-client';
import type { EditorSelection } from './editor-selection';
import { type PointPresets, newPointBody } from './new-point';

/** What the point panels need from the editor. */
export interface EditorPointSavesHost {
  readonly api: Pick<MapsClient, 'updatePoint' | 'deletePoint' | 'createPoint'>;
  state(): MapState;
  campaignId(): string;
  mapId(): string;
  selectedPoint(): MapPoint | null;
  /** The open point panel's `changes()`; `undefined` when there is none. */
  panelChanges(): PointChanges | null | undefined;
  /** Whether a move of this point is still on its way. */
  movePending(pointId: string): boolean;
  /** The kind waiting for a click on the map. */
  readonly placing: WritableSignal<MapPointKind | null>;
  readonly saving: WritableSignal<boolean>;
  readonly message: WritableSignal<string>;
}

/** "Salvar ponto" and "Apagar" of the point panels, and the lists a point saves on their own. */
export class EditorPointSaves {
  constructor(
    private readonly host: EditorPointSavesHost,
    /** What is chosen, and the question that guards it. */
    private readonly sel: EditorSelection,
  ) {}

  /** Saves the chosen point; `true` when it is saved, or there is nothing to save. */
  async save(): Promise<boolean> {
    const { host } = this;
    const point = host.selectedPoint();
    const mapId = host.mapId();
    const changes = host.panelChanges();
    if (!point || !mapId || changes === undefined) {
      return false;
    }
    if (changes === null) {
      // Nothing changed, or the draft breaks a rule (the fields say which).
      return !this.sel.dirty();
    }
    host.saving.set(true);
    this.sel.setPanelError(null);
    try {
      const saved = await host.api.updatePoint(host.campaignId(), mapId, point.id, changes);
      this.put(saved);
      host.message.set(`${saved.name} salvo.`);
      return true;
    } catch (err) {
      this.sel.setPanelError(editorErrorMessage(err, 'point', 'salvar o ponto'));
      return false;
    } finally {
      host.saving.set(false);
    }
  }

  async remove(): Promise<void> {
    const { host } = this;
    const point = host.selectedPoint();
    const mapId = host.mapId();
    if (!point || !mapId) {
      return;
    }
    try {
      await host.api.deletePoint(host.campaignId(), mapId, point.id);
      host.state().removePoint(point.id);
      this.sel.removed();
      host.message.set(`${point.name} foi apagado.`);
    } catch (err) {
      this.sel.setPanelError(editorErrorMessage(err, 'point', 'apagar o ponto'));
    }
  }

  /** Makes a new point of `kind` where the master clicked and chooses it, unless the chosen one has unsaved changes. */
  async create(
    kind: MapPointKind,
    at: { xBp: number; yBp: number },
    presets: PointPresets,
  ): Promise<void> {
    const { host } = this;
    const mapId = host.mapId();
    if (!mapId) {
      return;
    }
    if (this.sel.dirtyBlocks()) {
      host.message.set('Salve ou descarte as mudanças do ponto escolhido antes de pôr outro.');
      return;
    }
    host.placing.set(null);
    try {
      // One key per tap on the map: the point is made once, whatever the network does with the answer.
      const body = await newPointBody(kind, at, host.campaignId(), presets);
      const point = await host.api.createPoint(host.campaignId(), mapId, body, newKey());
      host.state().upsertPoint(point);
      this.sel.created(point.id);
      host.message.set('Ponto criado, escondido dos jogadores. Dê um nome a ele.');
    } catch (err) {
      host.message.set(editorErrorMessage(err, 'pointNew', 'criar o ponto'));
    }
  }

  /** The point as the map has it now (else the one given) with `patch` on it: the lists and the DC saved on their own. */
  patch(point: MapPoint, patch: Partial<MapPoint>): void {
    const now =
      this.host
        .state()
        .points()
        .find((p) => p.id === point.id) ?? point;
    this.host.state().upsertPoint({ ...now, ...patch });
  }

  /** A treasure marked or unmarked found (saved at once): the map carries the point the server answered. */
  put(point: MapPoint): void {
    const state = this.host.state();
    // A move of this point still on its way: the answer was computed before it, so the screen keeps the place the master dragged to.
    const shown = state.points().find((p) => p.id === point.id);
    const ahead = shown && this.host.movePending(point.id);
    state.upsertPoint(ahead ? { ...point, xBp: shown.xBp, yBp: shown.yBp } : point);
  }
}
