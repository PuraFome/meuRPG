/**
 * The view-models and the port of the session page (`/campanhas/:id/sessao`,
 * MR-011, MR-012, RN-02). Nothing here imports generated code: the
 * `LiveSessionSourceLive` maps `PlayService`, `CharacterService` and
 * `CampaignService` onto these shapes, and the page, its children and their
 * tests only see these.
 */

import type { DiceMode, DicePreference } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { CombatantMove, TurnChange } from '../../core/combat/combat-state';

/** One spell level's slots (`SpellSlotUsage`). */
export interface SlotUsageVm {
  readonly level: number;
  readonly total: number;
  readonly used: number;
}

/** A warlock's pact magic slots (`PactSlotUsage`): all of one level. */
export interface PactSlotsVm {
  readonly slotLevel: number;
  readonly total: number;
  readonly used: number;
}

/** A player character's live numbers (`CharacterVitals`, RN-02). */
export interface VitalsVm {
  readonly characterId: string;
  readonly name: string;
  /** Empty when the player deleted their account (RN-16). */
  readonly playerUserId: string;
  readonly hitPointsCurrent: number;
  readonly hitPointsMax: number;
  readonly hitPointsTemporary: number;
  /** Only the levels with slots, lowest first. */
  readonly spellSlots: readonly SlotUsageVm[];
  readonly pactSlots: PactSlotsVm | null;
  /** "3d6", or "2d10 + 1d8" for a multiclass character. */
  readonly hitDice: string;
  readonly hitDiceTotal: number;
  readonly hitDiceUsed: number;
  /** Of two copies of the same character's vitals, the larger is newer. */
  readonly revision: number;
}

/** The open game session being watched. */
export interface LiveSessionVm {
  readonly sessionId: string;
  readonly sessionNumber: number;
  readonly startedAt: Date;
}

/** The gallery image the master shows the players (MR-028, `ShownImage`):
 * what a player's block needs, so the frame is reserved from the size
 * before the bytes arrive. */
export interface ShownImageVm {
  readonly id: string;
  /** The image's name in the gallery: the caption. */
  readonly name: string;
  readonly width: number;
  readonly height: number;
  readonly url: string;
}

/** `GetLiveSession`: the session and the vitals the caller may see (the
 * master: every living player character; a player: only their own), the
 * session's current map and the image being shown. */
export interface LiveSnapshotVm {
  readonly session: LiveSessionVm;
  readonly vitals: readonly VitalsVm[];
  /** The current map's ID, or `null` while the master has not chosen one. */
  readonly currentMapId: string | null;
  readonly shownImage: ShownImageVm | null;
  /** The master's "Deixar com os jogadores" switch for the image shown;
   * always false for a player (`shown_image_keep`). */
  readonly shownImageKeep: boolean;
}

/** One event of `WatchGameSession`. */
export type LiveEventVm =
  | { readonly kind: 'ready' }
  | { readonly kind: 'heartbeat' }
  | { readonly kind: 'vitals'; readonly vitals: VitalsVm }
  | { readonly kind: 'ended' }
  /** `current_map_changed`; `mapId` is `null` when the map was cleared. */
  | { readonly kind: 'currentMap'; readonly mapId: string | null }
  /** `map_changed`: read the map again. */
  | { readonly kind: 'mapChanged'; readonly mapId: string }
  | {
      readonly kind: 'tokenMoved';
      readonly mapId: string;
      readonly characterId: string;
      readonly xBp: number;
      readonly yBp: number;
    }
  /** `shown_image_changed`; `image` is `null` when it stopped. */
  | { readonly kind: 'shownImage'; readonly image: ShownImageVm | null }
  /** `left_images_changed`: read the left images again. */
  | { readonly kind: 'leftImages' }
  /** `encounter_changed`: the combat changed, read it again if `revision`
   * is newer than the one on screen. */
  | { readonly kind: 'encounterChanged'; readonly encounterId: string; readonly revision: number }
  /** `turn_changed`, as this member may see it. */
  | ({ readonly kind: 'turnChanged' } & TurnChange)
  /** `combatant_moved`. */
  | ({ readonly kind: 'combatantMoved' } & CombatantMove)
  /** `combat_log_changed`: read the combat log again. */
  | { readonly kind: 'combatLogChanged' }
  /** `xp_changed`: an award, an undo or a milestone; read the XP again. */
  | { readonly kind: 'xpChanged' }
  /** `scene_changed`: the open scene changed, read it again. */
  | { readonly kind: 'sceneChanged' }
  /** `scene_check_rolled`: a check was rolled in it, read it again. */
  | { readonly kind: 'sceneCheckRolled' }
  /** `stage_changed` (MR-031): an NPC came in or went out, or the speaker
   * changed. It names nobody: the page reads the open scene again. */
  | { readonly kind: 'stageChanged' };

