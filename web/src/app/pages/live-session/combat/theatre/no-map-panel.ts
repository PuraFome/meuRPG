import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { THEATRE_WHY, THEATRE_WHY_PLAYER } from '../../../../core/combat/theatre';
import { GroupState } from '../action-groups/group-state';

/**
 * "Combate sem mapa" (RN-25, E10-04 state 10): where a player's map would be. One line says why (written once, here and in the start
 * dialog), no "Ver mapa", no legend, no fog: what depends on place does not exist, so no button is off for lack of a map. The order is the
 * strip above and the log below.
 */
@Component({
  selector: 'app-no-map-panel',
  imports: [MatIconModule],
  template: `
    <section class="panel" aria-labelledby="nomap-t" data-testid="no-map-panel">
      <h2 class="panel__title" id="nomap-t"><mat-icon aria-hidden="true">location_off</mat-icon>Combate sem mapa</h2>
      <p class="panel__text"><b>{{ why }}</b> {{ more }}</p>
    </section>
  `,
  styleUrl: './no-map-panel.scss',
})
export class NoMapPanel {
  protected readonly why = THEATRE_WHY;
  protected readonly more = THEATRE_WHY_PLAYER;
}

/**
 * "Reação" on the player's phone in a combat without a map (E10-04 state 10): where the opportunity attack comes from. The master offers it,
 * and the question opens by itself; the card says so and whether the reaction is free. It is the server's word (`reaction_used`), not a rule.
 */
@Component({
  selector: 'app-theatre-reaction',
  imports: [GroupState],
  template: `
    <section class="panel" aria-labelledby="react-t">
      <div class="panel__head">
        <h2 class="panel__title panel__title--plain" id="react-t">Reação</h2>
        <app-group-state [word]="used() ? 'Usada' : 'Disponível'" [used]="used()" />
      </div>
      <p class="panel__text">Se o mestre disser que um inimigo está saindo do seu alcance, a pergunta do ataque de oportunidade abre aqui.</p>
    </section>
  `,
  styleUrl: './no-map-panel.scss',
})
export class TheatreReaction {
  readonly used = input(false);
}
