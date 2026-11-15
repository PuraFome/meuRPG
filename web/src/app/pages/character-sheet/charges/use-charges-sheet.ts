import { ChangeDetectionStrategy, Component, effect, signal } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { ActionKey } from '../../../core/connect/idempotency';
import { CountPips } from '../../../shared/count-pips/count-pips';
import { CountStepper } from '../../../shared/count-stepper/count-stepper';
import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet, openSheet } from '../../../shared/sheet/sheet-host';

export interface UseChargesData {
  readonly itemName: string;
  readonly max: number;
  /** What the item holds now: the most that can be spent. */
  readonly left: number;
  /** Spends `amount` under `key`; answers the refusal in words, or '' when the charges are gone. */
  readonly spend: (amount: number, key: string) => Promise<string>;
}

/** Opens "Usar · <item>": a bottom sheet on a phone, a dialog from a tablet up. Answers `true` when charges were spent. */
export function openUseCharges(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: UseChargesData,
) {
  return openSheet<UseChargesSheet, UseChargesData, boolean>(dialog, bottomSheet, UseChargesSheet, {
    data,
    ariaLabel: `Usar ${data.itemName}`,
    labelledBy: 'use-charges-t',
    width: '480px',
    // The opener takes the focus back, with the focus ring.
    restoreFocus: false,
  });
}

/**
 * "Usar · Varinha de mísseis mágicos": how many charges the use costs (1 to what is left; what each use costs is in the
 * item's text) and "Gastar N cargas". The call has one idempotency key for the amount asked: the same amount again after a
 * failure is a retry and never spends twice. Works on a locked sheet: the sheet only reads and sends.
 */
@Component({
  selector: 'app-use-charges-sheet',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CountPips, CountStepper, MatButtonModule, MatIconModule, SheetFrame],
  template: `
    <app-sheet-frame
      [title]="'Usar · ' + data.itemName"
      titleId="use-charges-t"
      [phone]="inSheet"
      (closed)="close()"
    >
      <p class="left">
        @if (data.max <= 20) {
          <app-count-pips [left]="data.left" [total]="data.max" />
        }
        <span>{{ data.left === 1 ? 'Resta' : 'Restam' }} {{ data.left }} de {{ data.max }} {{ data.max === 1 ? 'carga' : 'cargas' }}</span>
      </p>
      @if (error()) {
        <p class="mr-notice mr-notice--danger" role="alert">
          <mat-icon aria-hidden="true">error</mat-icon><span>{{ error() }}</span>
        </p>
      }
      <div class="step">
        <span class="step__text">
          <b>Cargas a gastar</b>
          <span class="mr-muted">{{ data.left === 1 ? 'Só 1.' : 'De 1 a ' + data.left + '.' }}</span>
        </span>
        <app-count-stepper
          [value]="amount()"
          [min]="1"
          [max]="data.left"
          noun="carga"
          minusLabel="Menos uma carga"
          plusLabel="Mais uma carga"
          valueLabel="{n} a gastar"
          (valueChange)="amount.set($event)"
        />
      </div>
      <p class="mr-muted note">Quanto cada uso gasta está no texto do item.</p>
      <div foot class="foot">
        <button
          matButton="filled"
          type="button"
          class="go"
          [disabled]="busy()"
          disabledInteractive
          (click)="go()"
        >
          <mat-icon aria-hidden="true">bolt</mat-icon>{{ busy() ? 'Gastando...' : 'Gastar ' + amount() + (amount() === 1 ? ' carga' : ' cargas') }}
        </button>
        <button matButton="outlined" type="button" (click)="close()">Fechar</button>
      </div>
    </app-sheet-frame>
  `,
  styles: `
    .left {
      display: flex;
      align-items: center;
      gap: var(--mr-space-2);
      margin: 0 0 var(--mr-space-3);
    }
    .step {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: var(--mr-space-3);
    }
    .step__text {
      display: flex;
      flex-direction: column;
    }
    .note {
      margin: var(--mr-space-3) 0 0;
    }
    .foot {
      display: grid;
      gap: var(--mr-space-2);
    }
    .go {
      min-height: 48px;
    }
  `,
})
export class UseChargesSheet {
  private readonly sheet = injectSheet<UseChargesData, boolean>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  protected readonly amount = signal(1);
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  /** A request in the air: Esc and the backdrop do not close the sheet under it. */
  protected readonly lockWhileBusy = effect(() => this.sheet.lock(this.busy()));
  private readonly key = new ActionKey();

  protected async go(): Promise<void> {
    if (this.busy()) {
      return;
    }
    const amount = this.amount();
    this.busy.set(true);
    this.error.set('');
    try {
      const why = await this.data.spend(amount, this.key.keyFor({ amount }));
      if (why) {
        this.error.set(why);
      } else {
        this.key.renew();
        this.sheet.close(true);
      }
    } finally {
      this.busy.set(false);
    }
  }

  protected close(): void {
    if (!this.busy()) {
      this.sheet.close();
    }
  }
}
