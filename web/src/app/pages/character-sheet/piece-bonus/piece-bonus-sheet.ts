import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { BonusGroup } from '../../../shared/bonus-group/bonus-group';
import { saveFlow } from '../../../shared/sheet/save-flow';
import { SheetNote } from '../../../shared/sheet/sheet-note/sheet-note';
import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { SheetSaveFoot } from '../../../shared/sheet/sheet-save-foot/sheet-save-foot';
import { injectSheet, openSheet } from '../../../shared/sheet/sheet-host';

/** What the sheet of an armor's or a shield's magic bonus needs. */
export interface PieceBonusData {
  readonly pieceName: string;
  readonly kind: 'armor' | 'shield';
  readonly characterName: string;
  /** The master edits at any time and says so; the owner is told when the sheet locks. */
  readonly isMaster: boolean;
  /** The magic bonus stored for the piece, 0 to 3. */
  readonly bonus: number;
  /** Why the bonus does not count now; empty when it does. */
  readonly reason?: string;
  /** Saves the bonus; 0 removes it.
   * It answers the refusal in words.
   * It answers an empty text when the bonus is saved. */
  readonly save: (bonus: number) => Promise<string>;
}

/** Opens "Bônus · <peça>": a bottom sheet on a phone, a dialog from a tablet up. It answers `true` when the bonus was saved. */
export function openPieceBonus(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: PieceBonusData,
) {
  return openSheet<PieceBonusSheet, PieceBonusData, boolean>(dialog, bottomSheet, PieceBonusSheet, {
    data,
    ariaLabel: `Bônus · ${data.pieceName}`,
    labelledBy: 'sheet-t',
    width: '480px',
  });
}

/**
 * "Bônus · Escudo" (from the armor's or the shield's "⋮"): what the bonus adds to, the magic bonus and "Salvar".
 * A refusal is said here and the sheet stays open, with the choice as it was.
 */
@Component({
  selector: 'app-piece-bonus-sheet',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule, BonusGroup, SheetFrame, SheetNote, SheetSaveFoot],
  templateUrl: './piece-bonus-sheet.html',
  styleUrl: './piece-bonus-sheet.scss',
})
export class PieceBonusSheet {
  private readonly sheet = injectSheet<PieceBonusData, boolean>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  protected readonly bonus = signal(this.data.bonus);
  private readonly flow = saveFlow(this.sheet, () => this.data.save(this.bonus()));
  protected readonly busy = this.flow.busy;
  protected readonly error = this.flow.error;

  /** What the bonus adds to, in a line under the title; the master's line starts with the character. */
  protected description(): string {
    const what =
      this.data.kind === 'armor'
        ? 'soma na CA enquanto a armadura está equipada.'
        : 'soma na CA com o escudo em uso.';
    return this.data.isMaster
      ? `${this.data.characterName} · ${what}`
      : what.charAt(0).toUpperCase() + what.slice(1);
  }

  protected lock(): string {
    return this.data.isMaster
      ? `Muda a ficha de ${this.data.characterName}: depois da trava, só você muda o bônus. ${this.data.characterName} o lê na ficha.`
      : 'O dono só muda enquanto a ficha não trava. Depois, só o mestre.';
  }

  protected save(): Promise<void> {
    return this.flow.save();
  }

  protected cancel(): void {
    this.flow.cancel();
  }
}
