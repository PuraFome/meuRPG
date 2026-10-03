import { Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant, PendingDamage } from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { ActionOption, SpellOption, TurnOptions } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { feetToMeters, formatMeters, tight } from '../../../../core/combat/combat-grid';
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
 * the spells and features of their economy; Movimento, "Mover". Casting a
 * spell and the features' buttons come with the spells slice: those rows
 * are listed without a button, ready to get one (`cast`, `feature`).
 * Disabled options keep their place with the reason, by code (`reasonText`).
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
  /** A hit of this turn whose damage is still to roll. */
  readonly pendingRoll = input<PendingDamage | null>(null);
  readonly busy = input(false);

  /** "Atacar": the key of the attack, as in `Attack.key`. */
  readonly attack = output<string>();
  /** A standard action, by key ("standard:dash"). */
  readonly action = output<string>();
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

  protected featureDetail(a: ActionOption): string {
    return a.usesLeft > 0 ? `${a.usesLeft} ${a.usesLeft === 1 ? 'uso' : 'usos'} restantes` : '';
  }
}
