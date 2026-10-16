import type { Signal } from '@angular/core';
import { computed } from '@angular/core';

import type {
  Encounter,
  GetMoveOptionsResponse,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { Square } from '../../../../core/combat/combat-grid';
import { combatantInitial } from '../../../../core/combat/combat-view';
import type { SquareVerdict } from '../../../../core/combat/move-plan';
import {
  dragLegend,
  dragMovementLine,
  dragSentence,
  dragged,
  draggedTo,
} from '../../../../core/combat/drag-plan';
import type { DraggedMark } from '../../../../shared/combat-map/combat-map';

/** What the page knows that the drag words depend on. */
export interface MoveDragInputs {
  readonly options: Signal<GetMoveOptionsResponse | null>;
  readonly encounter: Signal<Encounter>;
  readonly jumping: Signal<boolean>;
  readonly chosen: Signal<Square | null>;
  readonly verdict: Signal<SquareVerdict | null>;
  readonly origin: Signal<Square>;
  readonly leftDft: Signal<number>;
}

/** The creature the mover drags (W7-X: it holds it grappled), with the speed halved unless it is much smaller. */
export class MoveDrag {
  private readonly who = computed(() => dragged(this.i.options(), this.i.encounter().combatants));
  readonly legend = computed(() => {
    const who = this.who();
    return who ? dragLegend(who) : '';
  });
  /** "Deslocamento: 4,5 m (metade, arrastando o Hobgoblin)": the movement left is the server's, already halved. */
  readonly line = computed(() => {
    const who = this.who();
    return who && !this.i.jumping()
      ? dragMovementLine(this.i.leftDft(), this.i.options()?.draggingHalved ?? false, who)
      : '';
  });
  /** Where the dragged creature ends if the mover goes to the chosen square (the server's square, never the page's). */
  private readonly end = computed<Square | null>(() => {
    const v = this.i.verdict();
    return this.i.chosen() && v?.kind === 'ok' && !this.i.jumping() ? draggedTo(v.square) : null;
  });
  readonly mark = computed<DraggedMark | null>(() => {
    const who = this.who();
    const square = this.end();
    return who && square ? { square, initial: combatantInitial(who.label) } : null;
  });
  readonly text = computed(() => {
    const who = this.who();
    const to = this.i.chosen();
    return who && to && this.end() ? dragSentence(this.i.origin(), to, who, this.i.leftDft()) : '';
  });

  constructor(private readonly i: MoveDragInputs) {}
}
