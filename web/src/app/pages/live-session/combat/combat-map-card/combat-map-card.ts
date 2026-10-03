import { Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
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

  readonly tokenDrop = output<TokenDrop>();
  /** "Ver mapa": the player's full-screen, read-only map. */
  readonly openMap = output<void>();

  protected readonly own = computed(() => ownCombatant(this.encounter()));
  protected readonly size = computed(() => {
    const e = this.encounter();
    return `${e.gridColumns} × ${e.gridRows} quadrados de 1,5 m`;
  });
  protected readonly hasHidden = computed(() => this.encounter().combatants.some((c) => c.hidden));
  protected readonly hasDefeated = computed(() => this.encounter().combatants.some((c) => c.defeated));
}
