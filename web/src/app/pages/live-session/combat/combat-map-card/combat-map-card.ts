import { Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { PHONE_QUERY, mediaQuery } from '../../../../shared/map-view/media-query';
import { ownCombatant } from '../../../../core/combat/combat-view';
import { metersFixed } from '../../../../core/units';
import { type MapLayers, NO_LAYERS } from '../../../../core/maps/layers';
import {
  CombatMap,
  type CombatMapImage,
  type OfferMark,
  type Reach,
  type TokenDrop,
} from '../../../../shared/combat-map/combat-map';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';
import { MapLayersLegend } from '../../../../shared/map-layers/map-layers-legend';

/**
 * The map panel of a running combat or of its setup (E6-04, E6-05, E6-11):
 * the battle map with its legend. The master drags any token ("O mestre anda
 * sem limite"); a player drags their own on their turn, which opens the "Mover" page on that square (the page asks, with its warnings; a drop never moves).
 * The legend names every shape the map uses, so no meaning is colour alone: the
 * layer marks the map has (`app-map-layers-legend`), then the tokens and, when
 * they are drawn, the movement marks and the square of an opportunity attack's
 * reactor. The master can turn the reach of whoever is on turn on and off
 * ("Mostrar o alcance do Toren no mapa", E9-05): the server's `GetMoveOptions`
 * answer, drawn as it comes.
 */
@Component({
  selector: 'app-combat-map-card',
  imports: [CombatMap, CombatantToken, MapLayersLegend, MatButtonModule, MatIconModule],
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
  /** The map's painted layers, once read. */
  readonly layers = input<MapLayers | null>(null);
  /** The reactors of pending opportunity offers the caller answers or waits on (E9-13). */
  readonly offers = input<readonly OfferMark[]>([]);
  /** The master's switch for the reach of whoever is on turn: its name, and whether it is on. */
  readonly reachSwitch = input<{ readonly name: string; readonly on: boolean } | null>(null);
  /** The player's own combatant may be dragged to pick a square (their turn, desktop): the drop opens "Mover" there. */
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
  /** The master turned the reach on or off. */
  readonly reachChange = output<boolean>();
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
  protected readonly emptyLayers = NO_LAYERS;
  protected readonly reachMeters = computed(() => metersFixed((this.reach()?.leftDft ?? 0) / 10));
  protected readonly hasHidden = computed(() => this.encounter().combatants.some((c) => c.hidden));
  protected readonly hasDefeated = computed(() => this.encounter().combatants.some((c) => c.defeated));
}
