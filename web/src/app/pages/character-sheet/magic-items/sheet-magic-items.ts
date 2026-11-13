import { type Signal, signal } from '@angular/core';
import type { Observable } from 'rxjs';

import type { AttunementEntryVm, AttunementVm } from '../../../core/characters/attunement';
import { describeCharacterError } from '../../../core/characters/character-errors';
import type { SheetEquipHost } from '../character-sheet-equip';
import type { AttunementSheetData } from './attunement-sheet';
import type { MagicItemPickerData } from './magic-item-picker-sheet';
import type { RemoveMagicItemData } from './remove-magic-item-sheet';

/** What the magic items and the attunement need of the page that owns them. */
export interface SheetMagicItemsHost extends Pick<
  SheetEquipHost,
  'source' | 'campaignId' | 'characterId' | 'reload' | 'vm' | 'apply'
> {
  readonly attunement: Signal<AttunementVm>;
  readonly revision: Signal<number>;
  /** The owner's window (`CharacterVitals.attunement_open`). */
  readonly windowOpen: Signal<boolean>;
  readonly openAttunement: (data: AttunementSheetData) => Observable<void | undefined>;
  readonly openPicker: (data: MagicItemPickerData) => Observable<boolean | undefined>;
  readonly openRemove: (data: RemoveMagicItemData) => Observable<boolean | undefined>;
}

/**
 * The sheet's magic items: "Mudar" (the sheet "Sintonização"), "Adicionar item mágico" and "Remover item". Adding and
 * removing are a full-sheet save at the revision the page holds (a stale one is refused and nothing is saved); an
 * attunement is one `SetAttunement` taking effect at once, inside the section.
 */
export class SheetMagicItems {
  /** What the last add or remove said, for the live region under the list. */
  readonly notice = signal('');

  constructor(private readonly host: SheetMagicItemsHost) {}

  /** The owner's window, for the "Mudar" button. */
  windowOpen(): boolean {
    return this.host.windowOpen();
  }

  openAttunement(): void {
    const vm = this.host.vm();
    if (!vm?.fullSheet) {
      return;
    }
    this.notice.set('');
    this.host
      .openAttunement({
        campaignId: this.host.campaignId(),
        characterId: vm.id,
        characterName: vm.name,
        isMaster: vm.isMaster,
        dead: vm.state === 'dead',
        attunement: this.host.attunement,
        revision: this.host.revision,
        windowOpen: this.host.windowOpen,
        changed: () => void this.host.reload(),
      })
      .subscribe();
  }

  openPicker(): void {
    this.notice.set('');
    this.host
      .openPicker({
        campaignId: this.host.campaignId(),
        add: (key) => this.save((items) => [...items, { id: '', key }]),
      })
      .subscribe((added) => {
        if (added) {
          this.notice.set('Item mágico adicionado.');
        }
      });
  }

  openRemove(entry: AttunementEntryVm): void {
    const counted = entry.asks && entry.attuned;
    this.notice.set('');
    this.host
      .openRemove({
        itemName: entry.namePt,
        attunedBefore: counted ? this.host.attunement().attuned : null,
        remove: () => this.save((items) => items.filter((i) => i.id !== entry.id)),
      })
      .subscribe((removed) => {
        if (removed) {
          this.notice.set(`${entry.namePt} saiu da ficha.`);
        }
      });
  }

  /** Saves the list of magic items `change` makes of the current one over the whole stored sheet; answers the refusal in
   * words, or '' when it is saved. */
  private async save(
    change: (items: { id: string; key: string }[]) => { id: string; key: string }[],
  ): Promise<string> {
    const vm = this.host.vm();
    if (!vm?.fullSheet) {
      return 'A ficha não está aberta.';
    }
    const now = this.host
      .attunement()
      .entries.filter((e) => e.kind === 'magic')
      .map((e) => ({ id: e.id, key: e.key }));
    try {
      const saved = await this.host.source.setMagicItems(
        this.host.campaignId(),
        vm.id,
        vm.revision,
        vm.name,
        vm.fullSheet,
        change(now),
      );
      this.host.apply(vm.id, saved);
      return '';
    } catch (err) {
      return describeCharacterError(err);
    }
  }
}
