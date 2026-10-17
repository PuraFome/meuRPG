import { computed } from '@angular/core';
import type { Combatant, Encounter } from '../../../../gen/meurpg/play/v1/combat_pb';
import type { CombatCalls } from '../../../core/combat/combat-calls';
import type { CombatClient } from '../../../core/combat/combat-client';
import type { CombatState } from '../../../core/combat/combat-state';

/** What `CombatRage` reads from the screen it serves. */
export interface CombatRageContext {
  readonly api: Pick<CombatClient, 'turn'>;
  readonly calls: Pick<CombatCalls, 'run'>;
  readonly campaignId: () => string;
  readonly encounter: () => Encounter | null;
  readonly isMaster: () => boolean;
  readonly state: () => CombatState;
}

/** The raging barbarian's calls: the question the turn waits for and the bonus action that ends the rage. */
export class CombatRage {
  /** The raging barbarian the turn waits for, when the question is mine to answer: the owner's, or the master's. */
  readonly asked = computed<Combatant | null>(() => {
    const e = this.ctx.encounter();
    const id = e?.ragePendingCombatantId;
    const who = id ? e.combatants.find((c) => c.id === id) : undefined;
    return who && (this.ctx.isMaster() || who.mine) ? who : null;
  });

  constructor(private readonly ctx: CombatRageContext) {}

  /** "Voltar e atacar" (`endRage` false) or "Deixar a fúria acabar": the answer to the rage question. */
  async answer(id: string, endRage: boolean): Promise<void> {
    // "Deixar a fúria acabar" ends the rage and then passes the turn, in one tap, as the
    // question says: the server only ends the rage, so the turn goes in the same call.
    await this.ctx.calls.run(async (e) => {
      const answered = await this.ctx.api.turn.answerRageEnd(
        this.ctx.campaignId(),
        e.id,
        id,
        endRage,
      );
      if (!endRage) {
        return answered;
      }
      this.ctx.state().apply(answered);
      return this.ctx.api.turn.endTurn(this.ctx.campaignId(), e.id, id, false, answered.round);
    });
  }

  /** "Encerrar fúria": the bonus action. */
  end(id: string): Promise<boolean> {
    return this.ctx.calls.run((e) => this.ctx.api.turn.endRage(this.ctx.campaignId(), e.id, id));
  }
}
