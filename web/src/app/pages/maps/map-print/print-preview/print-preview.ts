import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { MapImage } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import {
  MANY_SHEETS,
  OVERLAP_CM,
  TOO_MANY_LABELS,
  type MapSizeCm,
  type PaperPlans,
  cm1,
  gluedSize,
  sheetName,
  sheetOrigin,
  spareCm,
  tightCm,
} from '../print-math';

/** One label chip: where it sits as a share of the glued sheets. */
interface Chip {
  readonly name: string;
  readonly left: number;
  readonly top: number;
}

/**
 * "As folhas sobre o mapa" (E8-12): the sheets over the map, scaled to the
 * panel and out of scale (the legend says so). One SVG in centimetres holds
 * the image, the grid, the 1 cm bands shared by neighbours and the outline
 * of the whole; the labels A1, A2... are HTML chips over it, so their size
 * does not follow the drawing. Over 16 sheets the labels shrink; over 36
 * they go (a thousand chips would be noise).
 */
@Component({
  selector: 'app-print-preview',
  imports: [MatIconModule],
  templateUrl: './print-preview.html',
  styleUrl: './print-preview.scss',
})
export class PrintPreview {
  readonly image = input.required<MapImage | null>();
  readonly mapName = input.required<string>();
  readonly mapSize = input.required<MapSizeCm | null>();
  readonly squareCm = input.required<number | null>();
  readonly chosen = input.required<PaperPlans | null>();

  protected readonly overlap = OVERLAP_CM;

  protected readonly plan = computed(() => this.chosen()?.best ?? null);
  protected readonly glued = computed(() => {
    const p = this.plan();
    return p ? gluedSize(p) : null;
  });
  protected readonly viewBox = computed(() => {
    const g = this.glued();
    return g ? `0 0 ${g.width} ${g.height}` : '';
  });

  /** The bands two neighbours share, as [start, length] along each axis. */
  protected readonly vBands = computed(() => {
    const p = this.plan();
    return p
      ? Array.from({ length: p.columns - 1 }, (_, i) => (i + 1) * (p.usableW - OVERLAP_CM))
      : [];
  });
  protected readonly hBands = computed(() => {
    const p = this.plan();
    return p
      ? Array.from({ length: p.rows - 1 }, (_, i) => (i + 1) * (p.usableH - OVERLAP_CM))
      : [];
  });

  /** The grid over the map: a path of lines at every square. */
  protected readonly gridPath = computed(() => {
    const size = this.mapSize();
    const square = this.squareCm();
    if (!size || square === null) {
      return '';
    }
    const parts: string[] = [];
    for (let x = 0; x <= size.width + 1e-6; x += square) {
      parts.push(`M${round(x)} 0V${round(size.height)}`);
    }
    for (let y = 0; y <= size.height + 1e-6; y += square) {
      parts.push(`M0 ${round(y)}H${round(size.width)}`);
    }
    return parts.join('');
  });

  protected readonly showLabels = computed(() => {
    const p = this.plan();
    return !!p && p.sheets <= TOO_MANY_LABELS;
  });
  protected readonly smallLabels = computed(() => (this.plan()?.sheets ?? 0) > MANY_SHEETS);
  protected readonly chips = computed<Chip[]>(() => {
    const p = this.plan();
    const g = this.glued();
    if (!p || !g || !this.showLabels()) {
      return [];
    }
    const chips: Chip[] = [];
    for (let r = 0; r < p.rows; r++) {
      for (let c = 0; c < p.columns; c++) {
        const o = sheetOrigin(p, r, c);
        chips.push({
          name: sheetName(r, c),
          left: (o.x / g.width) * 100,
          top: (o.y / g.height) * 100,
        });
      }
    }
    return chips;
  });

  protected readonly summary = computed(() => {
    const c = this.chosen();
    if (!c) {
      return '';
    }
    const p = c.best;
    const last = `${sheetName(p.rows - 1, p.columns - 1)}`;
    const way = p.orientation === 'landscape' ? 'paisagem' : 'retrato';
    return p.sheets === 1
      ? `Folha A1 · 1 no total · ${way}`
      : `Folhas A1 a ${last} · ${p.sheets} no total · ${way}`;
  });
  protected readonly usable = computed(() => {
    const p = this.plan();
    return p ? tightCm(`${cm1(p.usableW)} × ${cm1(p.usableH)} cm de área útil`) : '';
  });
  protected readonly spare = computed(() => {
    const p = this.plan();
    const size = this.mapSize();
    if (!p || !size) {
      return '';
    }
    const s = spareCm(p, size);
    const parts: string[] = [];
    if (s.right > 0) {
      parts.push(`${cm1(s.right)} cm à direita`);
    }
    if (s.bottom > 0) {
      parts.push(`${cm1(s.bottom)} cm embaixo`);
    }
    return parts.length > 0
      ? tightCm(`Sobra de papel em branco: ${parts.join(' e ')}`)
      : 'O mapa cobre as folhas inteiras, sem sobra';
  });
  protected readonly description = computed(() => {
    const p = this.plan();
    return p
      ? `Prévia, fora de escala, de ${p.sheets} ${p.sheets === 1 ? 'folha' : 'folhas'} sobre o mapa ${this.mapName()}`
      : '';
  });
}

function round(n: number): number {
  return Math.round(n * 1000) / 1000;
}
