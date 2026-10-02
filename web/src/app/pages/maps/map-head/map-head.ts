import { Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import type { Map as MapMessage } from '../../../../gen/meurpg/maps/v1/maps_pb';

/**
 * The master's header of a map (E5-23, E5-24): "← Voltar para a campanha",
 * the name, and the map's state in words: "Revelado aos jogadores" (eye)
 * with the text button "Esconder", or "Escondido dos jogadores" (eye-off)
 * with "Revelar aos jogadores". On a computer, also "Imagem: <nome>" with
 * "Trocar imagem". The page runs the calls; this only asks.
 */
@Component({
  selector: 'app-map-head',
  imports: [MatButtonModule, MatIconModule, RouterLink],
  templateUrl: './map-head.html',
  styleUrl: './map-head.scss',
})
export class MapHead {
  readonly campaignId = input.required<string>();
  readonly map = input.required<MapMessage>();
  readonly showImage = input(true);
  readonly busy = input(false);
  readonly toggleReveal = output<void>();
  readonly changeImage = output<void>();
}
