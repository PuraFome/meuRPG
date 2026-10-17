import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant } from '../../../gen/meurpg/play/v1/combat_pb';
import type { Square } from '../../core/combat/combat-grid';
import {
  DFT_PER_FOOT,
  type TeleportScene,
  type Verdict,
  pickableSquares,
  squareSpoken,
  teleportJudge,
  verdictText,
} from '../../core/combat/teleport-pick';
import { rangeFt } from '../../core/combat/spell-area';
import type { MapLayers } from '../../core/maps/layers';
import type { Vision } from '../../core/maps/vision';
import { metersFixed, metersText } from '../../core/units';
import { CombatMap, type Chosen, type CombatMapImage, type Reach } from '../combat-map/combat-map';

/**
 * The square a teleport spell (Passo Nebuloso) takes the caster to, picked on the battle map the way a move is picked: the
 * map is the "Mover" page's, with the reach as its dashed circle and the squares that can be picked tinted.
 *
 * - **Pointer:** a click on a square picks it; one that is too far, taken, a wall or out of the caster's sight is drawn as
 *   refused and picks nothing. The master is not held to the reach or the sight and may pick anywhere (the server still refuses
 *   a wall or a creature).
 * - **Keyboard:** the map is one focus stop; the arrows move the square, and the first arrow starts from the caster. Enter says the
 *   square again. Every move is spoken in the live region (the column, the row, the distance and whether it can be picked).
 *
 * Presentational: it reports the square picked (`null` when the last one was refused); the cast sheet asks the server.
 */
@Component({
  selector: 'app-teleport-map',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CombatMap, MatIconModule],
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
      (confirm)="say()"
    />
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
  `,
})
export class TeleportMap {
  readonly image = input.required<CombatMapImage>();
  readonly mapName = input('');
  readonly columns = input.required<number>();
  readonly rows = input.required<number>();
  readonly combatants = input<readonly Combatant[]>([]);
  readonly layers = input<MapLayers | null>(null);
  readonly fog = input<Vision | null>(null);
  /** The master: not held to the reach or the sight, and may leave the square out. */
  readonly master = input(false);
  readonly casterId = input.required<string>();
  /** How far the spell takes the caster, in feet. */
  readonly reachFt = input.required<number>();
  /** The square picked, as the cast sheet has it. */
  readonly picked = input<Square | null>(null);

  /** A square that can be used, or `null` when the one just tried cannot. */
  readonly pick = output<Square | null>();

  /** The last square tried, with the verdict: the frame on the map and the words under it. */
  private readonly tried = signal<{ square: Square; verdict: Verdict } | null>(null);
  protected readonly spoken = signal('');

  private readonly caster = computed<Square | null>(() => {
    const c = this.combatants().find((x) => x.id === this.casterId() && x.placed);
    return c ? { col: c.col, row: c.row } : null;
  });
  private readonly scene = computed<TeleportScene>(() => ({
    caster: this.caster(),
    reachFt: this.reachFt(),
    master: this.master(),
    casterId: this.casterId(),
    combatants: this.combatants(),
    layers: this.layers(),
    fog: this.fog(),
    columns: this.columns(),
    rows: this.rows(),
  }));
  private readonly judge = computed(() => teleportJudge(this.scene()));

  protected readonly hint = computed(() => {
    const reach = metersText(this.reachFt());
    return this.master()
      ? `Escolha no mapa o espaço de chegada (até ${reach}). Sem escolher, a ficha fica onde está e você a move depois. As setas movem o quadrado.`
      : `Escolha no mapa o espaço para onde você vai, até ${reach}, num lugar que você vê. As setas movem o quadrado a partir de você.`;
  });
  /** The dashed circle of the reach, and the squares that can be picked tinted. */
  protected readonly reach = computed<Reach | null>(() => {
    const origin = this.caster();
    return origin
      ? {
          origin,
          leftDft: this.reachFt() * DFT_PER_FOOT,
          squares: pickableSquares(this.scene()),
        }
      : null;
  });
  protected readonly frame = computed<Chosen | null>(() => {
    const tried = this.tried();
    const square = tried?.square ?? this.picked();
    if (!square) {
      return null;
    }
    const from = this.caster();
    return {
      square,
      refused: !!tried && tried.verdict !== 'ok',
      ...(from ? { label: metersFixed(rangeFt(from, square)) } : {}),
    };
  });
  protected readonly refusal = computed(() => verdictText(this.tried()?.verdict ?? 'ok'));

  /** A click or an arrow on a square: it is the pick if it can be used, and the words say so either way. */
  protected choose(square: Square): void {
    const verdict = this.judge()(square);
    this.tried.set({ square, verdict });
    this.spoken.set(squareSpoken(square, this.caster(), verdict));
    this.pick.emit(verdict === 'ok' ? square : null);
  }

  /** Enter: the square in the frame is said again (the pick was made when the square was reached). */
  protected say(): void {
    const tried = this.tried();
    const square = tried?.square ?? this.picked();
    if (square) {
      this.spoken.set(squareSpoken(square, this.caster(), tried?.verdict ?? 'ok'));
    }
  }
}
