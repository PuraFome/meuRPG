import type { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type {
  InspirationOffer,
  LuckyOffer,
  PendingDamage,
  TargetInReach,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { Attack, BonusAttackRule } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import type { CombatState } from '../../../../core/combat/combat-state';

/** What the page hands the attack sheet. */
export interface AttackSheetData {
  readonly campaignId: string;
  readonly encounterId: string;
  readonly attackerId: string;
  readonly round: number;
  readonly attack: Attack;
  /** The attack's targets from `GetTurnOptions`, in turn order. */
  readonly targets: readonly TargetInReach[];
  /** The weapon attack of the extra action an effect gives (Velocidade): it spends that action, not the Attack action. */
  readonly useExtraAction?: boolean;
  readonly diceMode: DiceMode;
  readonly preference: DicePreference;
  /** Where each answer's combat goes (the page's copy of the combat). */
  readonly state: CombatState;
  /** An opportunity attack: a melee attack off turn that spends the reaction
   * instead of the action (`RollAttack.as_reaction`). */
  readonly asReaction?: boolean;
  /** The master rolls it (for an NPC): any roll mode, with no need to ask. */
  readonly master?: boolean;
  /** Extra Attack: how many attacks of the Attack action remain before this
   * one, and how many it makes. */
  readonly attacksLeft?: number;
  readonly attacksPerAction?: number;
  /** A bonus action attack (off hand, Artes Marciais, Rajada de Golpes): it
   * spends the bonus action, not the action; a Rajada de Golpes strike counts
   * down the `bonusAttacksLeft` the options had. */
  readonly bonusRule?: BonusAttackRule;
  readonly bonusAttacksLeft?: number;
  /** A cantrip with several beams (Eldritch Blast): how many were still to fire before this one. */
  readonly beamsLeft?: number;
  /** The answer to an opportunity offer (E9-13): the target is the mover (so the
   * sheet starts at "Rolar"), the reach is not checked (the attack comes right
   * before it leaves), and the roll names the offer. `byMaster` when the master
   * rolls it for an NPC. */
  readonly opportunity?: {
    readonly offerId: string;
    readonly targetId: string;
    readonly targetLabel: string;
    readonly byMaster: boolean;
  };
  /** A hit whose damage was never rolled (the sheet was closed): the sheet
   * opens at "Dano" with it. */
  readonly resume?: { readonly pending: PendingDamage; readonly targetLabel: string };
  /** A roll held for the Bardic Inspiration question (the player left the sheet, or read the screen again): the sheet
   * opens at the question, with the d20 that was rolled. */
  readonly inspiration?: { readonly offer: InspirationOffer; readonly targetLabel: string };
  /** The same for a roll held for the halfling's Lucky question ("Tirou 1: rolar de novo?"), which comes first. */
  readonly lucky?: { readonly offer: LuckyOffer; readonly targetLabel: string };
  /** A roll of this sheet was held for a question (Lucky or Bardic Inspiration): the page learns the hold, so it does not
   * open the question a second time. */
  readonly onHeld?: (holdId: string) => void;
  /** The monk's throw back after Defletir Projéteis: the reaction window that caught the missile
   * (`RollAttack.catch_window_id`). It is part of the same reaction, so it asks nothing more. */
  readonly catchWindowId?: string;
}
