import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { GetTrapNoticersResponse, TrapNoticer } from '../../../gen/meurpg/maps/v1/maps_pb';

interface Row {
  readonly n: TrapNoticer;
  /** "Perto, até 3 m", "Longe, a mais de 3 m", "Fora do mapa". */
  readonly where: string;
  readonly whereNote: string;
  readonly verdict: 'knows' | 'yes' | 'no';
  readonly verdictWord: string;
  /** "−5 na penumbra" when the server says the light takes 5 off. */
  readonly penalty: string;
}

/** What the table says of one character, from the server's numbers and flags alone: whoever is in range reads `would_notice`; whoever is not
 * reads `passes_dc` ("Se chegar a 3 m: nota"). The browser adds and compares nothing. */
export function noticerRow(n: TrapNoticer): Row {
  const near = n.onMap && n.inRange;
  const verdict = n.knows ? 'knows' : (near ? n.wouldNotice : n.passesDc) ? 'yes' : 'no';
  const word = verdict === 'knows' ? 'Já sabe' : verdict === 'yes' ? 'nota' : 'não nota';
  return {
    n,
    where: !n.onMap ? 'Fora do mapa' : n.inRange ? 'Perto, até 3 m' : 'Longe, a mais de 3 m',
    whereNote: n.onMap && n.inRange && !n.sees ? 'não vê a área' : '',
    verdict,
    verdictWord: verdict === 'knows' ? word : near ? word.charAt(0).toUpperCase() + word.slice(1) : `Se chegar a 3 m: ${word}`,
    penalty: n.lightPenalty < 0 ? `${n.lightPenalty.toString().replace('-', '−')} na penumbra` : '',
  };
}

/**
 * "Quem notaria" (E9-08 1, MR-035): the Perception of every living player character against the trap's DC to notice it, **as the server works it
 * out** (`GetTrapNoticers`): the passive score from the derived sheet with the light penalty beside it, where the character stands (on the map, within
 * 3 m, seeing the area) and whether it notices. A character in range reads "Nota" or "Não nota"; one out of range, "Se chegar a 3 m: nota" or "não nota"
 * (`passes_dc`). The distance in metres of the artboard is not in the answer. On a phone each character is one compact row. A failed read says so and
 * offers "Tentar de novo".
 */
@Component({
  selector: 'app-trap-noticers',
  imports: [MatButtonModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './trap-noticers.html',
  styleUrl: './trap-noticers.scss',
  host: { '[class.tn--panel]': 'panel()' },
})
export class TrapNoticers {
  readonly noticers = input<GetTrapNoticersResponse | undefined>(undefined);
  /** The read failed. */
  readonly failed = input(false);
  readonly retry = output<void>();
  /** In the editor's side panel: the panel's title style, and the rows stack in a narrow column (a container query). The session card keeps its own layout. */
  readonly panel = input(false);
  protected readonly rows = computed(() => (this.noticers()?.noticers ?? []).map(noticerRow));
}
