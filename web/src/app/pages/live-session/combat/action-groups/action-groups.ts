import { Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant, PendingDamage } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  type ActionOption,
  type Attack,
  type SpellDetails,
  type SpellOption,
  type TurnOptions,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { distanceInSentence, metersText } from '../../../../core/units';
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
  spellLine,
  spellTags,
} from '../../../../core/combat/combat-options';
import { spellSummary } from '../../../../core/combat/spell-summary';
import { freeText } from '../../../../core/combat/cast-flow';
import { mediaQuery } from '../../../../shared/map-view/media-query';
import type { PactSlotsVm, SlotUsageVm } from '../../live-session.types';
import { SlotDots } from '../../slot-dots/slot-dots';
import { ActionRow } from './action-row';
import { EconomyTiles } from './economy-tiles';
import { GroupState } from './group-state';

/**
 * "O que você pode fazer" (E6-06 on a phone, E6-14 on a laptop): the options
 * `GetTurnOptions` works out, grouped by economy. Each group header carries
 * its state word (the open circle and "Disponível", or "Usada"). Ação has
 * the attacks, the spells and the standard actions; Ação bônus and Reação
 * the features of their economy; Movimento, "Mover". The spells of every
 * economy are one list in Ação, in the order the server sends (the ones that can
 * be cast now first, then by circle and name: it is not sorted again here), each
 * with the "?" that opens its description (`describe`) and a tag for the economy
 * when it is not an action; the slot rows above them are the one explanation of
 * every "Sem espaço". A spell row has "Conjurar" (the cast sheet, `cast`; a
 * cantrip that asks for a saving throw is cast too, not attacked), a feature row
 * "Usar" (`feature`, a TakeAction); Escudo, a reaction, has none: it asks when a
 * hit lands, and on the character's own turn it says "Só fora da sua vez".
 * With Extra Attack the Ação header says how many attacks are left and the
 * attacks stay enabled. Disabled options keep their place with the reason, by
 * code (`reasonText`).
 */
@Component({
  selector: 'app-action-groups',
  imports: [ActionRow, EconomyTiles, GroupState, MatButtonModule, MatIconModule, SlotDots],
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
  /** The character's slots, for the rows above the spells ("1º círculo ○ ✕ ✕ ✕ 1 livre de 4"). */
  readonly slots = input<{ readonly usage: readonly SlotUsageVm[]; readonly pact: PactSlotsVm | null }>({
    usage: [],
    pact: null,
  });

  /** What `GetSpellDetails` said about each spell of the list, for the line under its name. */
  readonly details = input<ReadonlyMap<string, SpellDetails>>(new Map());

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
  /** The "?" of a spell: the key and the Portuguese name. */
  readonly describe = output<{ key: string; name: string }>();


  /** From 1024px the economy tiles are the panel's first thing (E6-14). */
  protected readonly desktop = mediaQuery('(min-width: 1024px)');

  protected readonly action_ = computed(() => optionsFor(this.options(), 'action'));
  /** Every spell, whatever its economy, as the server ordered them. */
  protected readonly spells = computed(() => this.options().spells);
  /** One row for each circle the character has slots in, then the pact slots. */
  protected readonly slotRows = computed(() => {
    const { usage, pact } = this.slots();
    const rows = usage
      .filter((u) => u.total > 0)
      .map((u) => ({ title: circleLabel(u.level), total: u.total, used: u.used, text: freeText(u.total - u.used, u.total) }));
    if (pact && pact.total > 0) {
      rows.push({
        title: `${circleLabel(pact.slotLevel)} (pacto)`,
        total: pact.total,
        used: pact.used,
        text: freeText(pact.total - pact.used, pact.total),
      });
    }
    return rows;
  });
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
    return {
      left,
      pill: left > 0 ? tight(`Restam ${metersText(left)}`) : 'Sem movimento',
      text:
        left > 0
          ? tight(
              used > 0
                ? `Você já andou ${metersText(used)}. Dá para andar mais ${distanceInSentence(left)}.`
                : `Você ainda não andou. Dá para andar até ${distanceInSentence(left)}.`,
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

  protected spellTags = spellTags;

  /** The line under a spell's name. */
  protected spellLine(s: SpellOption): string {
    return spellLine(s, spellSummary(this.details().get(s.spell?.key ?? '')));
  }

  /** The name a spell goes by on screen and in an accessible name. */
  protected spellName(s: SpellOption): string {
    return s.spell?.namePt || s.spell?.name || '';
  }

  /** A reaction (Escudo) keeps a dashed, disabled "Conjurar": it asks when its trigger happens. */
  protected spellButton(): string {
    return 'Conjurar';
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
