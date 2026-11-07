import { signal } from '@angular/core';

import { describeCharacterError } from '../../core/characters/character-errors';
import { combatErrorMessage, encounterBlocked } from '../../core/combat/combat-errors';
import type { GearToggle } from '../../shared/gear-list/gear-list';
import type { CharacterSheetSource, FullSheetVm } from './character-sheet.types';

/** What the equipping needs of the page that owns it. */
export interface SheetEquipHost {
  readonly source: CharacterSheetSource;
  readonly campaignId: () => string;
  /** The character on the page; empty when the page is gone. */
  readonly characterId: () => string;
  /** Reads the sheet again without the loading state; `true` when the new sheet is on screen. */
  readonly reload: () => Promise<boolean>;
  /** The sheet on screen, when it is a full one. */
  readonly sheet: () => FullSheetVm | null;
}

/**
 * "Equipar" and "Guardar" on the sheet: the server changes the piece, the page reads the sheet again for the names, the
 * groups and the armor class it derives, and the panel says what moved. A refusal of a running combat is said in words.
 */
export class SheetEquip {
  /** The piece whose change is on its way to the server. */
  readonly busyKey = signal<string | null>(null);
  readonly notice = signal('');
  readonly error = signal('');

  constructor(private readonly host: SheetEquipHost) {}

  async toggle(change: GearToggle): Promise<void> {
    const campaignId = this.host.campaignId();
    const characterId = this.host.characterId();
    if (!campaignId || !characterId || this.busyKey() !== null) {
      return;
    }
    this.busyKey.set(change.key);
    this.error.set('');
    try {
      await this.host.source.setEquipped(campaignId, characterId, change.key, change.equipped);
      const fresh = await this.host.reload();
      if (characterId !== this.host.characterId()) {
        return;
      }
      if (!fresh) {
        this.error.set(
          'A troca foi feita, mas não deu para ler a ficha de novo. Recarregue a página.',
        );
        return;
      }
      const full = this.host.sheet();
      const name = full?.gear.find((g) => g.key === change.key)?.namePt ?? '';
      this.notice.set(
        `${change.equipped ? 'Equipado' : 'Guardado'}: ${name}.${full ? ` Classe de armadura ${full.armorClass}.` : ''}`,
      );
    } catch (err) {
      this.notice.set('');
      this.error.set(
        encounterBlocked(err)
          ? combatErrorMessage(err, 'mudar o equipamento')
          : describeCharacterError(err),
      );
    } finally {
      this.busyKey.set(null);
    }
  }
}
