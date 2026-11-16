import { ChangeDetectionStrategy, Component, effect, inject, signal } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import type { Observable } from 'rxjs';

import type { CreatureSummary } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { ActionKey } from '../../../core/connect/idempotency';
import { NpcCheckClient } from '../../../core/npc-check/npc-check-client';
import { type RollError, PICKER_ERROR_TEXT, rollErrorOf } from '../../../core/npc-check/npc-check-errors';
import { NpcCheckForm } from '../../../core/npc-check/npc-check-form';
import { NpcCheckLauncher } from '../../../core/npc-check/npc-check-launcher';
import { rowId } from '../../../core/npc-check/npc-check-picker';
import type { NpcCheckState } from '../../../core/npc-check/npc-check-state';
import { PairFoot } from '../../../shared/pair-foot/pair-foot';
import { Segmented, type Segment } from '../../../shared/segmented/segmented';
import { SelectField, type SelectOption } from '../../../shared/form-fields/select-field';
import { CheckRow } from '../../../shared/form-fields/check-row';
import { TextField } from '../../../shared/form-fields/text-field';
import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet, openSheet } from '../../../shared/sheet/sheet-host';
import { NpcCheckWho } from './npc-check-who';

/** What the page hands the sheet "Teste de um NPC". */
export interface NpcCheckSheetData {
  readonly campaignId: string;
  /** The NPC to choose first (`''`: none). */
  readonly firstId: string;
  /** Where the settled check goes: the master's card. */
  readonly state: NpcCheckState;
}

const SUB_PHONE = 'Fora de combate · a rolagem é só sua';
const SUB_WIDE = `${SUB_PHONE} · um bardo do Conhecimento pode cortar`;
const SHEET_WIDTH = '820px';
const MAX_D20_LENGTH = 3;

const MODES: readonly Segment<'app' | 'type'>[] = [
  { value: 'app', label: 'Rolar no app' },
  { value: 'type', label: 'Digitar o d20' },
];

/** "Teste de um NPC": a bottom sheet on a phone, a dialog from a tablet up. It closes with `true` when the check was rolled. */
export function openNpcCheckSheet(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: NpcCheckSheetData,
): Observable<boolean | undefined> {
  return openSheet<NpcCheckSheet, NpcCheckSheetData, boolean>(dialog, bottomSheet, NpcCheckSheet, {
    data,
    ariaLabel: 'Teste de um NPC',
    labelledBy: 'npc-check-t',
    width: SHEET_WIDTH,
    tall: true,
  });
}

/**
 * "Teste de um NPC" (591-01, 591-07): who rolls (the stage, the map, the bestiary), which check with its bonus, how the d20
 * comes, and the box "Os bardos veem a criatura e ela os ouve" when the map cannot measure the creature. The sheet names the
 * roll in its button, and a button that cannot act says why in the line above it. Nothing is rolled by a refusal: the notice says
 * so and the form stays.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-npc-check-sheet',
  imports: [
    CheckRow,
    MatButtonModule,
    MatIconModule,
    MatProgressSpinnerModule,
    NpcCheckWho,
    PairFoot,
    Segmented,
    SelectField,
    SheetFrame,
    TextField,
  ],
  templateUrl: './npc-check-sheet.html',
  styleUrl: './npc-check-sheet.scss',
})
export class NpcCheckSheet {
  private readonly client = inject(NpcCheckClient);
  private readonly launcher = inject(NpcCheckLauncher);
  private readonly sheet = injectSheet<NpcCheckSheetData, boolean>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  protected readonly subtitle = this.inSheet ? SUB_PHONE : SUB_WIDE;
  protected readonly modes = MODES;
  protected readonly maxLength = MAX_D20_LENGTH;
  protected readonly form = new NpcCheckForm();
  protected readonly loading = signal(true);
  protected readonly loadFailed = signal(false);
  protected readonly busy = signal(false);
  protected readonly error = signal<RollError | null>(null);
  protected readonly pickerError = PICKER_ERROR_TEXT;
  private readonly keys = new ActionKey();
  protected readonly lockWhileBusy = effect(() => this.sheet.lock(this.busy()));

  constructor() {
    void this.read();
  }

  protected checkOptions(): SelectOption[] {
    return this.form.options().map((o) => ({ value: o.key, label: o.name, side: o.side }));
  }

  protected async read(): Promise<void> {
    this.loading.set(true);
    this.loadFailed.set(false);
    try {
      this.form.setCandidates(await this.client.picker(this.data.campaignId));
      if (this.data.firstId) {
        this.form.select(this.data.firstId);
      }
    } catch {
      this.loadFailed.set(true);
    } finally {
      this.loading.set(false);
    }
  }

  /** The bestiary's creature the master searched: the server reads its checks, and the group keeps it. */
  protected async found(c: CreatureSummary): Promise<void> {
    try {
      const all = await this.client.picker(this.data.campaignId, c.key);
      const own = all.find((o) => o.monsterKey === c.key && rowId(o) === c.key);
      if (own) {
        this.form.addFound(own);
      }
    } catch {
      this.error.set({ lead: PICKER_ERROR_TEXT, text: '', combat: false });
    }
  }

  protected async roll(): Promise<void> {
    const roll = this.form.roll();
    const who = this.form.selected();
    if (!roll || !who || this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set(null);
    try {
      const read = await this.client.roll(this.data.campaignId, roll, this.keys.keyFor(roll));
      this.keys.renew();
      this.launcher.combatOpen.set(false);
      this.data.state.apply(read);
      this.sheet.close(true);
    } catch (err) {
      const failure = rollErrorOf(err, who.name);
      this.launcher.combatOpen.set(failure.combat);
      this.error.set(failure);
    } finally {
      this.busy.set(false);
    }
  }

  protected cancel(): void {
    if (!this.busy()) {
      this.sheet.close(false);
    }
  }
}
