import { Component, computed, input } from '@angular/core';

import {
  type MapSizeCm,
  type PaperId,
  type PaperPlans,
  type Plan,
  cm1,
  sheetCount,
  tightCm,
} from '../print-math';

interface Cell {
  readonly sum: string;
  readonly total: string;
  readonly size: string;
  readonly best: boolean;
}

interface Row {
  readonly id: PaperId;
  readonly name: string;
  readonly chosen: boolean;
  readonly landscape: Cell;
  readonly portrait: Cell;
}

/**
 * "As contas" (E8-12): one row per paper, both orientations, with the
 * sheets each needs and "A menor" on the one the app uses. The numbers are
 * the whole argument, so the table shows them as written in the formula.
 */
@Component({
  selector: 'app-print-table',
  templateUrl: './print-table.html',
  styleUrl: './print-table.scss',
})
export class PrintTable {
  readonly plans = input.required<readonly PaperPlans[]>();
  readonly paperId = input.required<PaperId>();
  readonly mapSize = input.required<MapSizeCm>();

  protected readonly intro = computed(
    () =>
      tightCm(
        `O mapa tem ${cm1(this.mapSize().width)} × ${cm1(this.mapSize().height)} cm. Folhas por lado = o teto de (tamanho do mapa − 1 cm) ÷ (área útil − 1 cm). Área útil = folha − 2 cm de margem.`,
      ),
  );

  protected readonly rows = computed<Row[]>(() =>
    this.plans().map((p) => ({
      id: p.paper.id,
      name: p.paper.name,
      chosen: p.paper.id === this.paperId(),
      landscape: cell(p.landscape, p.best),
      portrait: cell(p.portrait, p.best),
    })),
  );
}

function cell(plan: Plan, best: Plan): Cell {
  return {
    sum: tightCm(`${plan.columns} × ${plan.rows}`) + '\u00a0=',
    total: tightCm(sheetCount(plan.sheets)),
    size: tightCm(`(${cm1(plan.usableW)} × ${cm1(plan.usableH)} cm)`),
    best: plan === best,
  };
}
