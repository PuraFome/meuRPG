import type { RollDie } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import type { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { ReactionWindow } from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { CombatState } from '../../../../core/combat/combat-state';

/** What the page hands the reaction sheet. */
export interface ReactionSheetData {
  readonly campaignId: string;
  readonly encounterId: string;
  /** The window to answer, as the server sent it (the sheet keeps its prompt for the result). */
  readonly window: ReactionWindow;
  readonly round: number;
  readonly usage: readonly {
    readonly level: number;
    readonly total: number;
    readonly used: number;
  }[];
  readonly pact: {
    readonly slotLevel: number;
    readonly total: number;
    readonly used: number;
  } | null;
  readonly state: CombatState;
  /** The player's armor class from their own sheet, for "Sua CA é 18"; `null` when unknown. */
  readonly armorClass: number | null;
  readonly diceMode: DiceMode;
  readonly preference: DicePreference;
  /** The table dice the character may add to a roll (`DerivedSheet.roll_dice`); the concentration save offers those that fit a Constitution save. */
  readonly rollDice?: readonly RollDie[];
}

/** What closes the sheet: the monk asked to throw the missile back (the page opens the attack with this window). */
export interface ReactionSheetResult {
  readonly throwWindowId: string;
}
