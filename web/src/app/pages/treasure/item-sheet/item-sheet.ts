import { Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { RouterLink } from '@angular/router';

import type { GetMagicItemResponse } from '../../../../gen/meurpg/maps/v1/treasure_pb';
import { TreasureClient } from '../../../core/treasure/treasure-client';
import { itemFailure } from '../../../core/treasure/treasure-errors';
import { VALUE_LABEL, attunementText, itemTags, itemValueRule, itemValueText, rarityText } from '../../../core/treasure/treasure-format';
import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet } from '../../../shared/sheet/sheet-host';

/** What the result hands the sheet. */
export interface ItemSheetData {
  readonly campaignId: string;
  /** The item's key, such as "item:ring-of-protection". */
  readonly key: string;
  /** The name the row showed, for the title while the sheet loads. */
  readonly namePt: string;
}

type State = { status: 'loading' } | { status: 'error'; message: string } | { status: 'ready'; item: GetMagicItemResponse };

/**
 * "Ver descrição" (MR-044, E10-10 state 3): a magic item's rarity, its value with the "Valores do SRD 5.2.1 (regras de 2024)"
 * label and the credits link, the halving or the scroll's rule said in words, and the SRD 5.1's text in English (`lang="en"`, as
 * `GetMagicItem` returns it). A bottom sheet on a phone and a dialog from a tablet up (`openSheet`); on a 320 × 568 phone the title
 * and "Fechar" stay put and only the text scrolls.
 */
@Component({
  selector: 'app-item-sheet',
  imports: [MatButtonModule, MatIconModule, MatProgressSpinnerModule, RouterLink, SheetFrame],
  templateUrl: './item-sheet.html',
  styleUrl: './item-sheet.scss',
})
export class ItemSheet {
  private readonly client = inject(TreasureClient);
  private readonly sheet = injectSheet<ItemSheetData, void>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  protected readonly state = signal<State>({ status: 'loading' });
  protected readonly valueLabel = VALUE_LABEL;
  protected readonly item = computed(() => {
    const s = this.state();
    return s.status === 'ready' ? s.item : null;
  });
  protected readonly tags = computed(() => {
    const i = this.item();
    return i ? itemTags(i) : [];
  });
  protected readonly attunement = computed(() => {
    const i = this.item();
    return i ? attunementText(i) : '';
  });
  protected readonly value = computed(() => {
    const i = this.item();
    return i ? itemValueText(i) : '';
  });
  protected readonly rule = computed(() => {
    const i = this.item();
    return i ? itemValueRule(i) : '';
  });
  protected readonly rarity = computed(() => {
    const i = this.item();
    return i ? rarityText(i.rarity) : '';
  });

  constructor() {
    void this.load();
  }

  protected async load(): Promise<void> {
    this.state.set({ status: 'loading' });
    try {
      this.state.set({ status: 'ready', item: await this.client.item(this.data.campaignId, this.data.key) });
    } catch (err) {
      this.state.set({ status: 'error', message: itemFailure(err) });
    }
  }

  protected close(): void {
    this.sheet.close();
  }
}
