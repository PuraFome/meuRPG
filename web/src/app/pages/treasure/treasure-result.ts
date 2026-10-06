import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import type { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { type Treasure, TreasureMode } from '../../../gen/meurpg/maps/v1/treasure_pb';
import {
  CONSUMABLE_RULE,
  VALUE_LABEL,
  coinRows,
  goldLine,
  groupItems,
  pieceRows,
  po,
  treasureTitle,
} from '../../core/treasure/treasure-format';
import { TreasureItemRow } from './treasure-item-row';

/** The row the person opened ("Ver descrição"), and the button, so the page gives the focus back to it. */
export interface ItemOpen {
  readonly key: string;
  readonly namePt: string;
  readonly button: HTMLElement;
}

/**
 * A generated treasure as the server returned it (MR-044, E10-10 states 1 and 2): the coins with what each stack is worth in PO, the
 * gems and the art with their values, and the magic items grouped when identical ("2 ×") with the rarity, the value or "sem preço" and
 * the attunement; then the gold that becomes the point's gold, the items' value apart (they never become XP) and the line that says
 * what the gold does in this campaign. A card a kind on a phone, one frame from a tablet up. Presentational: it rolls nothing and
 * adds only the two totals the server gave.
 */
@Component({
  selector: 'app-treasure-result',
  imports: [MatButtonModule, MatIconModule, RouterLink, TreasureItemRow],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './treasure-result.html',
  styleUrl: './treasure-result.scss',
})
export class TreasureResult {
  readonly treasure = input.required<Treasure>();
  readonly xpMode = input.required<XpMode>();
  readonly campaignName = input.required<string>();
  /** The treasure is being replaced: the buttons rest. */
  readonly busy = input(false);

  readonly place = output<void>();
  readonly again = output<void>();
  readonly openItem = output<ItemOpen>();

  protected readonly valueLabel = VALUE_LABEL;
  protected readonly consumableRule = CONSUMABLE_RULE;
  protected readonly po = po;
  protected readonly hoard = computed(() => this.treasure().mode === TreasureMode.HOARD);
  protected readonly title = computed(() => treasureTitle(this.treasure().mode, this.treasure().partyLevel));
  protected readonly coins = computed(() => coinRows(this.treasure().coins));
  protected readonly gemRows = computed(() => pieceRows(this.treasure().gems));
  protected readonly artRows = computed(() => pieceRows(this.treasure().art));
  protected readonly items = computed(() => groupItems(this.treasure().items));
  protected readonly hasGemsOrArt = computed(() => this.gemRows().length + this.artRows().length > 0);
  protected readonly gemsAndArtPo = computed(() => this.treasure().gemsPo + this.treasure().artPo);
  protected readonly total = computed(() => this.treasure().goldPo + this.treasure().itemsPo);
  protected readonly goldSentence = computed(() => goldLine(this.xpMode(), this.campaignName()));

  protected open(key: string, namePt: string, button: HTMLElement): void {
    this.openItem.emit({ key, namePt, button });
  }
}
