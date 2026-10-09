import { ChangeDetectionStrategy, Component } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet, openSheet } from '../../../shared/sheet/sheet-host';

/** What the danger confirmation says: the question, what it costs, and the two buttons. */
export interface CastConfirmData {
  /** "Isso encerra Bênção". */
  readonly title: string;
  /** The line under it ("Conjurar Escudo da Fé encerra a concentração em Bênção, e os efeitos dela terminam."). */
  readonly body: string;
  /** The danger button ("Encerrar Bênção e conjurar"). */
  readonly confirm: string;
  /** The safe button; the focus starts on it ("Cancelar"). */
  readonly cancel: string;
}

/** Opens the confirmation: it answers `true` only when the person confirms; the safe button has the focus. */
export function openCastConfirm(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: CastConfirmData,
) {
  return openSheet<CastConfirmSheet, CastConfirmData, boolean>(
    dialog,
    bottomSheet,
    CastConfirmSheet,
    {
      data,
      ariaLabel: data.title,
      labelledBy: 'cast-confirm-t',
      width: '440px',
      alert: true,
    },
  );
}

/**
 * The danger confirmation of the casts ("Isso encerra Bênção", "Encerrar a Bênção de Ilaria?"): an alert dialog that
 * Esc and the backdrop do not dismiss, with the danger outline on the button that does the thing and the focus on
 * "Cancelar". Nothing is sent from here: the page that opened it does the work when it answers `true`.
 */
@Component({
  selector: 'app-cast-confirm-sheet',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule, SheetFrame],
  template: `
    <app-sheet-frame [title]="data.title" titleId="cast-confirm-t" [phone]="inSheet" [closable]="false" [focusableBody]="true">
      <div class="confirm">
        <mat-icon class="confirm__icon" aria-hidden="true">warning</mat-icon>
        <p class="confirm__body">{{ data.body }}</p>
      </div>
      <div foot class="confirm__actions">
        <button mat-stroked-button type="button" data-initial-focus (click)="sheet.close(false)">{{ data.cancel }}</button>
        <button mat-stroked-button type="button" class="confirm__danger" (click)="sheet.close(true)">{{ data.confirm }}</button>
      </div>
    </app-sheet-frame>
  `,
  styles: `
    .confirm {
      display: flex;
      gap: 12px;
      padding: 8px 16px 16px;
    }
    .confirm__icon {
      flex: none;
      color: var(--mr-danger-ink);
    }
    .confirm__body {
      margin: 0;
      font-size: 16px;
      line-height: 24px;
    }
    .confirm__actions {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
      padding: 12px 16px 16px;
    }
    .confirm__actions button {
      flex: 1 1 140px;
      min-height: 48px;
    }
    .confirm__danger {
      --mat-button-outlined-outline-color: var(--mr-danger-ink);
      --mat-button-outlined-label-text-color: var(--mr-danger-ink);
      border: 2px solid var(--mr-danger-ink);
    }
  `,
})
export class CastConfirmSheet {
  protected readonly sheet = injectSheet<CastConfirmData, boolean>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
}
