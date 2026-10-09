import {
  CharacterKind,
  CharacterState,
  ReviewStatus,
} from '../../../core/characters/characters.types';

/**
 * The view-model and port `CampaignCharacters` needs. Phase 2 maps
 * `CharacterService.ListCharacters`'s response onto `CampaignCharactersVm`
 * (plan §4) — nothing below imports from `../../../../gen/...`.
 */

/** Where a character's claim link stands (`ClaimState`): what the master's "Personagens reservados" row says. */
export type ClaimStateVm = 'none' | 'sent' | 'expired' | 'revoked' | 'used';

/** A reserved character's link, or a claimed character's origin (MR-049). */
export interface ClaimVm {
  readonly state: ClaimStateVm;
  /** When the link stops working: set for `sent` and `expired`. */
  readonly expiresAt: Date | null;
  /** Who took the character: the player's display name, `null` when they have none (or deleted the account). */
  readonly claimedBy: string | null;
}

export interface CampaignCharacterListItemVm {
  readonly id: string;
  readonly name: string;
  readonly kind: CharacterKind;
  readonly state: CharacterState;
  /** e.g. "Mago 3" — empty for a basic-sheet NPC. */
  readonly classSummary: string;
  /** Only set for a player character (`Character.player_display_name`). */
  readonly playerDisplayName: string | null;
  /** `CharacterSummary.review_status`: only the master and the owner get it, and only a pending character has one. */
  readonly reviewStatus?: ReviewStatus | null;
  /** The race in Portuguese, "Halfling": empty for a basic-sheet NPC. */
  readonly raceName?: string;
  /** A reserved character: made by the master for a player to claim, with no owner (MR-049). Master only. */
  readonly reserved?: boolean;
  /** The claim link's state, for a reserved character or one a player claimed through a link; `null` for any other. */
  readonly claim?: ClaimVm | null;
}

/** A living player character with choices still open (PM-05), as the master's list shows it. */
export interface OpenChoicesVm {
  readonly characterId: string;
  /** How many selections are open. */
  readonly count: number;
  /** What is open, one label for each choice ("Estilo de Luta (Patrulheiro, nível 2)"). */
  readonly labels: readonly string[];
}

export interface CampaignCharactersVm {
  /** Master: every player character in the campaign, those waiting for
   * approval included (state `'pending'`, MR-024). Player: only their own
   * (`ListCharacters`'s authz row, plan §4). */
  readonly playerCharacters: readonly CampaignCharacterListItemVm[];
  /** Master only — always empty for a player (they never see NPCs, MR-005). */
  readonly npcs: readonly CampaignCharacterListItemVm[];
  /** Player only: whether they already have a living character in this
   * campaign — drives the "Criar meu personagem" call to action (RN-03). */
  readonly hasLivingCharacter: boolean;
}

/**
 * The port `CampaignCharacters` depends on, provided at the route level for
 * `/campaigns/:id` (`campaign-detail.routes.ts`) by
 * `CampaignCharactersSourceLive`, which wraps the generated `CharacterService`
 * client. No root fallback: a route reached without this provider fails
 * loudly (NG0201) instead of silently degrading — see `app.config.ts`.
 */
export abstract class CampaignCharactersSource {
  abstract listCharacters(campaignId: string): Promise<CampaignCharactersVm>;
  /** Master only (`GetCampaignOpenChoices`): the player characters with choices still open. A player is refused. */
  abstract openChoices(campaignId: string): Promise<readonly OpenChoicesVm[]>;
}
