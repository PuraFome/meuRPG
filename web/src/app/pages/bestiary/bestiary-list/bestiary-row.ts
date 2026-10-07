import { Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
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
 * and size, the ND, the armor class and average hit points, and "Pôr no combate" (MR-042). The row opens the stat
 * block and keeps the search in the link: the name is the link, stretched over the whole row, and the button sits
 * above that stretch, so the two never nest.
 */
@Component({
  selector: 'app-bestiary-row',
  imports: [CreatureArt, MatButtonModule, RouterLink],
  template: `
    <div class="row">
      <app-creature-art class="row__art" [monsterKey]="row().key" [type]="row().type" />
      <span class="row__name">
        <a class="row__link" [routerLink]="['/campaigns', campaignId(), 'bestiary', row().slug]" [queryParams]="queryParams()">
          <span class="row__pt">{{ row().namePt }}</span>
        </a>
        <span class="row__en"><span lang="en">{{ row().name }}</span> · SRD</span>
      </span>
      <span class="row__meta">{{ row().meta }}</span>
      <span class="row__kind">{{ row().kind }}</span>
      <span class="row__nd">{{ row().nd }}</span>
      <span class="row__stats">{{ row().stats }}</span>
      <button type="button" matButton="outlined" class="row__put" [attr.aria-label]="'Pôr no combate: ' + row().namePt" (click)="put.emit(row())">
        Pôr no combate
      </button>
    </div>
  `,
  styleUrl: './bestiary-row.scss',
})
export class BestiaryRow {
  readonly campaignId = input.required<string>();
  readonly row = input.required<BestiaryRowData>();
  /** The list's search, so the stat block's "Voltar ao Bestiário" brings it back. */
  readonly queryParams = input<Record<string, string>>({});
  /** "Pôr no combate": the page opens the sheet for this creature. */
  readonly put = output<BestiaryRowData>();
}
