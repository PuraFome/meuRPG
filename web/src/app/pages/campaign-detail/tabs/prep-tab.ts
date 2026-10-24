import {
  ChangeDetectionStrategy,
  Component,
  Injector,
  afterNextRender,
  inject,
} from '@angular/core';
import { ActivatedRoute } from '@angular/router';

import { BestiaryPanel } from '../bestiary-panel/bestiary-panel';
import { CampaignNpcs } from '../characters/campaign-npcs';
import { CampaignView } from '../campaign-view';
import { DocumentPanel } from '../document-panel/document-panel';
import { EncountersPanel } from '../encounters-panel/encounters-panel';
import { GalleryPanel } from '../gallery-panel/gallery-panel';
import { MapsPanel } from '../maps-panel/maps-panel';
import { PuzzlesPanel } from '../../../shared/puzzles-panel/puzzles-panel';

/**
 * The master's "Preparação" tab, `/campaigns/:id/prep`: what the master makes before a session.
 * "Mapas" (with "Gerar masmorra" and "Gerar tesouro"), "NPCs", "Encontros", "Quebra-cabeças",
 * "Bestiário", "Galeria" and "Documento da campanha", as board C6-02 lays them out: the lists
 * of what the master builds on the left (maps, encounters, puzzles), the people and the images on
 * the right, and the bestiary and the document as one row each.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-prep-tab',
  imports: [
    BestiaryPanel,
    CampaignNpcs,
    DocumentPanel,
    EncountersPanel,
    GalleryPanel,
    MapsPanel,
    PuzzlesPanel,
  ],
  template: `
    @if (view.data(); as d) {
      <div class="tab-layout">
        <div class="tab-layout__column">
          <app-maps-panel [campaignId]="d.campaign.id" [isMaster]="true" />
          <app-encounters-panel [campaignId]="d.campaign.id" />
          <app-puzzles-panel [campaignId]="d.campaign.id" />
        </div>
        <div class="tab-layout__column">
          <app-campaign-npcs [campaignId]="d.campaign.id" />
          <app-gallery-panel [campaignId]="d.campaign.id" />
          <app-bestiary-panel [campaignId]="d.campaign.id" />
          <app-document-panel [campaignId]="d.campaign.id" />
        </div>
      </div>
    }
  `,
  styleUrl: './prep-tab.scss',
})
export class PrepTab {
  protected readonly view = inject(CampaignView);

  constructor() {
    // A link with a fragment ("Abrir os mapas", in "Regras da mesa") lands on its panel once the tab has drawn it.
    const fragment = inject(ActivatedRoute).snapshot.fragment;
    if (fragment) {
      afterNextRender(() => document.getElementById(fragment)?.scrollIntoView?.(), {
        injector: inject(Injector),
      });
    }
  }
}
