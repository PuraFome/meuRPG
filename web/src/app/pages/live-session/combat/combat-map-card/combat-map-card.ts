import { Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { PHONE_QUERY, mediaQuery } from '../../../../shared/map-view/media-query';
import { ownCombatant } from '../../../../core/combat/combat-view';
import {
  CombatMap,
  type CombatMapImage,
  type Reach,
  type TokenDrop,
} from '../../../../shared/combat-map/combat-map';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';

/**
 * The map panel of a running combat or of its setup (E6-04, E6-05, E6-11):
 * the battle map with its legend. The master drags any token ("O mestre anda
 * sem limite"); a player drags their own inside the reach, on their turn.
 * The legend names every shape the map uses, so no meaning is colour alone.
 */
@Component({
  selector: 'app-combat-map-card',
  imports: [CombatMap, CombatantToken, MatButtonModule, MatIconModule],
  templateUrl: './combat-map-card.html',
  styleUrl: './combat-map-card.scss',
})
export class CombatMapCard {
  readonly encounter = input.required<Encounter>();
  readonly image = input.required<CombatMapImage>();
  readonly mapName = input('');
  readonly isMaster = input(false);
  /** The panel's title: "Mapa" for the master, the map's name for a player. */
  readonly title = input('Mapa');
  readonly reach = input<Reach | null>(null);
  /** The player's own combatant may be dragged (their turn, desktop). */
  readonly ownMovable = input(false);
  readonly hint = input(true);
  /** The legend of the shapes; the setup's small map says it in a note. */
  readonly legend = input(true);
  /** A line under the map instead of the legend. */
  readonly note = input('');
  /** On a phone the master gets a zoomed, cropped preview centered on whoever
   * is on turn, and "Abrir mapa" for the whole map (E6-12). */
  readonly cropOnPhone = input(false);

  readonly tokenDrop = output<TokenDrop>();
  /** "Ver mapa": the player's full-screen, read-only map. */
  readonly openMap = output<void>();

  private readonly phone = mediaQuery(PHONE_QUERY);
  protected readonly preview = computed(() => this.cropOnPhone() && this.isMaster() && this.phone());

  /** Where the 2x map sits inside the preview frame, so that the one on turn
   * is in the middle (clamped, so no empty edge shows). The frame is 326 x 224
   * like the artboard: its height over the map's is `frameOverMap`. */
  protected readonly shift = computed(() => {
    const e = this.encounter();
    const focus = e.combatants.find((c) => c.id === e.currentCombatantId && c.placed) ??
      e.combatants.find((c) => c.placed);
    const fx = focus ? (focus.col + 0.5) / e.gridColumns : 0.5;
    const fy = focus ? (focus.row + 0.5) / e.gridRows : 0.5;
    const ratio = this.image().width / Math.max(1, this.image().height);
    const frameOverMap = ratio / 2 / (326 / 224);
    const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v));
    const x = clamp(-(fx - 0.25), -0.5, 0);
    const y = frameOverMap >= 1 ? 0 : clamp(-(fy - frameOverMap / 2), -(1 - frameOverMap), 0);
    return `translate(${x * 100}%, ${y * 100}%)`;
  });

  protected readonly own = computed(() => ownCombatant(this.encounter()));
  protected readonly size = computed(() => {
    const e = this.encounter();
    return `${e.gridColumns} × ${e.gridRows} quadrados de 1,5 m`;
  });
  protected readonly hasHidden = computed(() => this.encounter().combatants.some((c) => c.hidden));
  protected readonly hasDefeated = computed(() => this.encounter().combatants.some((c) => c.defeated));
}
