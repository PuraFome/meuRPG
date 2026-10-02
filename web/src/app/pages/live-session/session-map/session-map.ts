import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * The map half of the session page (MR-012's map half, slice 5.3). The
 * maps don't exist yet, so today this only renders the empty state README-A
 * describes: a neutral notice in the map panel's place, and the vitals
 * don't move. The maps slice fills it: `mapId` is the session's current map
 * (`GetLiveSession` will carry it), `null` while the master hasn't chosen
 * one.
 */
@Component({
  selector: 'app-session-map',
  imports: [MatIconModule],
  template: `
    @if (mapId() === null) {
      <div class="mr-notice mr-notice--neutral">
        <mat-icon aria-hidden="true">map</mat-icon>
        @if (isMaster()) {
          <p>
            <strong>Nenhum mapa escolhido.</strong>
            Quando a sessão tiver um mapa atual, ele aparece aqui para você e para os jogadores.
          </p>
        } @else {
          <p>
            <strong>O mestre ainda não escolheu um mapa.</strong>
            Quando escolher, ele aparece aqui.
          </p>
        }
      </div>
    }
  `,
  styles: `
    :host {
      display: block;
    }
  `,
})
export class SessionMap {
  /** The session's current map, or `null` when there is none. */
  readonly mapId = input<string | null>(null);
  /** The master's copy speaks to the one who chooses the map. */
  readonly isMaster = input(false);
}