/**
 * What a failed call means for the page, from its Connect code and typed
 * detail (never from the message):
 * - `no-access`: `not_found` — not a member (or pending), or no such
 *   campaign; for an adjustment, the character isn't a living player
 *   character any more;
 * - `no-session`: `failed_precondition` with `NO_OPEN_SESSION`;
 * - `signed-out`: `unauthenticated`;
 * - `invalid`: `invalid_argument` (a value outside 0 to its maximum);
 * - `forbidden`: `permission_denied` (a player calling a master's method);
 * - `transient`: anything else (the network, `unavailable`, `aborted`):
 *   trying again may work.
 */
export type LiveErrorKind =
  'no-access' | 'no-session' | 'signed-out' | 'invalid' | 'forbidden' | 'transient';

/** `GetCampaign`, for the lead and to know who is looking. */
export interface CampaignInfoVm {
  readonly name: string;
  readonly isMaster: boolean;
  /** A pending member (RN-15): gets the same "Peça um convite" page. */
  readonly awaitingApproval: boolean;
  /** How the campaign's players roll dice (RN-18) and the caller's own
   * choice: the initiative screen offers the ways they allow. */
  readonly diceMode: DiceMode;
  readonly dicePreference: DicePreference;
}

/** From the player's own sheet (`GetCharacter`), what the vitals block
 * shows next to the live numbers. */
export interface PlayerSheetVm {
  /** `DerivedSheet.armor_class`; `null` for a sheet without it. */
  readonly armorClass: number | null;
  /** "Mago 3, Gnomo das Rochas". */
  readonly summary: string;
}

/** From `ListCharacters`, what the master's "Grupo" row shows under a
 * character's name. */
export interface PartyMemberInfoVm {
  /** "Mago 3". */
  readonly classSummary: string;
  /** The player's display name, or `null` when they have none. */
  readonly playerName: string | null;
}

/** The master's correction: every value set replaces the current one. */
export interface VitalsChange {
  readonly hitPointsCurrent?: number;
  readonly hitPointsTemporary?: number;
  readonly spellSlotsUsed?: readonly { readonly level: number; readonly used: number }[];
  readonly pactSlotsUsed?: number;
  readonly hitDiceUsed?: number;
}

/**
 * The port the session page depends on, provided at the route level by
 * `LiveSessionSourceLive` (`live-session.routes.ts`), so the generated
 * clients stay in this page's lazy chunk.
 */
export abstract class LiveSessionSource {
  abstract getCampaign(campaignId: string): Promise<CampaignInfoVm>;
  /** `WatchGameSession`. Aborting `signal` closes the stream. */
  abstract watch(campaignId: string, signal: AbortSignal): AsyncIterable<LiveEventVm>;
  abstract getLiveSession(campaignId: string): Promise<LiveSnapshotVm>;
  abstract adjustVitals(
    campaignId: string,
    characterId: string,
    idempotencyKey: string,
    change: VitalsChange,
  ): Promise<VitalsVm>;
  abstract endSession(campaignId: string, sessionId: string): Promise<void>;
  /** `SetCurrentMap`: the map the session shows (`null` clears it). */
  abstract setCurrentMap(campaignId: string, mapId: string | null): Promise<string | null>;
  /** `SetShownImage`: show a gallery image to the players (`null` stops).
   * `keep` is the "Deixar com os jogadores" switch for that image. */
  abstract setShownImage(
    campaignId: string,
    imageId: string | null,
    keep?: boolean,
  ): Promise<ShownImageVm | null>;
  /** `ListLeftImages`: the images the master left with the players. */
  abstract listLeftImages(campaignId: string): Promise<readonly ShownImageVm[]>;
  /** `TakeBackLeftImage`: the master takes an image back ("Tirar"). */
  abstract takeBackLeftImage(campaignId: string, imageId: string): Promise<void>;
  abstract getPlayerSheet(campaignId: string, characterId: string): Promise<PlayerSheetVm>;
  abstract getPartyInfo(campaignId: string): Promise<ReadonlyMap<string, PartyMemberInfoVm>>;
  abstract classifyError(err: unknown): LiveErrorKind;
}
