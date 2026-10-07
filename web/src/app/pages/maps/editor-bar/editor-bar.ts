import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';

import { MapPointKind } from '../../../../gen/meurpg/maps/v1/maps_pb';
import {
  COVER_LABEL,
  type CoverDegree,
  DOOR_CHOICES,
  DOOR_LABEL,
  LIGHT_LABEL,
  type LightDegree,
  type PaintSettings,
  type PaintTool,
  TOOL_LABEL,
} from '../../../core/maps/paint-tools';
import { DoorMark } from '../../../shared/map-layers/door-mark';
import type { RosterEntry } from '../../../core/maps/roster-client';
import { ChestIcon } from '../../../shared/chest-icon/chest-icon';
import { pointKindIcon, pointKindLabel } from '../../../shared/map-view/map-labels';
import { scaleLabel } from '../../../shared/map-view/map-geometry';

export type EditorMode = 'points' | 'paint';

const POINT_KINDS = [MapPointKind.BATTLE, MapPointKind.SUBMAP, MapPointKind.SCENE] as const;
const NEW_KINDS = [MapPointKind.LIGHT, MapPointKind.TRAP, MapPointKind.TREASURE] as const;
const TOOLS: readonly { tool: PaintTool; icon: string }[] = [
  { tool: 'terrain', icon: 'landscape' },
  { tool: 'wall', icon: 'brick' },
  { tool: 'cover', icon: 'fence' },
  { tool: 'light', icon: 'lightbulb' },
  { tool: 'door', icon: 'door_front' },
];
const LIGHT_ICON: Readonly<Record<LightDegree, string>> = {
  3: 'light_mode',
  2: 'contrast',
  1: 'dark_mode',
};

/**
 * The map editor's bar (E9-01, E9-02): "Pontos | Pintar", and, by mode, either "Adicionar ponto" (Batalha,
 * Submapa, Cena de RP, and the new Luz, Armadilha and Tesouro) with the token menu, or the paint tools: Terreno
 * difícil, Parede, Cobertura, Luz, "Apagar" and the brush (1×1, 3×3), with a second line for the degree of the
 * cover (Meia, Três quartos) or of the light (Claro, Penumbra, Escuro). The zoom buttons close it. The selected
 * choice always shows its check first, never colour alone. Without a grid the paint tools stay but say they
 * cannot act (`aria-disabled`, the reason under the bar). Presentational: the editor owns the state.
 */
@Component({
  selector: 'app-editor-bar',
  imports: [ChestIcon, DoorMark, MatButtonModule, MatIconModule, MatMenuModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './editor-bar.html',
  styleUrl: './editor-bar.scss',
})
export class EditorBar {
  readonly mode = input.required<EditorMode>();
  readonly settings = input.required<PaintSettings>();
  /** The map has a grid: painting can start. */
  readonly canPaint = input(true);
  readonly placing = input<MapPointKind | null>(null);
  /** The characters with no token on the map yet ("Adicionar token"). */
  readonly available = input<readonly RosterEntry[]>([]);
  readonly scale = input(1);
  /** Why the paint tools cannot act (no grid, the layers not read yet), written next to them. */
  readonly why = input('');
  readonly whyIcon = input('grid_on');

  readonly modeChange = output<EditorMode>();
  readonly settingsChange = output<PaintSettings>();
  readonly toggleKind = output<MapPointKind>();
  readonly addToken = output<RosterEntry>();
  readonly zoomIn = output<void>();
  readonly zoomOut = output<void>();
  readonly fit = output<void>();

  protected readonly Treasure = MapPointKind.TREASURE;
  protected readonly pointKinds = POINT_KINDS;
  protected readonly newKinds = NEW_KINDS;
  protected readonly tools = TOOLS;
  protected readonly toolLabel = TOOL_LABEL;
  protected readonly coverLabel = COVER_LABEL;
  protected readonly lightLabel = LIGHT_LABEL;
  protected readonly lightIcon = LIGHT_ICON;
  protected readonly kindLabel = pointKindLabel;
  protected readonly kindIcon = pointKindIcon;
  protected readonly scaleText = computed(() => scaleLabel(this.scale()));
  protected readonly covers: readonly CoverDegree[] = [1, 2];
  protected readonly lights: readonly LightDegree[] = [3, 2, 1];
  protected readonly doorChoices = DOOR_CHOICES;
  protected readonly doorLabel = DOOR_LABEL;

  /** A tool looks chosen only when it can act, and only one at a time: the eraser takes the place of the tool it erases. */
  protected toolOn(tool: PaintTool): boolean {
    return this.canPaint() && this.settings().tool === tool && !this.settings().erase;
  }

  protected eraseOn(): boolean {
    return this.canPaint() && this.settings().erase;
  }

  protected set(change: Partial<PaintSettings>): void {
    this.settingsChange.emit({ ...this.settings(), ...change });
  }

  /** Arrow keys inside a segmented control (the radio pattern). */
  protected onSegKey(event: KeyboardEvent): void {
    const keys = ['ArrowLeft', 'ArrowUp', 'ArrowRight', 'ArrowDown'];
    if (!keys.includes(event.key)) {
      return;
    }
    event.preventDefault();
    const step = event.key === 'ArrowLeft' || event.key === 'ArrowUp' ? -1 : 1;
    const buttons = Array.from(
      (event.currentTarget as HTMLElement).querySelectorAll<HTMLElement>('[role="radio"]'),
    );
    const here = buttons.findIndex((b) => b === document.activeElement);
    const next = buttons[(Math.max(0, here) + step + buttons.length) % buttons.length];
    next?.focus();
    next?.click();
  }
}
