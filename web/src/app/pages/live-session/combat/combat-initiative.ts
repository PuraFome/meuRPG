import type { Encounter } from '../../../../gen/meurpg/play/v1/combat_pb';
import type { CombatCalls } from '../../../core/combat/combat-calls';
import type { CombatClient } from '../../../core/combat/combat-client';
import { initiativeLine } from '../../../core/combat/combat-derived';
import { type RollAnimator, showOfDie } from '../../../shared/roll-overlay/roll-animator';

/** The d20 the initiative is rolled with. */
const INITIATIVE_DIE = 20;

/** What `CombatInitiative` reads from the screen it serves. */
export interface CombatInitiativeContext {
  readonly api: Pick<CombatClient, 'turn'>;
  readonly animator: Pick<RollAnimator, 'play'>;
  readonly calls: Pick<CombatCalls, 'run'>;
  readonly campaignId: () => string;
  readonly encounter: () => Encounter | null;
}

/** The initiative of the combat's setup: a typed face, the roll in the app and the master's order. */
export class CombatInitiative {
  constructor(private readonly ctx: CombatInitiativeContext) {}

  submitFace(combatantId: string, face: number): Promise<boolean> {
    return this.ctx.calls.run((e) =>
      this.ctx.api.turn.submitInitiative(this.ctx.campaignId(), e.id, combatantId, { face }),
    );
  }

  async rollInApp(combatantId: string): Promise<boolean> {
    const done = await this.ctx.calls.run((e) =>
      this.ctx.api.turn.submitInitiative(this.ctx.campaignId(), e.id, combatantId, { inApp: true }),
    );
    if (done) {
      this.animate(combatantId);
    }
    return done;
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
