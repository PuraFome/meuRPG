import { type Signal, computed } from '@angular/core';

import type { LevelUpOptions } from '../../../gen/meurpg/characters/v1/characters_pb';
import type { Ability, FeatOption } from '../../../gen/meurpg/rules/v1/rules_pb';
import { ABILITY_FROM_GEN } from '../characters/ability-gen';
import type { AbilityKey } from '../characters/characters.types';
import { abilityName } from '../content/feat-text';
import type { AsiMode } from './levelup-draft-types';

/** The picks the ability step holds, which the abilities of the draft are read from. */
export interface AbilityPicks {
  readonly abilityMode: Signal<'one' | 'two'>;
  readonly abilityKeys: Signal<readonly AbilityKey[]>;
  readonly asiMode: Signal<AsiMode>;
  readonly featKey: Signal<string>;
  readonly featAbilities: Signal<readonly Ability[]>;
}

/**
 * What the ability step makes of its picks: the increase of the SRD (+2 in one, +1 in two) or a feat (and the
 * abilities it raises, up to the 20 the server says a score stops at), as the Resumo and the request read them.
 */
export class LevelUpAbilities {
  readonly abilityAsked = computed(() => (this.picks.abilityMode() === 'one' ? 1 : 2));
  /** The feats the server offers at this level (empty: the step is today's increase only). */
  readonly feats = computed<readonly FeatOption[]>(() => this.options.feats);
  readonly hasFeats = computed(
    () => this.options.abilityScoreImprovement && this.feats().length > 0,
  );
  readonly taking = computed(() => this.hasFeats() && this.picks.asiMode() === 'feat');
  /** The picked feat, as the server lists it. */
  readonly feat = computed(() => this.feats().find((f) => f.key === this.picks.featKey()));
  /** The feat raises every ability it lists: nothing to pick, and the server applies it (stopping each at 20). */
  readonly featFixed = computed(() => {
    const inc = this.feat()?.increase;
    return inc !== undefined && inc.count >= inc.from.length;
  });
  /** The abilities of the feat's list the server says would pass 20. */
  readonly featCapped = computed<readonly Ability[]>(() => {
    const feat = this.feat();
    return feat?.increase ? feat.cappedAbilities : [];
  });
  /** How many abilities the player picks for a feat that gives a choice: the count, or fewer when too few are below 20. */
  readonly featAsked = computed(() => {
    const inc = this.feat()?.increase;
    if (!inc || this.featFixed()) {
      return 0;
    }
    const free = inc.from.filter((a) => !this.featCapped().includes(a)).length;
    return Math.min(inc.count, free);
  });
  /** "Talento: Atleta" with "+1 Força, +1 Destreza": what the Resumo and the list of the level say of the feat. */
  readonly featSummary = computed(() => {
    const feat = this.feat();
    if (!this.taking() || !feat) {
      return null;
    }
    const inc = feat.increase;
    const raised = this.featFixed()
      ? (inc?.from ?? []).filter((a) => !this.featCapped().includes(a))
      : this.picks.featAbilities();
    return {
      name: feat.namePt,
      increase: inc ? raised.map((a) => `+${inc.value} ${abilityName(a)}`).join(', ') : '',
    };
  });
  /** The abilities raised by what was picked, in the sheet's own keys: the increase, or the feat's. */
  readonly pickedKeys = computed<readonly AbilityKey[]>(() => {
    if (!this.taking()) {
      return this.picks.abilityKeys();
    }
    const raised = this.featFixed()
      ? (this.feat()?.increase?.from ?? [])
      : this.picks.featAbilities();
    return raised.map((a) => ABILITY_FROM_GEN[a]);
  });
  readonly abilityIncrease = computed<Partial<Record<AbilityKey, number>>>(() => {
    if (this.taking()) {
      // A feat that raises every ability it lists sends no increase: the server applies it, stopping at 20.
      const inc = this.feat()?.increase;
      const picked = this.picks.featAbilities();
      if (!inc || this.featFixed() || picked.length !== this.featAsked()) {
        return {};
      }
      return Object.fromEntries(picked.map((a) => [ABILITY_FROM_GEN[a], inc.value]));
    }
    const keys = this.picks.abilityKeys();
    if (!this.options.abilityScoreImprovement || keys.length !== this.abilityAsked()) {
      return {};
    }
    const amount = this.picks.abilityMode() === 'one' ? 2 : 1;
    return Object.fromEntries(keys.map((k) => [k, amount]));
  });

  constructor(
    private readonly options: LevelUpOptions,
    private readonly picks: AbilityPicks,
  ) {}

  /** The abilities the feat raises once `ability` is tapped: picked or dropped, never more than asked; `null` when the tap changes nothing. */
  nextFeatAbilities(ability: Ability): readonly Ability[] | null {
    const inc = this.feat()?.increase;
    const now = this.picks.featAbilities();
    if (!inc || this.featFixed() || !inc.from.includes(ability)) {
      return null;
    }
    if (now.includes(ability)) {
      return now.filter((a) => a !== ability);
    }
    if (this.featCapped().includes(ability)) {
      return null;
    }
    if (this.featAsked() === 1) {
      return [ability];
    }
    return now.length < this.featAsked() ? [...now, ability] : null;
  }

  /** The ability picks once `key` is tapped: dropped when picked, else added, a third replacing the oldest. */
  nextAbilityKeys(key: AbilityKey): readonly AbilityKey[] {
    const keys = this.picks.abilityKeys();
    return keys.includes(key)
      ? keys.filter((k) => k !== key)
      : [...keys, key].slice(-this.abilityAsked());
  }

  /** The feat abilities of an earlier draft that still stand for this feat: listed, below 20 and no more than asked. */
  adoptedFeatAbilities(was: readonly Ability[]): readonly Ability[] {
    const inc = this.feat()?.increase;
    return was
      .filter((a) => inc?.from.includes(a) && !this.featCapped().includes(a))
      .slice(0, this.featAsked());
  }
}
