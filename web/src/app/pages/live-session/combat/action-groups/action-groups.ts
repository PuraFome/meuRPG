import { Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant, PendingDamage } from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { ActionOption, Attack, SpellOption, TurnOptions } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { feetToMeters, formatMeters } from '../../../../core/combat/combat-grid';
import { tight } from '../../../../core/format/text';
import {
  attackDetail,
  attackName,
  circleLabel,
  groupState,
  isCantrip,
  isReactionHint,
  optionsFor,
  reasonText,
  spellDetail,
} from '../../../../core/combat/combat-options';
import { mediaQuery } from '../../../../shared/map-view/media-query';
import { ActionRow } from './action-row';
import { EconomyTiles } from './economy-tiles';
import { GroupState } from './group-state';

/**
 * "O que você pode fazer" (E6-06 on a phone, E6-14 on a laptop): the options
 * `GetTurnOptions` works out, grouped by economy. Each group header carries
 * its state word (the open circle and "Disponível", or "Usada"). Ação has
 * the attacks, the spells and the standard actions; Ação bônus and Reação
 * the spells and features of their economy; Movimento, "Mover". A spell
 * row has "Conjurar" (the cast sheet, `cast`; a cantrip that asks for a saving
 * throw is cast too, not attacked), a feature row "Usar" (`feature`, a
 * TakeAction); Escudo, a reaction, has none: it asks when a hit lands.
 * With Extra Attack the Ação header says how many attacks are left and the
 * attacks stay enabled. Disabled options keep their place with the reason, by
 * code (`reasonText`).
 */
@Component({
  selector: 'app-action-groups',
  imports: [ActionRow, EconomyTiles, GroupState, MatButtonModule, MatIconModule],
  templateUrl: './action-groups.html',
  styleUrl: './action-groups.scss',
})
export class ActionGroups {
  readonly options = input.required<TurnOptions>();
  /** The player's own combatant: its economy and movement. */
  readonly own = input.required<Combatant>();
  /** Extra Attack: the attacks of this Attack action that remain, and how many
   * it makes (`TurnEconomy`). */
  readonly attacksLeft = input(0);
  readonly attacksPerAction = input(1);
  /** What the last feature said ("Você tem outra ação"), in a live region. */
  readonly note = input('');
  /** A hit of this turn whose damage is still to roll. */
  readonly pendingRoll = input<PendingDamage | null>(null);
  readonly busy = input(false);

  /** "Atacar": the key of the attack, as in `Attack.key`. */
  readonly attack = output<string>();
  /** A standard action, by key ("standard:dash"). */
  readonly action = output<string>();
  /** "Conjurar": the key of the spell, or of a cantrip that asks for a save. */
  readonly cast = output<string>();
  /** "Usar": the key of a feature action ("feature:second-wind"). */
  readonly feature = output<string>();
  readonly move = output<void>();
  /** "Rolar o dano" of a hit that waits for its roll. */
  readonly rollDamage = output<void>();


  /** From 1024px the economy tiles are the panel's first thing (E6-14). */
  protected readonly desktop = mediaQuery('(min-width: 1024px)');

  protected readonly action_ = computed(() => optionsFor(this.options(), 'action'));
  protected readonly bonus = computed(() => optionsFor(this.options(), 'bonus'));
  protected readonly reaction = computed(() => optionsFor(this.options(), 'reaction'));
  /** Atacar and Conjurar are not repeated here: they are the rows above. */
  protected readonly standard = computed(() =>
    this.options().standardActions.filter((a) => !/^standard:(attack|cast-a-spell)$/.test(a.action?.key ?? '')),
  );
  protected readonly standardWhy = computed(() => {
    const first = this.standard().find((a) => !a.enabled);
    return first ? reasonText(first.reason) : '';
  });
  protected readonly movement = computed(() => {
    const own = this.own();
    const left = own.movementLeftFt;
    const used = own.movementUsedFt;
    const squares = Math.floor(left / 5);
    return {
      left,
      pill: left > 0 ? tight(`Restam ${formatMeters(feetToMeters(left))}`) : 'Sem movimento',
      text:
        left > 0
          ? tight(
              `${used > 0 ? `Você já andou ${formatMeters(feetToMeters(used))}. ` : ''}Dá para andar ${used > 0 ? 'mais ' : ''}${formatMeters(feetToMeters(left))} (${squares} ${squares === 1 ? 'quadrado' : 'quadrados'}).`,
            )
          : 'Você já usou todo o movimento deste turno.',
    };
  });

  /** The word on the Ação header: with Extra Attack, once the first attack
   * spent the action, "1 ataque restante" (an open circle: it is not over). */
  protected readonly actionWord = computed(() => {
    const used = this.own().actionUsed;
    const left = this.attacksLeft();
    if (used && left > 0 && this.attacksPerAction() > 1) {
      return { word: `${left} ${left === 1 ? 'ataque restante' : 'ataques restantes'}`, used: false };
    }
    return { word: groupState(used), used };
  });

  protected readonly attackName = attackName;
  protected readonly attackDetail = (a: Parameters<typeof attackDetail>[0]) => attackDetail(a);
  protected readonly reasonText = reasonText;
  protected readonly state = groupState;
  protected readonly isCantrip = isCantrip;

  protected pillOf(s: SpellOption): string {
    return circleLabel(s.spell?.level ?? 0);
  }

  protected spellDetail(s: SpellOption): string {
    return spellDetail(s);
  }

  /** A reaction that waits for its trigger says so instead of a reason. */
  protected spellHint(s: SpellOption): string {
    return isReactionHint(s.reason)
      ? 'Quando você for atingido, o app pergunta se quer usar.'
      : this.spellDetail(s);
  }

  protected spellOff(s: SpellOption): boolean {
    return !s.enabled && !isReactionHint(s.reason);
  }

  protected featureOff(a: ActionOption): boolean {
    return !a.enabled && !isReactionHint(a.reason);
  }

  /** "1 uso", "2 usos": what is left of the feature's resource. */
  protected featureDetail(a: ActionOption): string {
    return a.usesLeft > 0 ? `${a.usesLeft} ${a.usesLeft === 1 ? 'uso' : 'usos'}` : '';
  }

  /** A save cantrip (Chama Sagrada) is in the attacks, but it is cast. */
  protected asksSave(a: Attack): boolean {
    return a.saveDc > 0;
  }
}
