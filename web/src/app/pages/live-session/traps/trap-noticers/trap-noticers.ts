import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { GetTrapNoticersResponse, TrapNoticer } from '../../../../../gen/meurpg/maps/v1/maps_pb';

interface Row {
  readonly n: TrapNoticer;
  readonly near: string;
  readonly nearNote: string;
  readonly verdict: 'knows' | 'yes' | 'no';
  readonly verdictWord: string;
  readonly penalty: string;
}

/** What the table says of one character: the server's numbers and flags, in words. */
export function noticerRow(n: TrapNoticer): Row {
  const verdict = n.knows ? 'knows' : n.wouldNotice ? 'yes' : 'no';
  return {
    n,
    near: !n.onMap ? 'Fora do mapa' : n.inRange ? 'Perto, até 3 m' : 'Longe, a mais de 3 m',
    nearNote: n.onMap && n.inRange && !n.sees ? 'não vê a área' : '',
    verdict,
    verdictWord: verdict === 'knows' ? 'Já sabe' : verdict === 'yes' ? 'Nota' : 'Não nota',
    penalty: n.lightPenalty < 0 ? `${n.lightPenalty.toString().replace('-', '−')} na penumbra` : '',
  };
}

/**
 * "Quem notaria" (E9-08 1, MR-035): the Perception of every living player character against the trap's DC
 * to notice it, **as the server works it out** (`GetTrapNoticers`): the passive score from the derived
 * sheet, the −5 of a lightly obscured square, whether the character is on the map, within 3 m of the area
 * and sees it, and whether it would notice standing there. The browser adds, compares and measures
 * nothing: the table writes the numbers and flags it is given. The distance in metres of the artboard is
 * not in the answer, so "Perto" and "Longe" say it in the server's own terms.
 */
@Component({
  selector: 'app-trap-noticers',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './trap-noticers.html',
  styleUrl: './trap-noticers.scss',
})
export class TrapNoticers {
  readonly noticers = input<GetTrapNoticersResponse | undefined>(undefined);
  protected readonly rows = computed(() => (this.noticers()?.noticers ?? []).map(noticerRow));
  protected readonly dc = computed(() => this.noticers()?.noticeDc ?? 0);
}
