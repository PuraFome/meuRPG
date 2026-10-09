import { CharacterKind, CharacterState } from '../../../core/characters/characters.types';

/**
 * The view-model and port `CampaignCharacters` needs. Phase 2 maps
 * `CharacterService.ListCharacters`'s response onto `CampaignCharactersVm`
 * (plan §4) — nothing below imports from `../../../../gen/...`.
 */

export interface CampaignCharacterListItemVm {
  readonly id: string;
  readonly name: string;
  readonly kind: CharacterKind;
  readonly state: CharacterState;
  /** e.g. "Mago 3" — empty for a basic-sheet NPC. */
  readonly classSummary: string;
  /** Only set for a player character (`Character.player_display_name`). */
  readonly playerDisplayName: string | null;
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
