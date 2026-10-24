import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

/**
 * The master's "Bestiário" row on "Preparação" (MR-042; board C6-02): one row that is the link to
 * the SRD's 334 creatures. The tab renders it for the master only: the SRD is public and the
 * server lets any member read it, but the app shows the bestiary (and "Criar NPC") to the master alone.
 */
@Component({
  selector: 'app-bestiary-panel',
  imports: [MatIconModule, RouterLink],
  template: `
    <a class="mr-panel row" [routerLink]="['/campaigns', campaignId(), 'bestiary']">
      <mat-icon class="row__icon" aria-hidden="true">menu_book</mat-icon>
      <span class="row__text">
        <span class="row__title">Bestiário</span>
        <span class="row__sub">334 criaturas do SRD 5.1 · “Criar NPC” copia a ficha</span>
      </span>
      <mat-icon class="row__chevron" aria-hidden="true">chevron_right</mat-icon>
    </a>
  `,
  styleUrl: './bestiary-panel.scss',
})
export class BestiaryPanel {
  readonly campaignId = input.required<string>();
}
