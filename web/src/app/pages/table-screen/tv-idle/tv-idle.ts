import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { CombatantToken } from '../../../shared/combatant-token/combatant-token';

/** How many names "Na mesa" draws before "+N". */
export const TV_PARTY_MAX = 12;

/** The waiting screen: "Nada à mostra ainda" and who is at the table. */
@Component({
  selector: 'app-tv-idle',
  imports: [CombatantToken],
  templateUrl: './tv-idle.html',
  styleUrl: './tv-idle.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TvIdle {
  readonly party = input<readonly string[]>([]);

  protected readonly shown = computed(() => this.party().slice(0, TV_PARTY_MAX));
  protected readonly more = computed(() => Math.max(0, this.party().length - TV_PARTY_MAX));

  protected initial(name: string): string {
    return [...name.trim()][0]?.toUpperCase() ?? '?';
  }
}
