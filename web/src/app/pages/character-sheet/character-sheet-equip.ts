import { Code, ConnectError } from '@connectrpc/connect';
import { signal } from '@angular/core';
import type { Observable } from 'rxjs';

import { describeCharacterError } from '../../core/characters/character-errors';
import { combatErrorMessage, encounterBlocked } from '../../core/combat/combat-errors';
import type { GearVm, MarksVm } from '../../core/characters/gear';
import type { GearToggle } from '../../shared/gear-list/gear-list';
import type { PieceBonusData } from './piece-bonus/piece-bonus-sheet';
import type { WeaponChange, WeaponMarksData } from './weapon-marks/weapon-marks-sheet';
import { marksOf } from './character-sheet-marks';
import type {
  CharacterSheetSource,
  CharacterSheetVm,
  FullSheetVm,
  GearChange,
} from './character-sheet.types';

const STALE_BONUS =
  'Não foi possível salvar o bônus. A ficha mudou enquanto você editava. Nada foi salvo: feche e abra de novo.';
const STALE_MARKS =
  'Não foi possível salvar as marcas e o bônus. A ficha mudou enquanto você editava. Nada foi salvo: feche e abra de novo.';

/** The marks and the bonus the sheet stores for a piece. */
interface Stored {
  readonly marks: MarksVm;
  readonly bonus: number;
}

function storedOf(vm: CharacterSheetVm | null, key: string): Stored {
  const full = vm?.fullSheet;
  const mark = full?.weaponMarks.find((m) => m.weaponKey === key);
  return {
    marks: mark ? marksOf(mark) : { silvered: false, adamantine: false, magical: false },
    bonus: full?.itemBonuses.find((b) => b.key === key)?.bonus ?? 0,
  };
}

/** Whether the stored sheet still lists the piece: its armor, its shield or one of its weapons. */
function onSheet(vm: CharacterSheetVm, piece: GearVm): boolean {
  const full = vm.fullSheet;
  if (!full) {
    return false;
  }
  return piece.kind === 'armor'
    ? full.armorKey === piece.key
    : piece.kind === 'shield'
      ? full.shield
      : full.weaponKeys.includes(piece.key);
}

