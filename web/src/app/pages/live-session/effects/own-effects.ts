import { type Signal, computed, effect, inject, signal, untracked } from '@angular/core';

import { type Encounter, EncounterStatus } from '../../../../gen/meurpg/play/v1/combat_pb';
import type { CharacterEffect } from '../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { conditionTags } from '../../../core/combat/conditions';
import { EffectsClient } from '../../../core/effects/effects-client';
import {
  type EffectCardView,
  characterCards,
  characterConditions,
  combatCards,
} from '../../../core/effects/effects';

/** What `OwnEffects` reads from the page. */
export interface OwnEffectsSources {
  readonly campaignId: Signal<string>;
  readonly isMaster: Signal<boolean>;
  /** The combat on the page (the one a player is in), or `null`. */
  readonly encounter: Signal<Encounter | null>;
  /** Moves when something that may change the effects happened (the stream's hints, the vitals' revision). */
  readonly tick: Signal<number>;
}

/**
 * The effects on the player's own character for the live sheet (RN-22): in a running combat the cards come with the
 * player's combatant (`Combatant.effects`, which the stream keeps current); outside one they are read with
 * `ListCharacterEffects` each time the tick moves (a cast, a rest, the master's change of the game time). The master
 * has the effects panel of the combat instead, so nothing is read for them here. A read that fails keeps what was
 * there: the sheet never blocks on it.
 */
export class OwnEffects {
  private readonly api = inject(EffectsClient);
  private readonly outside = signal<readonly CharacterEffect[]>([]);
  private readonly own = computed(() => {
    const e = this.src.encounter();
    return !this.src.isMaster() && e?.status === EncounterStatus.ACTIVE
      ? (e.combatants.find((c) => c.mine) ?? null)
      : null;
  });

  /** The cards of "Seus efeitos". */
  readonly cards = computed<EffectCardView[]>(() => {
    const own = this.own();
    return own ? combatCards(own.effects) : characterCards(this.outside());
  });
  /** The names of the conditions the cards hold, for the label under the numbers. */
  readonly conditions = computed<string[]>(() => {
    const own = this.own();
    return own ? conditionTags(own) : characterConditions(this.outside());
  });

  constructor(private readonly src: OwnEffectsSources) {
    effect(() => {
      const campaignId = this.src.campaignId();
      this.src.tick();
      const inCombat = this.own() !== null;
      if (!campaignId || this.src.isMaster() || inCombat) {
        return;
      }
      untracked(() => void this.read(campaignId));
    });
  }

  private async read(campaignId: string): Promise<void> {
    try {
      this.outside.set(await this.api.listCharacterEffects(campaignId));
    } catch {
      // The next hint reads it again; the sheet keeps what it had.
    }
  }
}
