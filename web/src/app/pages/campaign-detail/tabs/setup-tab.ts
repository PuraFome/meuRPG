import { ChangeDetectionStrategy, Component, inject } from '@angular/core';

import { CampaignView } from '../campaign-view';
import { ContentSummary } from '../content-summary/content-summary';
import { DicePanel } from '../dice-panel/dice-panel';
import { PanelAction } from '../panel-action/panel-action';
import { PanelHead } from '../panel-head/panel-head';
import { RulesSummary } from '../rules-summary/rules-summary';

/**
 * The master's "Regras" tab, `/campaigns/:id/setup` (`/rules` is the "Regras da mesa" page): how
 * the table is set up, as board C6-03 lays it out. On the left "Dados" and "Regras da mesa"
 * (the rules in words); on the right "Conteúdo da mesa", "Magias" and "Pacote da campanha".
 * Each opens the page where it is changed.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-setup-tab',
  imports: [ContentSummary, DicePanel, PanelAction, PanelHead, RulesSummary],
  template: `
    @if (view.data(); as d) {
      <div class="tab-layout">
        <div class="tab-layout__column">
          <app-dice-panel
            [campaignId]="d.campaign.id"
            [savedMode]="d.campaign.diceMode"
            [members]="d.members"
          />
          <app-rules-summary [campaignId]="d.campaign.id" [xpMode]="d.campaign.xpMode" />
        </div>
        <div class="tab-layout__column">
          <app-content-summary [campaignId]="d.campaign.id" />
          <section class="mr-panel setup-panel" aria-labelledby="spells-heading">
            <app-panel-head icon="auto_fix_high" title="Magias" titleId="spells-heading" />
            <p class="setup-panel__text">
              As magias do SRD e as da mesa, com busca por nome, classe, nível da magia e escola.
            </p>
            <div>
              <app-panel-action
                label="Abrir as magias"
                [underline]="true"
                [link]="['/campaigns', d.campaign.id, 'spells']"
              />
            </div>
          </section>
          <section class="mr-panel setup-panel" aria-labelledby="export-heading">
            <app-panel-head icon="layers" title="Pacote da campanha" titleId="export-heading" />
            <p class="setup-panel__text">
              Um arquivo com tudo o que é da mesa, para criar a mesma campanha em outro servidor.
            </p>
            <div>
              <app-panel-action
                icon="download"
                label="Exportar campanha"
                [link]="['/campaigns', d.campaign.id, 'export']"
              />
            </div>
          </section>
        </div>
      </div>
    }
  `,
  styleUrl: './setup-tab.scss',
})
export class SetupTab {
  protected readonly view = inject(CampaignView);
}
