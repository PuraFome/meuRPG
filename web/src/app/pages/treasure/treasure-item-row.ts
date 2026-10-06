import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

import { type ItemGroup, itemTags } from '../../core/treasure/treasure-format';

/**
 * One magic item of a treasure (E10-10): the Portuguese name and the SRD's in English (`lang="en"`), the value (and "50 PO cada" and
 * "metade de 100 PO" under it), the tags as words, and "Ver descrição", which hands the button back so the page can return the focus to it.
 */
@Component({
  selector: 'app-treasure-item-row',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="item">
      <p class="item__head">
        <span class="item__names">
          <b class="item__name">{{ group().title }}</b>
          <span class="item__en" lang="en">{{ group().item.name }}</span>
        </span>
        <span class="item__val">
          <b>{{ group().value }}</b>
          @if (group().each) {
            <span class="row__sub">{{ group().each }}</span>
          }
          @if (group().halvedNote) {
            <span class="row__sub">{{ group().halvedNote }}</span>
          }
        </span>
      </p>
      <ul class="tags" [attr.aria-label]="'Etiquetas de ' + group().item.namePt">
        @for (tag of tags(); track tag) {
          <li class="mr-tag">{{ tag }}</li>
        }
      </ul>
      <button type="button" class="item__desc" [attr.aria-label]="'Ver descrição: ' + group().item.namePt" (click)="describe.emit($any($event.currentTarget))">
        Ver descrição
      </button>
    </div>
  `,
  styleUrl: './treasure-item-row.scss',
})
export class TreasureItemRow {
  readonly group = input.required<ItemGroup>();
  /** The button that asked, for the focus to come back to it. */
  readonly describe = output<HTMLElement>();
  protected readonly tags = () => itemTags(this.group().item);
}
