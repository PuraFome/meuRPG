import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import { CreatureArt } from '../../../shared/creatures/creature-art';

/** One creature of the bestiary's list, as the page's rows say it. */
export interface BestiaryRowData {
  readonly key: string;
  /** The SRD type, for the picture. */
  readonly type: string;
  readonly slug: string;
  readonly namePt: string;
  /** The SRD's English name, in small type. */
  readonly name: string;
  /** "Fera · Médio". */
  readonly kind: string;
  /** "ND 1/4". */
  readonly nd: string;
  /** "CA 13 · PV 11". */
  readonly stats: string;
  /** All of it in one line, for a phone: "Fera · Médio · ND 1/4 · CA 13 · PV 11". */
  readonly meta: string;
}

/**
 * A row of the bestiary (E10-08): the picture, the Portuguese name with the SRD's in small type, the type
 * and size, the ND, and the armor class and average hit points. The whole row opens the stat block and keeps
 * the search in the link.
 */
@Component({
  selector: 'app-bestiary-row',
  imports: [CreatureArt, MatIconModule, RouterLink],
  template: `
    <a class="row" [routerLink]="['/campanhas', campaignId(), 'bestiario', row().slug]" [queryParams]="queryParams()">
      <app-creature-art class="row__art" [monsterKey]="row().key" [type]="row().type" />
      <span class="row__name">
        <span class="row__pt">{{ row().namePt }}</span>
        <span class="row__en"><span lang="en">{{ row().name }}</span> · SRD</span>
      </span>
      <span class="row__meta">{{ row().meta }}</span>
      <span class="row__kind">{{ row().kind }}</span>
      <span class="row__nd">{{ row().nd }}</span>
      <span class="row__stats">{{ row().stats }}</span>
      <mat-icon class="row__go" aria-hidden="true">chevron_right</mat-icon>
    </a>
  `,
  styleUrl: './bestiary-row.scss',
})
export class BestiaryRow {
  readonly campaignId = input.required<string>();
  readonly row = input.required<BestiaryRowData>();
  /** The list's search, so the stat block's "Voltar ao Bestiário" brings it back. */
  readonly queryParams = input<Record<string, string>>({});
}
