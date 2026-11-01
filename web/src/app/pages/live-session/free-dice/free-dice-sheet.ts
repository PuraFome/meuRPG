import { ChangeDetectionStrategy, Component, type Signal, computed, effect } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import type { Observable } from 'rxjs';

import type { FreeDiceState } from '../../../core/free-dice/free-dice-state';
import {
  FREE_DICE,
  type FreeMode,
  type FreeRollVm,
  MAX_COUNT,
  MAX_MODIFIER,
  SHORTCUTS,
  type FreeShortcut,
  canAdvantage,
  repeatLabel,
  rollButtonLabel,
  rollButtonName,
  summaryOf,
  whenText,
} from '../../../core/free-dice/free-roll';
import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet, openSheet } from '../../../shared/sheet/sheet-host';
import { FreeChoice, type FreeChoiceOption } from './free-choice';
import { FreeRollCard } from './free-roll-card';
import { NumberField } from './number-field';

/** What the page hands the dice sheet. */
export interface FreeDiceSheetData {
  readonly state: FreeDiceState;
  readonly isMaster: boolean;
  /** False while the stream reconnects: there is no table to roll for. */
  readonly online: Signal<boolean>;
  /** The table plays the game's dice with physical dice (RN-18): the sheet says this roll is the app's. */
  readonly physical: boolean;
}

/** The free dice roller: a bottom sheet on a phone, a dialog from a tablet up. */
export function openFreeDice(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: FreeDiceSheetData,
): Observable<void | undefined> {
  return openSheet<FreeDiceSheet, FreeDiceSheetData, void>(dialog, bottomSheet, FreeDiceSheet, {
    data,
    ariaLabel: 'Rolar dados',
    labelledBy: 'free-dice-t',
    width: '440px',
    panelClass: 'mr-sheet-dice',
  });
}

const DICE_OPTIONS: readonly FreeChoiceOption[] = FREE_DICE.map((n) => ({
  value: String(n),
  label: `d${n}`,
}));
const MODE_OPTIONS: readonly FreeChoiceOption[] = [
  { value: 'normal', label: 'Normal' },
  { value: 'advantage', label: 'Vantagem' },
  { value: 'disadvantage', label: 'Desvantagem' },
];
const AUDIENCE_OPTIONS: readonly FreeChoiceOption[] = [
  { value: 'table', label: 'Para a mesa' },
  { value: 'me', label: 'Só eu' },
];

/**
 * "Rolar dados" (MR-052, B5-01 to B5-06): the dice, how many, the modifier, advantage for one d20, and for the master who
 * sees it; the big button rolls with the server's dice, the result card replaces the form ("Mudar dados" goes back,
 * "Rolar … de novo" rolls again) and the caller's last rolls stay below with "Repetir". Nothing here decides a rule: the
 * state asks and the server rolls. While the stream is down the button is off and says why; a refusal (the rate limit)
 * says the wait and leaves the form as it was.
 */
@Component({
  selector: 'app-free-dice-sheet',
  imports: [FreeChoice, FreeRollCard, MatButtonModule, MatIconModule, NumberField, SheetFrame],
  templateUrl: './free-dice-sheet.html',
  styleUrl: './free-dice-sheet.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class FreeDiceSheet {
  private readonly sheet = injectSheet<FreeDiceSheetData, void>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  protected readonly state = this.data.state;

  protected readonly shortcuts = SHORTCUTS;
  protected readonly diceOptions = DICE_OPTIONS;
  protected readonly modeOptions = MODE_OPTIONS;
  protected readonly audienceOptions = AUDIENCE_OPTIONS;
  protected readonly maxCount = MAX_COUNT;
  protected readonly maxModifier = MAX_MODIFIER;

  protected readonly form = this.state.form;
  private readonly context = computed(() => ({
    master: this.data.isMaster,
    physical: this.data.physical,
  }));
  protected readonly label = computed(() => rollButtonLabel(this.form(), this.context()));
  protected readonly name = computed(() => rollButtonName(this.form(), this.context()));
  protected readonly againLabel = computed(() =>
    rollButtonLabel(this.form(), this.context(), true),
  );
  protected readonly againName = computed(() => rollButtonName(this.form(), this.context(), true));
  protected readonly oneD20 = computed(() => canAdvantage(this.form()));
  protected readonly secret = computed(() => this.data.isMaster && this.form().secret);
  /** Nothing rolls while a roll is on its way or the stream is down. */
  protected readonly blocked = computed(() => this.state.rolling() || !this.data.online());
  protected readonly unknownDice = computed(() =>
    Array.from({ length: this.form().count }, (_, i) => i),
  );

  /**
   * Esc and the backdrop wait for the answer of a roll in the air.
   * The sheet shrinks to the rolling card on the first tap and the button moves away.
   * The second tap of a double tap would land on the backdrop and close the sheet.
   */
  protected readonly lockWhileRolling = effect(() => this.sheet.lock(this.state.rolling()));

  protected close(): void {
    this.sheet.close();
  }

  protected choose(sides: string): void {
    this.state.change({ sides: Number(sides) });
  }

  protected shortcut(s: FreeShortcut): void {
    this.state.shortcut(s);
  }

  protected setMode(mode: string): void {
    this.state.change({ mode: mode as FreeMode });
  }

  protected setAudience(who: string): void {
    this.state.change({ secret: who === 'me' });
  }

  protected roll(): void {
    if (!this.blocked()) {
      void this.state.roll();
    }
  }

  protected repeat(roll: FreeRollVm): void {
    if (!this.blocked()) {
      void this.state.repeat(roll);
    }
  }

  protected summary(roll: FreeRollVm): string {
    return summaryOf(roll);
  }

  protected when(roll: FreeRollVm): string {
    return whenText(roll.createdAt, this.state.now());
  }

  protected repeatText(roll: FreeRollVm): string {
    return repeatLabel(roll);
  }
}
