import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { MapPoint } from '../../../gen/meurpg/maps/v1/maps_pb';
import { ChestIcon } from '../chest-icon/chest-icon';
import { foundLine, poText } from '../../core/traps/treasure-text';

/**
 * What a player reads of a treasure that was found (E9-09 4): "Encontrado" by whom and when, the value in
 * PO (the unit tied to the number) and what is inside. The server sends the value and the description only
 * for a treasure that was found, and never what the master wrote for himself; this draws what it is given.
 * Used by the session page's sheet and by the full map's point sheet.
 */
@Component({
  selector: 'app-treasure-facts',
  imports: [ChestIcon, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <p class="tf__by">
      <span class="mr-tag mr-tag--success"><mat-icon aria-hidden="true">check</mat-icon>Encontrado</span>
      @if (line().names) {
        <span>por <b>{{ line().names }}</b>{{ line().at ? ' às\u00a0' + line().at : '' }}</span>
      }
    </p>
    <div class="tf__value">
      <app-chest-icon />
      <span class="tf__vt">
        <span class="tf__cap">Valor</span>
        <b class="tf__num">{{ value() }}</b>
      </span>
    </div>
    @if (point().description) {
      <p class="tf__cap tf__cap--in">O que tem dentro</p>
      <p class="tf__inside">{{ point().description }}</p>
    }
  `,
  styles: `
    :host {
      display: block;
    }

    .tf__by {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 6px 8px;
      margin: 0;
      font-size: 15px;
    }

    .tf__value {
      display: flex;
      align-items: center;
      gap: var(--mr-space-3);
      margin-top: var(--mr-space-3);
      padding: 10px 14px;
      border: 1px solid var(--mr-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-ground);
    }

    .tf__vt {
      display: flex;
      flex-direction: column;
    }

    .tf__cap {
      font-size: 13px;
      line-height: 17px;
      color: var(--mr-ink-muted);
    }

    .tf__cap--in {
      margin: var(--mr-space-3) 0 0;
    }

    .tf__num {
      font-family: var(--mr-font-display);
      font-size: 24px;
      line-height: 30px;
    }

    .tf__inside {
      margin: 2px 0 0;
      font-size: 16px;
      line-height: 22px;
      overflow-wrap: anywhere;
    }
  `,
})
export class TreasureFacts {
  readonly point = input.required<MapPoint>();
  protected readonly line = computed(() => foundLine(this.point()));
  protected readonly value = computed(() => poText(this.point().treasureValuePo));
}
