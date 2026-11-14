import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet, openSheet } from '../../../shared/sheet/sheet-host';

export interface RemoveMagicItemData {
  readonly itemName: string;
  /** The item is attuned: the sentence says the attunement ends with it, and the count goes down. */
  readonly attunedBefore: number | null;
  /** `remove` answers the refusal in words, or '' when the item is gone. */
  readonly remove: () => Promise<string>;
}

/** Opens "Remover <item>?": it answers `true` when the item was removed. */
export function openRemoveMagicItem(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: RemoveMagicItemData,
) {
  return openSheet<RemoveMagicItemSheet, RemoveMagicItemData, boolean>(
    dialog,
    bottomSheet,
    RemoveMagicItemSheet,
    {
      data,
      ariaLabel: `Remover ${data.itemName}?`,
      labelledBy: 'sheet-t',
      width: '480px',
      alert: true,
    },
  );
}

/** "Remover Anel de ação livre?": says what ends. The removal is a full-sheet save at the held revision. */
@Component({
  selector: 'app-remove-magic-item-sheet',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule, SheetFrame],
  template: `
    <app-sheet-frame [title]="'Remover ' + data.itemName + '?'" titleId="sheet-t" [phone]="inSheet" (closed)="cancel()">
      <p>
        @if (data.attunedBefore !== null) {
          O item sai da ficha, e a sintonização acaba com ele: Sintonizados {{ data.attunedBefore }} → {{ data.attunedBefore - 1 }}.
        } @else {
          O item sai da ficha.
        }
      </p>
      @if (error()) {
        <p class="mr-notice mr-notice--danger" role="alert">
          <mat-icon aria-hidden="true">error</mat-icon><span>{{ error() }}</span>
        </p>
      }
      <div foot class="foot">
        <button matButton="outlined" type="button" class="danger" [disabled]="busy()" disabledInteractive (click)="remove()">
          <mat-icon aria-hidden="true">delete</mat-icon>Remover
        </button>
        <button matButton="outlined" type="button" data-initial-focus [disabled]="busy()" (click)="cancel()">Cancelar</button>
      </div>
    </app-sheet-frame>
  `,
  styles: `
    .foot {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: var(--mr-space-3);
    }
    .danger {
      color: var(--mr-danger-ink, inherit);
      border-color: var(--mr-danger-ink, currentColor);
    }
  `,
})
export class RemoveMagicItemSheet {
  private readonly sheet = injectSheet<RemoveMagicItemData, boolean>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  protected readonly busy = signal(false);
  protected readonly error = signal('');

  protected async remove(): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const why = await this.data.remove();
      if (why) {
        this.error.set(why);
      } else {
        this.sheet.close(true);
      }
    } finally {
      this.busy.set(false);
    }
  }

  protected cancel(): void {
    if (!this.busy()) {
      this.sheet.close();
    }
  }
}
