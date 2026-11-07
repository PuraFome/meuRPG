import { NgTemplateOutlet } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { TvCombatantVm, TvMapVm, TvRollVm } from '../../../core/play/table-screen-view';
import { TvMap } from '../tv-map/tv-map';
import { TvOrder } from '../tv-order/tv-order';
import { TvRolls } from '../tv-rolls/tv-rolls';

/** Lines of "Último no combate" beside a map (the order and the rolls share the column), and with no map. */
export const TV_LOG_WITH_MAP = 1;
export const TV_LOG_WITHOUT_MAP = 6;

/**
 * The combat as the table sees it: the map (or its card, or nothing in a combat without one), the order with the
 * round, the last lines of the log and the free rolls. The server sends what the table may read; this lays it out.
 */
@Component({
  selector: 'app-tv-combat',
  imports: [MatIconModule, NgTemplateOutlet, TvMap, TvOrder, TvRolls],
  templateUrl: './tv-combat.html',
  styleUrl: './tv-combat.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TvCombat {
  readonly round = input(1);
  readonly order = input<readonly TvCombatantVm[]>([]);
  readonly mastersTurn = input(false);
  readonly log = input<readonly string[]>([]);
  readonly map = input<TvMapVm | null>(null);
  readonly rolls = input<readonly TvRollVm[]>([]);

  protected readonly lines = computed(() =>
    this.log().slice(0, this.map() ? TV_LOG_WITH_MAP : TV_LOG_WITHOUT_MAP),
  );
}
