import { ChangeDetectionStrategy, Component, effect, inject, signal } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { ActionKey } from '../../../core/connect/idempotency';
import { parseSum } from '../../../core/combat/combat-dice';
import type { ItemRecharge } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { lastChargeBody } from '../../../core/characters/recharge-text';
import type { LastChargeRoll } from '../../../core/play/charges-client';
import { RollAnimator, showOfDie } from '../../../shared/roll-overlay/roll-animator';
import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet, openSheet } from '../../../shared/sheet/sheet-host';

/** The d20 of the last charge. */
const D20 = 20;

/** What one try of the d20 came to: done, a refusal in words, or the server asking for the face of the regain die. */
export type LastChargeOutcome =
  | { readonly kind: 'done'; readonly d20: number }
  | { readonly kind: 'error'; readonly text: string }
  | { readonly kind: 'regain'; readonly faces: number };

export interface LastChargeData {
  readonly itemName: string;
  readonly max: number;
  /** "Rolar no app" is allowed (the table did not force physical dice). */
  readonly canApp: boolean;
  /** Typing the face of a real die is allowed (the table did not force the app). */
  readonly canType: boolean;
  /** Where the player rolls once the table's mode and their choice are put together (RN-18). */
  readonly preferApp: boolean;
  /** The item's rules, which say what a 1 and a 20 do; the sheet words them. */
  readonly recharge?: ItemRecharge;
  readonly roll: (roll: LastChargeRoll, key: string) => Promise<LastChargeOutcome>;
}

/** Opens "A última carga se foi": an alert dialog, a bottom sheet on a phone. Answers `true` when the d20 was rolled. */
export function openLastCharge(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: LastChargeData,
) {
  return openSheet<LastChargeSheet, LastChargeData, boolean>(dialog, bottomSheet, LastChargeSheet, {
    data,
    ariaLabel: 'A última carga se foi',
    labelledBy: 'last-charge-t',
    width: '480px',
    // Esc and the backdrop do not close an alert dialog: the sheet answers Esc itself (it skips nothing, see below), and the
    // backdrop stays blocked so a stray tap does not.
    alert: true,
    // The page puts the focus back on the item's row (with the focus ring) once it has drawn it again.
    restoreFocus: false,
  });
}

/**
 * "A última carga se foi": the d20 an item with a rule for its last charge asks for. The table's dice follow RN-18 like
 * every roll of the app: "Rolar no app", or the face of a real die in "Face do d20" (which leads for a player who rolls
 * their own dice, with "Rolar no app" beside it). A typed 20 on an item that regains charges may ask for the face of the
 * die that counts them. Closing does not skip the die: the item's row asks for it ("Rolar") until it is rolled, so the close
 * button and Esc both close it (the backdrop does not). The call
 * has one idempotency key for the roll it carries: the same roll again after a failure is a retry.
 */
