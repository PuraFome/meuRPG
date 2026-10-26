import type { Encounter, HeldRollOffer } from '../../../../gen/meurpg/play/v1/combat_pb';
import type { AnswerHeldRollResponse } from '../../../../gen/meurpg/play/v1/resources_pb';
import type { CombatCalls } from '../../../core/combat/combat-calls';
import type { CombatClient } from '../../../core/combat/combat-client';
import type { InitiativeManeuver, InitiativeRoll } from '../../../core/combat/combat-client-types';
import { initiativeLine } from '../../../core/combat/combat-derived';
import { HeldRollController } from '../../../core/held-rolls/held-roll-controller';
import type { HeldAnswer } from '../../../core/held-rolls/held-roll-offer';
import { type RollAnimator, showOfDie } from '../../../shared/roll-overlay/roll-animator';

/** The d20 the initiative is rolled with. */
const INITIATIVE_DIE = 20;

/** What `CombatInitiative` reads from the screen it serves. */
export interface CombatInitiativeContext {
  readonly api: Pick<CombatClient, 'turn'>;
  /** Answers the question of a held roll (`ResourceClient.answerHeldRoll`). */
  readonly answerHeld: (
    holdId: string,
    answer: HeldAnswer,
    idempotencyKey: string,
  ) => Promise<AnswerHeldRollResponse>;
  readonly animator: Pick<RollAnimator, 'play'>;
  readonly calls: Pick<CombatCalls, 'run'>;
  readonly campaignId: () => string;
  readonly encounter: () => Encounter | null;
  readonly ownId: () => string | undefined;
}

/** The initiative of the combat's setup: a typed face, the roll in the app and the master's order. */
export class CombatInitiative {
  /** The question of the player's own initiative when the d20 was kept for one: the halfling's Lucky about a 1, then the Bardic
   * Inspiration die the character holds; the initiative is not set until it is answered. */
  readonly held: HeldRollController;
  /** The table die the player ticked for their own initiative (set by the initiative screen); sent with their roll. */
  maneuver: InitiativeManeuver | null = null;

  constructor(private readonly ctx: CombatInitiativeContext) {
    this.held = new HeldRollController({
      send: (holdId, answer, key) => ctx.answerHeld(holdId, answer, key),
      settle: (res) => this.answered(res),
    });
  }

  /** Shows the question the combat says waits for the player's own initiative (a reload, or another screen asked it). */
  resume(offer: HeldRollOffer | undefined): void {
    if (offer?.holdId !== this.held.offer()?.holdId) {
      this.held.ask(offer);
    }
  }

  submitFace(combatantId: string, face: number): Promise<boolean> {
    return this.send(combatantId, { face });
  }

  async rollInApp(combatantId: string): Promise<boolean> {
    const done = await this.send(combatantId, { inApp: true });
    if (done) {
      this.animate(combatantId);
    }
    return done;
  }

  /** One initiative call; a d20 kept for a question is asked on the player's own screen. */
  private send(combatantId: string, roll: InitiativeRoll): Promise<boolean> {
    return this.ctx.calls.run(async (e) => {
      const maneuver = combatantId === this.ctx.ownId() ? this.maneuver : null;
      const args = [this.ctx.campaignId(), e.id, combatantId, roll] as const;
      const res = await (maneuver
        ? this.ctx.api.turn.submitInitiative(...args, maneuver)
        : this.ctx.api.turn.submitInitiative(...args));
      // The master's roll for a player: the question sits in the player's row (the order's), so only the player's own is asked here.
      if (res.held && combatantId === this.ctx.ownId()) {
        this.held.ask(res.held);
      }
      return res.encounter;
    });
  }

  /** The answer to the question set the initiative: the combat after it, and the d20 on the table. */
  private async answered(res: AnswerHeldRollResponse): Promise<void> {
    if (res.result.case !== 'initiative') {
      return;
    }
    const encounter = res.result.value.encounter;
    if (encounter) {
      await this.ctx.calls.run(() => Promise.resolve(encounter));
      const own = this.ctx.ownId();
      if (own) {
        this.animate(own);
      }
    }
  }

  /** The initiative d20 the app just rolled for the player's own combatant, as the initiative card tells it (d20 + bonus = total). */
  animate(combatantId: string): void {
    const shown = initiativeLine(
      this.ctx.encounter()?.combatants.find((x) => x.id === combatantId),
    );
    const show = shown && showOfDie('Iniciativa', INITIATIVE_DIE, shown.face, shown.line);
    if (show) {
      this.ctx.animator.play(show);
    }
  }

  setOrder(ids: string[]): Promise<boolean> {
    return this.ctx.calls.run((e) => this.ctx.api.turn.setOrder(this.ctx.campaignId(), e.id, ids));
  }
}
