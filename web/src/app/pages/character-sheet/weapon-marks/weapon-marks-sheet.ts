import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import type { MarksVm } from '../../../core/characters/gear';
import { BonusGroup } from '../../../shared/bonus-group/bonus-group';
import { saveFlow } from '../../../shared/sheet/save-flow';
import { SheetSaveFoot } from '../../../shared/sheet/sheet-save-foot/sheet-save-foot';
import { SheetNote } from '../../../shared/sheet/sheet-note/sheet-note';
import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet, openSheet } from '../../../shared/sheet/sheet-host';

/** What the marks and bonus changed on a weapon, saved together. */
export interface WeaponChange {
  readonly marks: MarksVm;
  /** 0 is no bonus. */
  readonly bonus: number;
}

/** What the sheet of a weapon's marks and bonus needs. */
export interface WeaponMarksData {
  readonly weaponName: string;
  readonly characterName: string;
  /** The master edits at any time and says so; the owner is told when the sheet locks. */
  readonly isMaster: boolean;
  readonly marks: MarksVm;
  /** The magic bonus stored for the weapon, 0 to 3. */
  readonly bonus: number;
  /** How many copies of the weapon the sheet lists; they share marks and bonus. */
  readonly copies: number;
  /** Why the bonus does not count now; empty when it does. */
  readonly reason?: string;
  /** Saves the marks and the bonus.
   * It answers the refusal in words.
   * It answers an empty text when they are saved. */
  readonly save: (change: WeaponChange) => Promise<string>;
}

type MarkName = keyof MarksVm;

/** The three marks: the icon, the word and what each one beats (SRD 5.1: resistance and immunity to nonmagical attacks,
 * "silvered" and "adamantine" weapons; a magical weapon beats all three). */
const MARKS: readonly {
  readonly name: MarkName;
  readonly word: string;
  readonly icon: string;
  readonly beats: string;
}[] = [
  { name: 'silvered', word: 'Prateada', icon: 'dark_mode', beats: 'vence “nem prateado”' },
  { name: 'adamantine', word: 'Adamantina', icon: 'diamond', beats: 'vence “nem adamantino”' },
  {
    name: 'magical',
    word: 'Mágica',
    icon: 'auto_awesome',
    beats: 'vence os três: “não mágico”, “nem prateado”, “nem adamantino”',
  },
];

/** Opens "Marcas · <arma>": a bottom sheet on a phone, a dialog from a tablet up. It answers `true` when they were saved. */
export function openWeaponMarks(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: WeaponMarksData,
) {
  return openSheet<WeaponMarksSheet, WeaponMarksData, boolean>(
    dialog,
    bottomSheet,
    WeaponMarksSheet,
    {
      data,
      ariaLabel: `Marcas · ${data.weaponName}`,
      labelledBy: 'sheet-t',
      width: '480px',
    },
  );
}

/**
 * "Marcas · Espada curta" (from the weapon's "⋮"): "A arma é:", one row for each mark, the magic bonus and one "Salvar"
 * that saves marks and bonus together over the whole sheet (the page does it, at the revision it holds). A refusal is
 * said here and the sheet stays open, with the choices as they were. With several copies a line says they share them.
 */
@Component({
  selector: 'app-weapon-marks-sheet',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule, BonusGroup, SheetFrame, SheetNote, SheetSaveFoot],
  templateUrl: './weapon-marks-sheet.html',
  styleUrl: './weapon-marks-sheet.scss',
})
export class WeaponMarksSheet {
  private readonly sheet = injectSheet<WeaponMarksData, boolean>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  protected readonly options = MARKS;
  protected readonly draft = signal<MarksVm>(this.data.marks);
  protected readonly bonus = signal(this.data.bonus);
  private readonly flow = saveFlow(this.sheet, () =>
    this.data.save({ marks: this.draft(), bonus: this.bonus() }),
  );
  protected readonly busy = this.flow.busy;
  protected readonly error = this.flow.error;

  protected lock(): string {
    return this.data.isMaster
      ? `Muda a ficha de ${this.data.characterName}: depois da trava, só você muda marcas e bônus. ${this.data.characterName} os lê na ficha e nos ataques.`
      : 'O dono só muda enquanto a ficha não trava. Depois, só o mestre.';
  }

  protected set(name: MarkName, on: boolean): void {
    this.draft.update((d) => ({ ...d, [name]: on }));
  }

  protected save(): Promise<void> {
    return this.flow.save();
  }

  protected cancel(): void {
    this.flow.cancel();
  }
}
