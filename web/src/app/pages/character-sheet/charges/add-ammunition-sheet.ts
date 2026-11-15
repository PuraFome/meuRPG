import { ChangeDetectionStrategy, Component, computed, effect, signal } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { AMMUNITION_KINDS } from '../../../core/characters/charges';
import { CountStepper } from '../../../shared/count-stepper/count-stepper';
import { OptionCard } from '../../../shared/option-card/option-card';
import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet, openSheet } from '../../../shared/sheet/sheet-host';

/** The most pieces of one kind the server takes. */
const MOST = 9999;

export interface AddAmmunitionData {
  /** The kind to start on (the free-text hint's); arrows when absent. */
  readonly kind?: string;
  /** The count to start with (the hint's); the kind's bundle when absent. */
  readonly quantity?: number;
  /** What the character holds of each kind now, by key. */
  readonly current: Readonly<Record<string, number>>;
  /** Adds `amount` pieces to the kind (the page adds them to what the sheet holds when it sends: `SetAmmunition` takes a
   * total and has no delta); answers the refusal in words, or '' when they are added. */
  readonly add: (key: string, amount: number) => Promise<string>;
}

/** Opens "Adicionar munição". Answers `true` when a kind was added. */
export function openAddAmmunition(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: AddAmmunitionData,
) {
  return openSheet<AddAmmunitionSheet, AddAmmunitionData, boolean>(
    dialog,
    bottomSheet,
    AddAmmunitionSheet,
    { data, ariaLabel: 'Adicionar munição', labelledBy: 'add-ammo-t', width: '480px' },
  );
}

/**
 * "Adicionar munição": the four kinds of the SRD (arrows, bolts, sling bullets, blowgun needles) as radio cards, how many
 * come with them (the bundle of the kind, to change at will) and "Adicionar N flechas". A kind the character already holds
 * gets the new pieces on top of what it has: the page reads the sheet again and sends the total (the server's call has no delta).
 */
@Component({
  selector: 'app-add-ammunition-sheet',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CountStepper, MatButtonModule, MatIconModule, OptionCard, SheetFrame],
  template: `
    <app-sheet-frame title="Adicionar munição" titleId="add-ammo-t" [phone]="inSheet" (closed)="close()">
      @if (error()) {
        <p class="mr-notice mr-notice--danger" role="alert">
          <mat-icon aria-hidden="true">error</mat-icon><span>{{ error() }}</span>
        </p>
      }
      <div class="kinds" role="radiogroup" aria-label="Tipo de munição">
        @for (k of kinds; track k.key) {
          <app-option-card
            kind="radio"
            name="ammo-kind"
            [title]="k.namePt"
            [sub]="k.usedBy + ' · vem ' + k.bundle"
            [checked]="kind() === k.key"
            (chosen)="pick(k.key)"
          />
        }
      </div>
      <div class="step">
        <span class="step__text">
          <b>Quantas</b>
          <span class="mr-muted">Começa no pacote do SRD; mude à vontade.</span>
        </span>
        <app-count-stepper
          [value]="amount()"
          [min]="1"
          [max]="most()"
          noun="peça"
          minusLabel="Menos uma peça"
          plusLabel="Mais uma peça"
          valueLabel="{n} a adicionar"
          (valueChange)="amount.set($event)"
        />
      </div>
      @if (held() > 0) {
        <p class="mr-muted note">Já tem {{ held() }}: ficam {{ held() + amount() }}.</p>
      }
      <div foot class="foot">
        <button
          matButton="filled"
          type="button"
          class="go"
          [disabled]="busy()"
          disabledInteractive
          (click)="go()"
        >
          Adicionar {{ amount() }} {{ amount() === 1 ? singular() : name() }}
        </button>
        <button matButton="outlined" type="button" (click)="close()">Cancelar</button>
      </div>
    </app-sheet-frame>
  `,
  styles: `
    .kinds {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: var(--mr-space-2);
      margin-bottom: var(--mr-space-3);
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
      margin: var(--mr-space-2) 0 0;
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
export class AddAmmunitionSheet {
  private readonly sheet = injectSheet<AddAmmunitionData, boolean>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  protected readonly kinds = AMMUNITION_KINDS;
  protected readonly kind = signal(
    AMMUNITION_KINDS.some((k) => k.key === this.data.kind)
      ? (this.data.kind as string)
      : AMMUNITION_KINDS[0].key,
  );
  protected readonly amount = signal(
    Math.max(
      1,
      this.data.quantity ?? AMMUNITION_KINDS.find((k) => k.key === this.kind())?.bundle ?? 1,
    ),
  );
  protected readonly held = computed(() => this.data.current[this.kind()] ?? 0);
  protected readonly most = computed(() => Math.max(1, MOST - this.held()));
  private readonly chosen = computed(() => AMMUNITION_KINDS.find((k) => k.key === this.kind()));
  /** "flechas", "virotes", "balas de funda", "agulhas de zarabatana". */
  protected readonly name = computed(() => this.chosen()?.many ?? '');
  /** "flecha", "virote", "bala de funda", "agulha de zarabatana": the one piece. */
  protected readonly singular = computed(() => this.chosen()?.one ?? '');
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly lockWhileBusy = effect(() => this.sheet.lock(this.busy()));

  protected pick(key: string): void {
    this.kind.set(key);
    this.amount.set(AMMUNITION_KINDS.find((k) => k.key === key)?.bundle ?? 1);
    this.error.set('');
  }

  protected async go(): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const why = await this.data.add(this.kind(), this.amount());
      if (why) {
        this.error.set(why);
      } else {
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
