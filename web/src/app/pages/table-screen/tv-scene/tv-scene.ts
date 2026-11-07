import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { CombatantToken } from '../../../shared/combatant-token/combatant-token';

/** How many NPCs "Em cena" draws before "+N". */
export const TV_STAGE_MAX = 8;

export interface TvSceneNpc {
  readonly name: string;
  readonly speaking: boolean;
}

/** The scene open to the whole table: its description, large, and the NPCs the table sees, with who speaks. */
@Component({
  selector: 'app-tv-scene',
  imports: [CombatantToken, MatIconModule],
  templateUrl: './tv-scene.html',
  styleUrl: './tv-scene.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TvScene {
  readonly description = input('');
  readonly stage = input<readonly TvSceneNpc[]>([]);

  protected readonly shown = computed(() => this.stage().slice(0, TV_STAGE_MAX));
  protected readonly more = computed(() => Math.max(0, this.stage().length - TV_STAGE_MAX));

  protected initial(name: string): string {
    return [...name.trim()][0]?.toUpperCase() ?? '?';
  }
}
