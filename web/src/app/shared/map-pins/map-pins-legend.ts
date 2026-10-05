import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { type MapPoint, MapPointKind, TrapState } from '../../../gen/meurpg/maps/v1/maps_pb';

interface Entry {
  readonly key: string;
  readonly label: string;
  readonly kind: 'trap' | 'treasure' | 'light';
  readonly state: string;
  readonly eye: boolean;
}

/**
 * The names of the trap, treasure and light marks on a map (MAP-LANGUAGE.md): one entry for each look
 * that is on the map right now, never colour alone. The master's marks say "só você vê". The screen
 * puts it next to the map's other legends (fog states before, layers and tokens after).
 */
@Component({
  selector: 'app-map-pins-legend',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (entries().length > 0) {
      <ul class="mr-legend" aria-label="Marcas do mapa">
        @for (e of entries(); track e.key) {
          <li>
            <span class="sw" [class]="'sw sw--' + e.kind + ' sw--' + e.state" aria-hidden="true">
              <mat-icon>{{ e.kind === 'trap' ? 'warning' : e.kind === 'treasure' ? 'inventory_2' : 'lightbulb' }}</mat-icon>
              @if (e.eye) {
                <span class="sw__eye"><mat-icon>visibility_off</mat-icon></span>
              }
            </span>
            {{ e.label }}
          </li>
        }
      </ul>
    }
  `,
  styles: `
    :host {
      display: block;
    }

    .sw {
      position: relative;
      display: inline-flex;
      flex: none;
      align-items: center;
      justify-content: center;
      box-sizing: border-box;
      width: 22px;
      height: 22px;
      border: 2px solid var(--mr-map-token-ink);
      border-radius: 5px;
      background: var(--mr-map-token-surface);
      color: var(--mr-map-token-ink);
    }

    .sw .mat-icon {
      width: 14px;
      height: 14px;
      font-size: 14px;
    }

    .sw--trap {
      border-color: var(--mr-map-accent);
      border-style: dashed;
      color: var(--mr-map-accent);
    }

    .sw--fired {
      border-style: solid;
      background: var(--mr-map-accent);
      color: var(--mr-map-token-surface);
    }

    .sw--disarmed {
      border-color: var(--mr-map-token-down-line);
      color: var(--mr-map-token-down-ink);
    }

    .sw--hidden,
    .sw--light {
      border-style: dashed;
    }

    .sw__eye {
      position: absolute;
      top: -6px;
      right: -6px;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 12px;
      height: 12px;
      border: 1px solid var(--mr-map-token-ink);
      border-radius: 50%;
      background: var(--mr-map-token-surface);
      color: var(--mr-map-token-ink);
    }

    .sw__eye .mat-icon {
      width: 8px;
      height: 8px;
      font-size: 8px;
    }
  `,
})
export class MapPinsLegend {
  readonly points = input.required<readonly MapPoint[]>();
  readonly isMaster = input(false);

  protected readonly entries = computed<readonly Entry[]>(() => {
    const out = new Map<string, Entry>();
    const master = this.isMaster();
    for (const p of this.points()) {
      if (p.kind === MapPointKind.TRAP) {
        const state = p.trap?.state ?? TrapState.ARMED;
        if (state === TrapState.TRIGGERED) {
          out.set('fired', { key: 'fired', label: 'Armadilha disparada', kind: 'trap', state: 'fired', eye: false });
        } else if (state === TrapState.DISARMED) {
          out.set('disarmed', { key: 'disarmed', label: 'Armadilha desarmada', kind: 'trap', state: 'disarmed', eye: false });
        } else if (master && !p.revealed && p.trapRevealedTo.length === 0) {
          out.set('secret', { key: 'secret', label: 'Armadilha (só você vê)', kind: 'trap', state: 'armed', eye: true });
        } else {
          out.set('armed', { key: 'armed', label: 'Armadilha', kind: 'trap', state: 'armed', eye: false });
        }
      } else if (p.kind === MapPointKind.TREASURE) {
        if (p.treasureFoundAt) {
          out.set('found', { key: 'found', label: 'Tesouro encontrado', kind: 'treasure', state: 'found', eye: false });
        } else {
          out.set('hidden', { key: 'hidden', label: 'Tesouro (escondido)', kind: 'treasure', state: 'hidden', eye: false });
        }
      } else if (p.kind === MapPointKind.LIGHT && master) {
        out.set('light', { key: 'light', label: 'Luz (só você vê)', kind: 'light', state: '', eye: false });
      }
    }
    return [...out.values()];
  });
}
