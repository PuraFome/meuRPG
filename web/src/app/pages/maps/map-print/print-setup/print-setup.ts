import { Component, computed, input, model, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatRadioModule } from '@angular/material/radio';
import { RouterLink } from '@angular/router';

import { joinDots } from '../../../../core/format/text';
import {
  DEFAULT_SQUARE_CM,
  MANY_SHEETS,
  MAX_SQUARE_CM,
  MIN_SQUARE_CM,
  PAPERS,
  type MapSizeCm,
  type PaperId,
  type PaperPlans,
  cm1,
  cmShort,
  sheetCount,
  tightCm,
} from '../print-math';

const DEFAULT_TEXT = String(DEFAULT_SQUARE_CM).replace('.', ',');

/**
 * "Papel e escala" (E8-12): the square's size, the paper, what comes out on
 * each sheet, the summary of the result and "Imprimir". The page owns the
 * numbers; this keeps the field and the cards. The field shows the value as
 * typed ("5", never "5,00"); anything outside 1 to 10 cm gets the error box
 * and a dashed "Imprimir" that does nothing.
 */
@Component({
  selector: 'app-print-setup',
  imports: [MatButtonModule, MatIconModule, MatRadioModule, RouterLink],
  templateUrl: './print-setup.html',
  styleUrl: './print-setup.scss',
})
export class PrintSetup {
  /** The field's text. */
  readonly typed = model.required<string>();
  readonly paperId = model.required<PaperId>();
  /** The parsed size, or `null` while the field is not 1 to 10 cm. */
  readonly squareCm = input.required<number | null>();
  readonly mapSize = input.required<MapSizeCm | null>();
  readonly chosen = input.required<PaperPlans | null>();
  readonly leanest = input.required<PaperPlans | null>();
  readonly gridColumns = input.required<number>();
  readonly gridRows = input.required<number>();
  readonly backLink = input.required<string[]>();
  readonly printMap = output<void>();

  protected readonly papers = PAPERS.map((p) => ({
    ...p,
    size: `${cmShort(p.shortCm)} × ${cmShort(p.longCm)} cm`,
  }));
  protected readonly min = MIN_SQUARE_CM;
  protected readonly max = MAX_SQUARE_CM;
  protected readonly changed = computed(() => this.typed().trim() !== DEFAULT_TEXT);
  protected readonly invalid = computed(() => this.squareCm() === null);

  /** "76,2 × 50,8 cm" */
  protected readonly mapText = computed(() => {
    const size = this.mapSize();
    return size ? tightCm(`${cm1(size.width)} × ${cm1(size.height)} cm`) : null;
  });
  protected readonly orientationText = computed(() => {
    const c = this.chosen();
    if (!c) {
      return null;
    }
    return {
      name: c.best.orientation === 'landscape' ? 'Paisagem' : 'Retrato',
      detail: tightCm(`Paisagem usa ${c.landscape.sheets} folhas e retrato usa ${c.portrait.sheets}: o app escolhe a que gasta menos.`),
    };
  });
  protected readonly sheetsText = computed(() => {
    const c = this.chosen();
    return c ? tightCm(`em ${sheetCount(c.best.sheets)}`) + ` ${c.paper.name}` : null;
  });
  protected readonly gridText = computed(() => {
    const p = this.chosen()?.best;
    if (!p) {
      return null;
    }
    const cols = `${p.columns} ${p.columns === 1 ? 'coluna' : 'colunas'}`;
    const rows = `${p.rows} ${p.rows === 1 ? 'linha' : 'linhas'}`;
    // The pieces never split ("1 cm", "3 linhas") and a dot never starts a line.
    return joinDots([tightCm(`${cols} × ${rows}`), 'margem de 1\u00a0cm', '1\u00a0cm de sobreposição']);
  });
  /** The amber notice over 16 sheets: names the paper that spends fewer. */
  protected readonly manySheets = computed(() => {
    const c = this.chosen();
    const lean = this.leanest();
    if (!c || !lean || c.best.sheets <= MANY_SHEETS) {
      return null;
    }
    const total = tightCm(`São ${c.best.sheets} folhas.`);
    if (lean.best.sheets >= c.best.sheets) {
      return { total, rest: 'Um quadrado menor gasta menos folhas.' };
    }
    const b = lean.best;
    const way = b.orientation === 'landscape' ? 'paisagem' : 'retrato';
    return {
      total,
      rest: tightCm(`${lean.paper.inName} o mesmo mapa gasta ${b.sheets} ${b.sheets === 1 ? 'folha' : 'folhas'} (${way}, ${b.columns} × ${b.rows}); um quadrado menor também gasta menos.`),
    };
  });

  protected onType(event: Event): void {
    this.typed.set((event.target as HTMLInputElement).value);
  }

  protected resetSquare(): void {
    this.typed.set(DEFAULT_TEXT);
  }
}
