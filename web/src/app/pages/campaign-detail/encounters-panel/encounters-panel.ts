import { Component, input } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

/**
 * The master's "Encontros" panel on `/campanhas/:id` (MR-043): the way in to the encounter builder, where he measures an encounter
 * against the party, draws one, and keeps it on a battle point. `CampaignDetail` renders it for the master alone: the builder
 * and what it keeps are his secret (RN-10).
 */
@Component({
  selector: 'app-encounters-panel',
  imports: [MatButtonModule, MatIconModule, RouterLink],
  template: `
    <section class="mr-panel enc" aria-labelledby="encounters-panel-heading">
      <h2 class="mr-panel__title" id="encounters-panel-heading">Encontros</h2>
      <p class="enc__text">Monte um encontro com as criaturas do bestiário, veja se ele é baixo, moderado ou alto para o grupo e guarde-o num ponto de batalha.</p>
      <a matButton="outlined" class="enc__open" [routerLink]="['/campanhas', campaignId(), 'encontros']">
        <mat-icon aria-hidden="true">swords</mat-icon>Montar um encontro
      </a>
    </section>
  `,
  styles: `
    :host {
      display: block;
    }

    .enc {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-2);

      .mr-panel__title {
        margin: 0;
      }
    }

    .enc__text {
      margin: 0;
      font-size: 16px;
      line-height: 22px;
      color: var(--mr-ink-muted);
    }

    .enc__open {
      align-self: flex-start;
    }

    @media (max-width: 767.98px) {
      .enc__open {
        align-self: stretch;
      }
    }
  `,
})
export class EncountersPanel {
  readonly campaignId = input.required<string>();
}