@Component({
  selector: 'app-last-charge-sheet',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule, SheetFrame],
  host: { '(keydown.escape)': 'close()' },
  template: `
    <app-sheet-frame
      title="A última carga se foi"
      titleId="last-charge-t"
      icon="casino"
      [phone]="inSheet"
      (closed)="close()"
    >
      <p class="sub mr-muted">{{ data.itemName }} · 0 de {{ data.max }}</p>
      <p class="body">
        @if (words(); as w) {
          {{ w.intro }}
          @if (w.strong) {
            <strong>{{ w.strong }}</strong>
          }{{ w.rest }}
        } @else {
          O item pede um d20 agora. Com 1, o item se perde como o texto dele diz; com 20, ele pode devolver
          cargas.
        }
      </p>
      @if (error()) {
        <p class="mr-notice mr-notice--danger" role="alert">
          <mat-icon aria-hidden="true">error</mat-icon><span>{{ error() }}</span>
        </p>
      }
      @if (typing()) {
        <div class="type">
          <label class="type__label" for="last-charge-face">{{ regainFaces() ? 'Face do d' + regainFaces() : 'Face do d20' }}</label>
          <div class="type__row">
            <input
              id="last-charge-face"
              class="type__field"
              type="text"
              inputmode="numeric"
              maxlength="2"
              autocomplete="off"
              data-initial-focus
              [value]="text()"
              [attr.aria-invalid]="invalid() ? 'true' : null"
              [attr.aria-describedby]="'last-charge-hint' + (invalid() ? ' last-charge-err' : '')"
              (input)="onType($event)"
              (keydown.enter)="confirm()"
            />
            <button
              matButton="filled"
              type="button"
              [disabled]="value() === null || busy()"
              disabledInteractive
              (click)="confirm()"
            >Confirmar</button>
          </div>
          <p id="last-charge-hint" class="type__hint mr-muted">
            {{ regainFaces() ? 'De 1 a ' + regainFaces() + '. O d20 foi 20: a face dá as cargas de volta.' : 'De 1 a 20.' }}
            @if (!data.canApp) {
              A mesa joga dados físicos: digite a face que saiu.
            }
          </p>
          @if (invalid()) {
            <p id="last-charge-err" class="type__err" role="alert">
              <mat-icon aria-hidden="true">error</mat-icon>Digite um número de 1 a {{ limit() }}
            </p>
          }
        </div>
        @if (data.canApp) {
          <button matButton type="button" class="app" [disabled]="busy()" disabledInteractive (click)="inApp()">
            <mat-icon aria-hidden="true">casino</mat-icon>Rolar no app
          </button>
        }
      } @else {
        <button
          matButton="filled"
          type="button"
          class="app app--main"
          data-initial-focus
          [disabled]="busy()"
          disabledInteractive
          (click)="inApp()"
        >
          <mat-icon aria-hidden="true">casino</mat-icon>Rolar no app
        </button>
        @if (data.canType) {
          <button matButton type="button" (click)="typing.set(true)">Digitar o resultado</button>
        }
      }
      <p class="note mr-muted">
        <mat-icon aria-hidden="true">info</mat-icon>Fechar não pula o dado: a linha do item pede o d20 até ele sair.
      </p>
    </app-sheet-frame>
  `,
  styles: `
    .sub {
      margin: 0 0 var(--mr-space-3);
    }
    .type__label {
      display: block;
      font-size: var(--mr-font-14);
    }
    .type__row {
      display: flex;
      gap: var(--mr-space-3);
      align-items: center;
    }
    .type__field {
      box-sizing: border-box;
      flex: 1;
      min-height: 56px;
      padding: 0 var(--mr-space-3);
      border: 2px solid var(--mr-accent);
      border-radius: var(--mr-radius-md);
      background: var(--mr-surface);
      color: var(--mr-ink);
      font: inherit;
      font-size: var(--mr-font-18);
    }
    .type__row button {
      min-height: 56px;
      min-width: 112px;
    }
    .type__hint {
      margin: var(--mr-space-1) 0 var(--mr-space-2);
      font-size: var(--mr-font-14);
    }
    .type__err {
      display: flex;
      align-items: center;
      gap: var(--mr-space-2);
      margin: 0 0 var(--mr-space-2);
      color: var(--mr-danger-ink);
    }
    .app {
      min-height: 44px;
    }
    .app--main {
      display: flex;
      width: 100%;
      min-height: 48px;
    }
    .note {
      display: flex;
      gap: var(--mr-space-2);
      margin: var(--mr-space-3) 0 0;
      font-size: var(--mr-font-14);
    }
  `,
})
export class LastChargeSheet {
  private readonly sheet = injectSheet<LastChargeData, boolean>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  /** The field is open: first for a player who rolls their own dice, or once "Digitar o resultado" was tapped. */
  protected readonly typing = signal(
    this.data.canType && (!this.data.canApp || !this.data.preferApp),
  );
  protected readonly text = signal('');
  /** The size of the die that counts the regained charges, once the server asks for it after a typed 20. */
  protected readonly regainFaces = signal(0);
  private readonly d20 = signal(0);
  protected readonly limit = () => (this.regainFaces() > 0 ? this.regainFaces() : D20);
  protected readonly value = () => parseSum(this.text(), 1, this.limit());
  protected readonly invalid = () => this.text().trim() !== '' && this.value() === null;
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  /** A request in the air: Esc and the backdrop do not close the sheet under it. */
  protected readonly lockWhileBusy = effect(() => this.sheet.lock(this.busy()));
  private readonly key = new ActionKey();
  private readonly animator = inject(RollAnimator);
  /** What the d20 decides for this item, from its rules; `null` when the page gave none. */
  protected readonly words = () =>
    this.data.recharge ? lastChargeBody(this.data.itemName, this.data.recharge) : null;

  protected onType(event: Event): void {
    this.text.set((event.target as HTMLInputElement).value);
  }

  protected inApp(): Promise<void> {
    return this.send({ inApp: true });
  }

  protected confirm(): Promise<void> {
    const v = this.value();
    if (v === null) {
      return Promise.resolve();
    }
    return this.regainFaces() > 0
      ? this.send({ face: this.d20(), regainFace: v })
      : this.send({ face: v });
  }

  private async send(roll: LastChargeRoll): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const out = await this.data.roll(roll, this.key.keyFor(roll));
      if (out.kind === 'done') {
        this.key.renew();
        if ('inApp' in roll) {
          const show = showOfDie('Última carga', D20, out.d20);
          if (show) {
            this.animator.play(show);
          }
        }
        this.sheet.close(true);
      } else if (out.kind === 'regain' && !('inApp' in roll)) {
        this.d20.set(roll.face);
        this.regainFaces.set(out.faces);
        this.text.set('');
      } else if (out.kind === 'error') {
        this.error.set(out.text);
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
