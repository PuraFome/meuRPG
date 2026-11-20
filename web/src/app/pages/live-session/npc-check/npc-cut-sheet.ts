import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import type { Observable } from 'rxjs';

import type { CuttingFlow } from '../../../core/npc-check/cutting-flow';
import { cutLabel, cuttingView, typedFace } from '../../../core/npc-check/cutting-question';
import { PairFoot } from '../../../shared/pair-foot/pair-foot';
import { TextField } from '../../../shared/form-fields/text-field';
import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet, openSheet } from '../../../shared/sheet/sheet-host';

/** What the page hands the bard's question. */
export interface NpcCutData {
  readonly flow: CuttingFlow;
  /** The bard rolls the d8 on a real die and types the face (the table's dice mode and the player's own choice). */
  readonly typed: boolean;
}

const SHEET_WIDTH = '480px';
const MAX_FACE_LENGTH = 3;

/** The bard's question: an alert that must be answered (Esc and the backdrop do not close it). It closes with `true` when the bard let it pass. */
export function openNpcCut(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: NpcCutData,
): Observable<boolean | undefined> {
  return openSheet<NpcCutSheet, NpcCutData, boolean>(dialog, bottomSheet, NpcCutSheet, {
    data,
    ariaLabel: 'Palavras de Interrupção',
    labelledBy: 'npc-cut-t',
    width: SHEET_WIDTH,
    alert: true,
  });
}

/**
 * The bard's side of "Teste de um NPC" (591-03 to 591-05, 591-08 state 7): "Usar Palavras de Interrupção?" with what it costs and
 * what the bard does not read (which check, the total and the DC stay with the master), "Deixar passar" (the first focus: nothing
 * is spent unless the bard cuts) and "Cortar", with the real die's face typed first when the bard rolls it by hand. It reads the
 * bard's own die and the uses left after a cut, and says so when the master went on without the answer. The question has no clock
 * and never reads "Sem efeito".
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-npc-cut-sheet',
  imports: [MatButtonModule, MatIconModule, PairFoot, SheetFrame, TextField],
  templateUrl: './npc-cut-sheet.html',
  styleUrl: './npc-cut-sheet.scss',
})
export class NpcCutSheet {
  private readonly sheet = injectSheet<NpcCutData, boolean>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  protected readonly flow = this.data.flow;
  protected readonly maxLength = MAX_FACE_LENGTH;
  protected readonly text = signal('');
  protected readonly view = computed(() => {
    const offer = this.flow.offer();
    return offer ? cuttingView(offer) : null;
  });
  protected readonly face = computed(() => {
    const v = this.view();
    return this.data.typed && v ? typedFace(this.text(), v.sides) : null;
  });
  protected readonly invalid = computed(
    () => this.data.typed && this.text().trim() !== '' && this.face() === null,
  );
  /** The button reads "Cortar", then "Cortar com 4" once the face is typed, and "Cortando…" while the answer is on the way. */
  protected readonly cutText = computed(() =>
    this.flow.busy() ? 'Cortando…' : cutLabel(this.face()),
  );
  protected readonly cutReady = computed(
    () => !this.flow.busy() && (!this.data.typed || this.face() !== null),
  );
  protected readonly lockWhileBusy = effect(() => this.sheet.lock(this.flow.busy()));
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);
  private phase = this.flow.phase();
  /** The phase changed (the result, the master going on, the answer refused): the button that was pressed may be gone, so the new primary button takes the focus. */
  protected readonly focusOnPhase = effect(() => {
    const phase = this.flow.phase();
    if (phase === this.phase) {
      return;
    }
    this.phase = phase;
    untracked(() =>
      afterNextRender(
        () =>
          this.host.nativeElement
            .querySelector<HTMLElement>('[data-initial-focus]:not([disabled])')
            ?.focus(),
        { injector: this.injector },
      ),
    );
  });

  protected async cut(): Promise<void> {
    await this.flow.answer(true, this.data.typed ? this.face() : null);
  }

  /** Letting it pass closes the sheet: the page says so for a moment. */
  protected async pass(): Promise<void> {
    await this.flow.answer(false, null);
    if (this.flow.phase() === 'passed') {
      this.sheet.close(true);
    }
  }

  protected close(): void {
    this.sheet.close(false);
  }
}
