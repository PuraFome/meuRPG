import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { type Combatant, CombatantKind, type Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { article } from '../../../../core/combat/combat-log';
import { combatantInitial } from '../../../../core/combat/combat-view';
import { groupFeminine, groupName, isCreature } from '../../../../core/combat/creature-names';
import { jointTurn, listNames, partEconomy } from '../../../../core/combat/joint-turn';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';
import { PartState } from './part-state';

/**
 * The player's card about the others of a joint turn (E8-01, state 5), read
 * only, with a word and an icon for every state:
 *   - "others": on their own turn, "O que a Brisa ainda tem";
 *   - "summary": after they ended, "Neste turno conjunto", every part with what
 *     it still has and what it spent;
 *   - "outside": a player whose group is not on turn, "Quem já encerrou", the
 *     members and their state only.
 * A member's economy comes only for the player characters of the group, so an
 * NPC shows its state alone (RN-20).
 */
@Component({
  selector: 'app-joint-others',
  imports: [CombatantToken, MatIconModule, PartState],
  templateUrl: './joint-others.html',
  styleUrl: './joint-others.scss',
})
export class JointOthers {
  readonly encounter = input.required<Encounter>();
  readonly mode = input.required<'others' | 'summary' | 'outside'>();

  protected readonly Player = CombatantKind.PLAYER;
  protected readonly Creature = CombatantKind.CREATURE;
  protected readonly joint = computed(() => jointTurn(this.encounter()));
  /** The members the card draws: the others while it is their turn, everyone otherwise. */
  protected readonly blocks = computed(() => {
    const j = this.joint();
    if (!j) {
      return [];
    }
    if (this.mode() === 'outside') {
      // Who ended is news; who still acts is one line ("Faltam os 2 Lobos"), never a row each saying "Ainda age".
      return j.members.filter((m) => m.turnPartEnded);
    }
    return this.mode() === 'others' ? j.members.filter((m) => !m.mine) : j.members;
  });
  /** "Faltam os 2 Lobos", "Falta a Brisa": the members that still act, for the viewer outside the group. */
  protected readonly missing = computed(() => {
    const j = this.joint();
    const still = j ? j.members.filter((m) => !m.turnPartEnded) : [];
    if (this.mode() !== 'outside' || still.length === 0) {
      return '';
    }
    const who =
      still.length > 1 && still.every(isCreature) && new Set(still.map((m) => m.monsterKey)).size === 1
        ? `${groupFeminine(still) ? 'as' : 'os'} ${still.length} ${groupName(still).split(' ')[0]}`
        : listNames(still.map((m) => `${article(m.label)} ${m.label}`));
    return `${still.length === 1 ? 'Falta' : 'Faltam'} ${who}`;
  });
  protected readonly title = computed(() => {
    switch (this.mode()) {
      case 'others': {
        const others = this.blocks();
        return others.length === 1 ? `O que ${article(others[0].label)} ${others[0].label} ainda tem` : 'O que os outros ainda têm';
      }
      case 'summary':
        return 'Neste turno conjunto';
      default:
        return this.blocks().length === 0 ? this.missing() : 'Quem já encerrou';
    }
  });
  /** The economy of a member, only for a player's character when the card shows it. */
  protected readonly detailed = computed(() => this.mode() !== 'outside');

  protected initial(c: Combatant): string {
    return combatantInitial(c.label);
  }

  protected showsEconomy(c: Combatant): boolean {
    // Only a player's character, or the viewer's own creature: nobody else's creature has its economy sent.
    return this.detailed() && (c.kind === CombatantKind.PLAYER || c.controlledByMe);
  }

  protected economy(c: Combatant) {
    return partEconomy(c);
  }
}