function sameStored(a: Stored, b: Stored): boolean {
  return (
    a.bonus === b.bonus &&
    a.marks.silvered === b.marks.silvered &&
    a.marks.adamantine === b.marks.adamantine &&
    a.marks.magical === b.marks.magical
  );
}

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
  /** The character as the page holds it: its stored full sheet and revision are what a change of the marks is saved over. */
  readonly vm: () => CharacterSheetVm | null;
  /** Shows the answer of a write as the newest word on its character; `false` when the page left it. */
  readonly apply: (characterId: string, vm: CharacterSheetVm) => boolean;
  /** Opens the sheet of a weapon's marks; it answers `true` when they were saved. */
  readonly openMarksSheet: (data: WeaponMarksData) => Observable<boolean | undefined>;
  /** Opens the sheet of an armor's or a shield's bonus; it answers `true` when it was saved. */
  readonly openBonusSheet: (data: PieceBonusData) => Observable<boolean | undefined>;
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
  /** The sheet could not be read after a change: "Tentar de novo" asks again. */
  readonly readFailed = signal(false);

  /** What the last save of a piece's marks or bonus said ("Marcas e bônus salvos."), for the live region under the list. */
  readonly marksNotice = signal('');

  constructor(private readonly host: SheetEquipHost) {}

  /**
   * The piece's "⋮": a weapon's marks and bonus, or an armor's or a shield's bonus, in a sheet.
   * "Salvar" saves over the whole stored sheet, at the revision the page holds.
   * A piece changed since the sheet opened is not saved over.
   */
  openPiece(piece: GearVm): void {
    const vm = this.host.vm();
    if (!vm?.fullSheet) {
      return;
    }
    this.marksNotice.set('');
    const opened = storedOf(vm, piece.key);
    const common = {
      characterName: vm.name,
      isMaster: vm.isMaster,
      bonus: piece.bonus ?? 0,
      reason: piece.bonusReason ?? '',
    };
    if (piece.kind !== 'weapon') {
      this.host
        .openBonusSheet({
          ...common,
          pieceName: piece.namePt,
          kind: piece.kind,
          save: (bonus) =>
            this.saveGear({ bonus: { key: piece.key, bonus } }, piece, opened, STALE_BONUS),
        })
        .subscribe((saved) => saved && this.marksNotice.set('Bônus salvo.'));
      return;
    }
    const current =
      vm.sheet.kind === 'full'
        ? vm.sheet.weaponMarks.find((m) => m.weaponKey === piece.key)
        : undefined;
    this.host
      .openMarksSheet({
        ...common,
        weaponName: piece.namePt,
        copies: piece.count,
        marks: current ?? piece.marks ?? { silvered: false, adamantine: false, magical: false },
        save: (change) => this.saveWeapon(piece, change, opened),
      })
      .subscribe((saved) => saved && this.marksNotice.set('Marcas e bônus salvos.'));
  }

  /** Saves one weapon's marks and bonus together; the answer is the refusal in words, or '' when it is saved. */
  private saveWeapon(piece: GearVm, change: WeaponChange, opened: Stored): Promise<string> {
    const weaponKey = piece.key;
    const marks = change.marks;
    const mine =
      marks.silvered || marks.adamantine || marks.magical ? [{ weaponKey, ...marks }] : [];
    const others = (this.host.vm()?.fullSheet?.weaponMarks ?? []).filter(
      (m) => m.weaponKey !== weaponKey,
    );
    return this.saveGear(
      {
        marks: [...others.map((m) => ({ weaponKey: m.weaponKey, ...marksOf(m) })), ...mine],
        bonus: { key: weaponKey, bonus: change.bonus },
      },
      piece,
      opened,
      STALE_MARKS,
    );
  }

  /**
   * Saves a change of the gear over the whole stored sheet.
   * The answer is the refusal in words, or '' when it is saved.
   * A piece that is not as the sheet opened with, a piece the stored sheet no longer lists, or a stale revision,
   * answers `stale` and reads the sheet again.
   */
  private async saveGear(
    change: GearChange,
    piece: GearVm,
    opened: Stored,
    stale: string,
  ): Promise<string> {
    const vm = this.host.vm();
    const campaignId = this.host.campaignId();
    if (!vm?.fullSheet || !campaignId) {
      return 'A ficha não está aberta.';
    }
    if (!onSheet(vm, piece) || !sameStored(opened, storedOf(vm, piece.key))) {
      await this.host.reload();
      return stale;
    }
    try {
      const saved = await this.host.source.saveGear(
        campaignId,
        vm.id,
        vm.revision,
        vm.name,
        vm.fullSheet,
        change,
      );
      this.host.apply(vm.id, saved);
      return '';
    } catch (err) {
      if (ConnectError.from(err, Code.Unavailable).code === Code.Aborted) {
        await this.host.reload();
        return stale;
      }
      return describeCharacterError(err);
    }
  }

  /** The tab is shown again: the sheet is read again, so a change made from another tab or by the master is not left
   * on screen. Returns what stops the listening. */
  refreshWhenShown(doc: Document): () => void {
    const onShown = () => {
      if (doc.visibilityState === 'visible' && this.busyKey() === null && this.host.characterId()) {
        void this.host.reload();
      }
    };
    doc.addEventListener('visibilitychange', onShown);
    return () => doc.removeEventListener('visibilitychange', onShown);
  }

  /** "Tentar de novo": reads the sheet again after a change whose list could not be read. */
  async retry(): Promise<void> {
    if (await this.host.reload()) {
      this.readFailed.set(false);
      this.error.set('');
    }
  }

  async toggle(change: GearToggle): Promise<void> {
    const campaignId = this.host.campaignId();
    const characterId = this.host.characterId();
    if (!campaignId || !characterId || this.busyKey() !== null) {
      return;
    }
    this.busyKey.set(change.key);
    this.error.set('');
    this.notice.set('');
    this.marksNotice.set('');
    this.readFailed.set(false);
    try {
      await this.host.source.setEquipped(campaignId, characterId, change.key, change.equipped);
      const fresh = await this.host.reload();
      if (characterId !== this.host.characterId()) {
        return;
      }
      if (!fresh) {
        this.readFailed.set(true);
        this.error.set('A troca foi feita, mas não deu para ler a ficha de novo.');
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
