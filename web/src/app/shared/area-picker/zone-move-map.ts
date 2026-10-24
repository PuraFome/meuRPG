import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant, SpellZone } from '../../../gen/meurpg/play/v1/combat_pb';
import type { Square } from '../../core/combat/combat-grid';
import { DFT_PER_FOOT } from '../../core/combat/teleport-pick';
import {
  ZONE_MOVE_RANGE_FT,
  zoneMovePreview,
  zoneRangeText,
  zoneSquareSpoken,
} from '../../core/combat/zone-move';
import type { MapLayers } from '../../core/maps/layers';
import type { Vision } from '../../core/maps/vision';
import { metersFixed, metersText } from '../../core/units';
import { CombatMap, type Chosen, type CombatMapImage, type Reach } from '../combat-map/combat-map';
import { AreaOverlay } from './area-overlay';

/**
 * Where a spell zone that moves goes ("Mover o Raio Lunar"), picked on the battle map the way a teleport square is: the zone's
 * current footprint stays drawn dimmed ("Ponto atual"), the footprint it would have around the picked square is the preview
 * (open floor: a wall does not cut it here, the server cuts it on commit), and the dashed ring is the reach of a player's
 * move. The master is held to no range.
 *
 * - **Pointer:** a click on a square picks it; one past the range is drawn as refused and says why.
 * - **Keyboard:** the map is one focus stop; the arrows move the square (the first one starts from the zone's origin), Enter
 *   confirms. Every pick is spoken in the live region.
 *
 * Presentational: it reports the square picked and Enter; the sheet asks the server.
 */
@Component({
  selector: 'app-zone-move-map',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [AreaOverlay, CombatMap, MatIconModule],
  template: `
    <p class="hint">{{ hint() }}</p>
    <app-combat-map
      [image]="image()"
      [mapName]="mapName()"
      [columns]="columns()"
      [rows]="rows()"
      [combatants]="combatants()"
      [isMaster]="master()"
      [layers]="layers()"
      [fog]="fog()"
      [reach]="reach()"
      [chosen]="frame()"
      [pickSquares]="true"
      [showTurn]="false"
      (choose)="choose($event)"
      (confirm)="confirm.emit()"
    >
      <app-area-overlay
        class="zm__now"
        [columns]="columns()"
        [rows]="rows()"
        [squares]="zone().squares"
        [origin]="zone().origin ?? null"
      />
      @if (preview(); as p) {
        <app-area-overlay
          [columns]="columns()"
          [rows]="rows()"
          [squares]="p.squares"
          [origin]="picked()"
        />
      }
    </app-combat-map>
    <ul class="mr-legend" aria-label="Legenda do mapa">
      <li><span class="zm__sw zm__sw--new" aria-hidden="true"></span>Nova posição</li>
      <li><span class="zm__sw zm__sw--now" aria-hidden="true"></span>Ponto atual</li>
    </ul>
    @if (refusal(); as text) {
      <div class="mr-notice mr-notice--danger" role="alert">
        <mat-icon aria-hidden="true">block</mat-icon>
        <p>{{ text }}</p>
      </div>
    }
    <p class="mr-visually-hidden" role="status" aria-live="polite">{{ spoken() }}</p>
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
      min-width: 0;
    }

    .hint {
      margin: 0;
      font-size: 15px;
      line-height: 20px;
    }

    .zm__now {
      opacity: 0.5;
    }

    .zm__sw {
      display: inline-block;
      width: 14px;
      height: 14px;
      margin-right: var(--mr-space-2);
      border: 2px solid var(--mr-map-accent);
      vertical-align: middle;
    }

    .zm__sw--new {
      background: color-mix(in srgb, var(--mr-map-accent) 38%, transparent);
    }

    .zm__sw--now {
      border-style: dashed;
      opacity: 0.5;
    }
  `,
})
export class ZoneMoveMap {
  readonly image = input.required<CombatMapImage>();
  readonly mapName = input('');
  readonly columns = input.required<number>();
  readonly rows = input.required<number>();
  readonly combatants = input<readonly Combatant[]>([]);
  readonly layers = input<MapLayers | null>(null);
  readonly fog = input<Vision | null>(null);
  /** The zone being moved, as the combat has it now. */
  readonly zone = input.required<SpellZone>();
  /** The master is held to no range. */
  readonly master = input(false);
  /** The square picked, as the sheet has it. */
  readonly picked = input<Square | null>(null);

  /** A square was picked (a click, or an arrow key); the sheet decides if it can be confirmed. */
  readonly pick = output<Square>();
  /** Enter on the map. */
  readonly confirm = output<void>();

  protected readonly spoken = signal('');

  protected readonly preview = computed(() => {
    const at = this.picked();
    return at ? zoneMovePreview(this.zone(), at, this.master()) : null;
  });
  protected readonly hint = computed(() =>
    this.master()
      ? 'Escolha no mapa para onde a zona vai. Sem limite de alcance para o mestre. As setas movem o quadrado a partir do ponto atual.'
      : `Escolha no mapa para onde a zona vai, até ${metersText(ZONE_MOVE_RANGE_FT)} do ponto atual. As setas movem o quadrado a partir dele.`,
  );
  /** The ring of a player's reach around the zone's origin; the master has none (zero radius draws nothing). */
  protected readonly reach = computed<Reach | null>(() => {
    const origin = this.zone().origin;
    return origin
      ? {
          origin: { col: origin.col, row: origin.row },
          leftDft: this.master() ? 0 : ZONE_MOVE_RANGE_FT * DFT_PER_FOOT,
          squares: [],
        }
      : null;
  });
  protected readonly frame = computed<Chosen | null>(() => {
    const at = this.picked();
    const p = this.preview();
    return at && p ? { square: at, refused: !p.inRange, label: metersFixed(p.distanceFt) } : null;
  });
  protected readonly refusal = computed(() => {
    const p = this.preview();
    return p && !p.inRange ? zoneRangeText(p, this.master()) : '';
  });

  protected choose(square: Square): void {
    this.spoken.set(zoneSquareSpoken(square, zoneMovePreview(this.zone(), square, this.master())));
    this.pick.emit(square);
  }
}
