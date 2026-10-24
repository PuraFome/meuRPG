import { computed, signal } from '@angular/core';

import type { Encounter } from '../../../../gen/meurpg/play/v1/combat_pb';
import type { CombatCalls } from '../../../core/combat/combat-calls';
import type { CombatClient } from '../../../core/combat/combat-client';
import type { CombatState } from '../../../core/combat/combat-state';
import { zoneEndError } from '../../../core/combat/zone-errors';
import { zoneViews } from '../../../core/combat/zones';

/** What `ZoneFlow` reads from the screen it serves. */
export interface ZoneFlowContext {
  readonly api: Pick<CombatClient, 'casts'>;
  readonly calls: Pick<CombatCalls, 'busy' | 'refreshAfter'>;
  readonly campaignId: () => string;
  readonly encounter: () => Encounter | null;
  readonly state: () => CombatState;
  readonly isMaster: () => boolean;
  /** Opens the move picker for a zone; `onClosed` runs when it closes. `false` when there is no map to pick on. */
  readonly openMove: (zoneId: string, onClosed: () => void) => boolean;
}

/**
 * The spell zones on the combat screen: the list the panel and the map read, the master's "Encerrar zona" and the zone he asked
 * to move. The end shares the screen's one-call-at-a-time rule (`calls.busy`) but words its own refusals.
 */
export class ZoneFlow {
  /** The zone whose move picker is open ("Mover"); `null` when none is. */
  readonly moving = signal<string | null>(null);
  /** A refusal of the last end, said in the panel. */
  readonly error = signal('');

  readonly views = computed(() => zoneViews(this.ctx.encounter(), this.ctx.isMaster()));

  constructor(private readonly ctx: ZoneFlowContext) {}

  /** "Mover": opens the picker for a zone the viewer may move (a movement ACTION zone and the master); a scene without a map says so. */
  move(zoneId: string): void {
    if (!this.views().find((v) => v.id === zoneId)?.canMove) {
      return;
    }
    this.error.set('');
    this.moving.set(zoneId);
    const opened = this.ctx.openMove(zoneId, () => this.moving.set(null));
    if (!opened) {
      this.moving.set(null);
      this.error.set('Esta cena não tem mapa para escolher o lugar da zona.');
    }
  }

  /** `EndSpellZone`: the combat as it is now is applied; `false` when no call was made or the server refused it. */
  async end(zoneId: string): Promise<boolean> {
    const e = this.ctx.encounter();
    if (!e || this.ctx.calls.busy()) {
      return false;
    }
    this.ctx.calls.busy.set(true);
    this.error.set('');
    try {
      this.ctx
        .state()
        .apply(await this.ctx.api.casts.endSpellZone(this.ctx.campaignId(), e.id, zoneId));
      return true;
    } catch (err) {
      this.error.set(zoneEndError(err));
      await this.ctx.calls.refreshAfter(err);
      return false;
    } finally {
      this.ctx.calls.busy.set(false);
    }
  }
}
