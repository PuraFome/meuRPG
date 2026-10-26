import { Component, input } from '@angular/core';

import { PanelAction } from '../panel-action/panel-action';
import { PanelHead } from '../panel-head/panel-head';

/**
 * The master's "Encontros" panel on "Preparação" (MR-043; board C6-02): the way in to the
 * encounter builder, "Montar encontro", in the title row, where he measures an encounter against
 * the party, draws one, and keeps it on a battle point. `CampaignDetail` renders it for the master
 * alone: the builder and what it keeps are his secret (RN-10).
 *
 * The board also lists the saved encounters with their difficulty. The server keeps an encounter
 * on a battle point of a map and has no list of them for the whole campaign, so the panel says
 * what the builder does instead.
 */
@Component({
  selector: 'app-encounters-panel',
  imports: [PanelAction, PanelHead],
  template: `
    <section class="mr-panel enc" aria-labelledby="encounters-panel-heading">
      <app-panel-head icon="swords" title="Encontros" titleId="encounters-panel-heading">
        <app-panel-action
          icon="add"
          label="Montar encontro"
          [link]="['/campaigns', campaignId(), 'encounters']"
        />
      </app-panel-head>
      <p class="enc__text">
        Monte um encontro com as criaturas do bestiário, veja se ele é baixo, moderado ou alto para o
        grupo e guarde-o num ponto de batalha.
      </p>
    </section>
  `,
  styles: `
    :host {
      display: block;
    }

    .enc {
      display: flex;
      flex-direction: column;
      gap: 12px;
      padding: 16px 20px 20px;
    }

    .enc__text {
      margin: 0;
      font-size: var(--mr-font-16);
      line-height: 22px;
      color: var(--mr-ink-muted);
    }
  `,
})
export class EncountersPanel {
  readonly campaignId = input.required<string>();
}
