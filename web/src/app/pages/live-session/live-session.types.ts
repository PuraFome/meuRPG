/**
 * The view-models and the port of the session page (`/campanhas/:id/sessao`,
 * MR-011, MR-012, RN-02). Nothing here imports generated code: the
 * `LiveSessionSourceLive` maps `PlayService`, `CharacterService` and
 * `CampaignService` onto these shapes, and the page, its children and their
 * tests only see these.
 */

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

/** `GetLiveSession`: the session and the vitals the caller may see (the
 * master: every living player character; a player: only their own). */
export interface LiveSnapshotVm {
  readonly session: LiveSessionVm;
  readonly vitals: readonly VitalsVm[];
}

/** One event of `WatchGameSession`. */
export type LiveEventVm =
  | { readonly kind: 'ready' }
  | { readonly kind: 'heartbeat' }
  | { readonly kind: 'vitals'; readonly vitals: VitalsVm }
  | { readonly kind: 'ended' };

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
  abstract getPlayerSheet(campaignId: string, characterId: string): Promise<PlayerSheetVm>;
  abstract getPartyInfo(campaignId: string): Promise<ReadonlyMap<string, PartyMemberInfoVm>>;
  abstract classifyError(err: unknown): LiveErrorKind;
}
