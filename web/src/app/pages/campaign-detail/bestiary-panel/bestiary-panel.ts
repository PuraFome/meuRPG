import { Component, input } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

/**
 * The master's "Bestiário" panel on `/campanhas/:id` (MR-042): the way in to the SRD's 334
 * creatures. `CampaignDetail` renders it for the master only: the SRD is public and the server
 * lets any member read it, but the app shows the bestiary (and "Criar NPC") to the master alone.
 */
@Component({
  selector: 'app-bestiary-panel',
  imports: [MatButtonModule, MatIconModule, RouterLink],
  template: `
    <section class="mr-panel bestiary" aria-labelledby="bestiary-panel-heading">
      <h2 class="mr-panel__title" id="bestiary-panel-heading">Bestiário</h2>
      <p class="bestiary__text">As 334 criaturas do SRD 5.1: procure pelo nome, abra a ficha e faça um NPC com nome a partir dela.</p>
      <a matButton="outlined" class="bestiary__open" [routerLink]="['/campanhas', campaignId(), 'bestiario']">
        <mat-icon aria-hidden="true">pets</mat-icon>Abrir o bestiário
      </a>
    </section>
  `,
  styles: `
    :host {
      display: block;
    }

    .bestiary {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-2);

      .mr-panel__title {
        margin: 0;
      }
    }

    .bestiary__text {
      margin: 0;
      font-size: 16px;
      line-height: 22px;
      color: var(--mr-ink-muted);
    }

    // The button is as wide as the panel on a phone, and sits on the left from a tablet up.
    .bestiary__open {
      align-self: flex-start;
    }

    @media (max-width: 767.98px) {
      .bestiary__open {
        align-self: stretch;
      }
    }
  `,
})
export class BestiaryPanel {
  readonly campaignId = input.required<string>();
}
