import { signal } from '@angular/core';

import type { LightReach } from '../../../shared/editor-overlay/editor-overlay';
import type { MapSelection } from '../../../shared/map-view/map-view';
import type { EditorMode } from '../editor-bar/editor-bar';

/** What the question "save, discard or keep editing?" needs from the editor. */
export interface EditorSelectionHost {
  /** Goes to the mode the master asked for. */
  applyMode(mode: EditorMode): void;
  /** Throws away the open point panel's unsaved changes. */
  discardPanel(): void;
  /** Saves the open point; `true` when it is saved. */
  save(): Promise<boolean>;
}

/**
 * What is chosen on the map (a point or a token) and the question that guards it: while the point on the panel has unsaved changes,
 * choosing another point, or the "Pintar" mode, waits for "Salvar e continuar", "Descartar mudanças" or "Continuar editando".
 */
export class EditorSelection {
  readonly selection = signal<MapSelection | null>(null);
  /** What the master asked for while the point on the panel has unsaved changes. */
  readonly pending = signal<MapSelection | null>(null);
  /** The mode the master asked for in the same situation. */
  readonly pendingMode = signal<EditorMode | null>(null);
  readonly dirty = signal(false);
  private readonly panelErrorState = signal<string | null>(null);
  private readonly justCreatedState = signal(false);
  /** What the open point panel could not do ("Salvar ponto", "Apagar"). */
  readonly panelError = this.panelErrorState.asReadonly();
  /** The chosen point was just made: its panel puts the focus on the name. */
  readonly justCreated = this.justCreatedState.asReadonly();
  readonly lightReach = signal<LightReach | null>(null);

  constructor(private readonly host: EditorSelectionHost) {}

  /** Unsaved changes: the page asks before moving on. */
  dirtyBlocks(): boolean {
    return this.dirty() && this.selection()?.kind === 'point';
  }

  request(next: MapSelection | null): void {
    const current = this.selection();
    if (current?.kind === next?.kind && current?.id === next?.id) {
      return;
    }
    if (this.dirtyBlocks()) {
      this.pending.set(next ?? { kind: 'point', id: '' });
      return;
    }
    this.move(next);
  }

  /** Sets or clears the panel's error. */
  setPanelError(message: string | null): void {
    this.panelErrorState.set(message);
  }

  /** Chooses the point just made, and the panel asks for its name. */
  created(pointId: string): void {
    this.panelErrorState.set(null);
    this.justCreatedState.set(true);
    this.selection.set({ kind: 'point', id: pointId });
  }

  /** The chosen point is gone: nothing is chosen and nothing is left unsaved. */
  removed(): void {
    this.dirty.set(false);
    this.move(null);
  }

  move(next: MapSelection | null): void {
    this.pending.set(null);
    this.justCreatedState.set(false);
    this.panelErrorState.set(null);
    this.lightReach.set(null);
    this.selection.set(next && next.id !== '' ? next : null);
  }

  async saveAndContinue(): Promise<void> {
    if (await this.host.save()) {
      this.continue();
    }
  }

  discardAndContinue(): void {
    this.host.discardPanel();
    this.dirty.set(false);
    this.continue();
  }

  keepEditing(): void {
    this.pending.set(null);
    this.pendingMode.set(null);
  }

  /** What the master was going to do when the question came: another point, or the "Pintar" mode. */
  private continue(): void {
    const mode = this.pendingMode();
    const next = this.pending();
    this.pending.set(null);
    this.pendingMode.set(null);
    this.move(mode === null ? next : null);
    if (mode !== null) {
      this.host.applyMode(mode);
    }
  }
}
