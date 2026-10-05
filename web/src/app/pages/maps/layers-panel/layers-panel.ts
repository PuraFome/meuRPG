import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { MapLayers } from '../../../core/maps/layers';
import { type PaintSaveStatus } from '../../../core/maps/paint-queue';
import { LIGHT_LABEL, type PaintTool, layerLines } from '../../../core/maps/paint-tools';
import { RevealSwitch } from '../reveal-switch/reveal-switch';

/** Which layers the master's map shows (a view setting of his drawing: it changes nothing for anyone). */
export type LayerVisibility = Readonly<Record<PaintTool, boolean>>;

/**
 * "Camadas" (E9-01 1): the four layers the master paints, each with how many squares it holds and a switch that
 * shows or hides it on his own map (his view only), the same drawings as the map's legend, and the "Tudo salvo"
 * tag that says whether the strokes reached the server ("Salvando…", or "Não salvou" with "Tentar de novo").
 * The light's three glyphs are named under its row. Presentational: the editor owns the layers and the queue.
 */
@Component({
  selector: 'app-layers-panel',
  imports: [MatButtonModule, MatIconModule, RevealSwitch],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './layers-panel.html',
  styleUrl: './layers-panel.scss',
})
export class LayersPanel {
  readonly layers = input.required<MapLayers>();
  readonly visible = input.required<LayerVisibility>();
  readonly status = input<PaintSaveStatus>('saved');

  readonly visibleChange = output<LayerVisibility>();
  readonly retry = output<void>();

  protected readonly lines = computed(() => layerLines(this.layers()));
  protected readonly lights = [3, 2, 1] as const;
  protected readonly lightLabel = LIGHT_LABEL;
  protected readonly lightIcon = { 3: 'light_mode', 2: 'contrast', 1: 'dark_mode' } as const;

  protected toggle(tool: PaintTool, on: boolean): void {
    this.visibleChange.emit({ ...this.visible(), [tool]: on });
  }
}
