import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import type { CoverDegree } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CoverMark } from './cover-mark';

/**
 * What a row of the master's order says about cover (E9-07): the cover the
 * combatant has against whoever has the turn, with its pictogram and its source
 * ("Três quartos (do mapa) contra o Pensantus"), the text action "Marcar
 * cobertura" (named for the combatant) and, when it is open, the radio group of
 * the master's own mark (`CoverMark`).
 */
@Component({
  selector: 'app-row-cover',
  imports: [CoverMark, MatButtonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (line()) {
      <span class="cover">
        @if (pictogram(); as mark) {
          <span class="mr-swatch mr-swatch--sm" [class.mr-swatch--half]="mark === 'half'" [class.mr-swatch--three]="mark === 'three'" aria-hidden="true"></span>
        }
        {{ line() }}
      </span>
    }
    @if (offer() && !marking()) {
      <button mat-button type="button" class="action" [attr.aria-label]="'Marcar cobertura de ' + label()" (click)="open.emit()">
        Marcar cobertura
      </button>
    }
    @if (marking()) {
      <app-cover-mark [label]="label()" [current]="current()" [busy]="busy()" (pick)="pick.emit($event)" (close)="close.emit()" />
    }
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }

    .cover {
      display: flex;
      align-items: center;
      gap: 6px;
      font-size: 14px;
      line-height: 19px;
      color: var(--mr-ink-muted);
    }

    // A text action that starts a line pulls its padding back, to line up with the words above.
    .action {
      align-self: flex-start;
      margin-inline-start: -12px;
    }
  `,
})
export class RowCover {
  readonly label = input.required<string>();
  /** "Três quartos (do mapa) contra o Pensantus", or `''`. */
  readonly line = input('');
  readonly pictogram = input<'half' | 'three' | null>(null);
  /** The master's mark now. */
  readonly current = input<CoverDegree>(1);
  /** Show "Marcar cobertura": cover matters here (a line, or a mark already there). */
  readonly offer = input(false);
  readonly marking = input(false);
  readonly busy = input(false);
  readonly open = output<void>();
  readonly close = output<void>();
  readonly pick = output<CoverDegree>();
}
