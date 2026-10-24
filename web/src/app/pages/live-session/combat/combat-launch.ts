import { Component, computed, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import type { Observable } from 'rxjs';

import type { Encounter } from '../../../../gen/meurpg/play/v1/combat_pb';
import type { MapState } from '../../../core/maps/map-state';
import {
  type CombatMapInfo,
  StartCombatDialog,
  type StartCombatData,
} from '../../../shared/start-combat/start-combat-dialog';

/** The session's current map as the start dialog wants it, or `null` while there is none or it has no image. */
export function combatMapInfo(state: MapState): CombatMapInfo | null {
  const map = state.map();
  return map?.image
    ? {
        id: map.id,
        name: map.name,
        image: { url: map.image.url, width: map.image.width, height: map.image.height },
        columns: map.gridColumns,
        rows: map.gridRows,
      }
    : null;
}

/** Opens "Iniciar combate" (a full-screen dialog on a phone) and answers with the combat it started, or `undefined`. */
export function openStartCombat(
  dialog: MatDialog,
  phone: boolean,
  data: StartCombatData,
): Observable<Encounter | undefined> {
  return dialog
    .open<StartCombatDialog, StartCombatData, Encounter>(StartCombatDialog, {
      data,
      width: phone ? '100vw' : '760px',
      maxWidth: phone ? '100vw' : 'calc(100vw - 32px)',
      height: phone ? '100dvh' : undefined,
      maxHeight: phone ? '100dvh' : '92dvh',
      ariaLabelledBy: 'start-title',
      autoFocus: 'first-tabbable',
    })
    .afterClosed();
}

/**
 * "Iniciar combate" on the master's session page while there is no combat (MR-013, E6-01): the session bar's button
 * (and, on a phone, the menu's first item) asks for this. The map the fight is on is the session's current map; the
 * button waits while it is read, and a map that could not be read starts nothing (`CombatLaunchNotice` says so).
 */
export class CombatLaunch {
  constructor(
    private readonly dialog: MatDialog,
    private readonly phone: () => boolean,
    private readonly campaignId: () => string,
    private readonly state: () => MapState,
    private readonly started: (encounter: Encounter) => void,
  ) {}

  /** The session's current map is still being read: "Iniciar combate" waits for it. */
  readonly loading = computed(() => this.state().status() === 'loading');

  /** The read of the current map failed: unlike a session without a map, its answer is unknown. */
  readonly unreadable = computed(() => this.state().status() === 'error');

  /** The button is off: the dialog takes the map when it opens, and a click now would offer only the theatre of the mind. */
  readonly off = computed(() => this.loading() || this.unreadable());

  open(): void {
    if (this.off()) {
      return;
    }
    // Without a map the combat starts without one (the theatre of the mind, RN-25): the dialog offers the choice.
    openStartCombat(this.dialog, this.phone(), {
      campaignId: this.campaignId(),
      mode: 'start',
      map: combatMapInfo(this.state()),
    }).subscribe((encounter) => {
      if (encounter) {
        this.started(encounter);
      }
    });
  }
}

/** The line under the session bar when the current map could not be read, with "Tentar de novo": no combat starts on a guess. */
@Component({
  selector: 'app-combat-launch-notice',
  imports: [MatButtonModule, MatIconModule],
  template: `
    <div class="mr-notice mr-notice--warning">
      <mat-icon aria-hidden="true">warning</mat-icon>
      <p>
        <strong>Não deu para ler o mapa atual.</strong>
        Sem ele o combate começaria no teatro da mente, e isso não muda depois de iniciado.
      </p>
      <button matButton="outlined" type="button" (click)="retry.emit()">
        <mat-icon aria-hidden="true">refresh</mat-icon>Tentar de novo
      </button>
    </div>
  `,
  styles: ':host { display: block; }',
})
export class CombatLaunchNotice {
  readonly retry = output<void>();
}
