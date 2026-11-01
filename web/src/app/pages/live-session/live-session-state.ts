import { computed, signal } from '@angular/core';

import type { Map as MapMessage } from '../../../gen/meurpg/maps/v1/maps_pb';
import type {
  CampaignInfoVm,
  LiveSessionVm,
  PartyMemberInfoVm,
  PlayerSheetVm,
  ShownImageVm,
  VitalsVm,
} from '../../core/play/live-session.types';
import type { LiveStream } from '../../core/play/live-stream';
import { betterArmorClass } from '../../core/play/vitals';

/**
 * Where the page stands. `live` covers reconnecting too: the numbers stay
 * on screen, readable, while the status line says "Reconectando…".
 */
export type Phase = 'loading' | 'live' | 'no-access' | 'no-session' | 'ended' | 'error';

/** What the session page holds in signals (the page's own state, with no service in it). */
export class LiveSessionState {
  readonly campaignId = signal('');
  readonly phase = signal<Phase>('loading');
  readonly campaign = signal<CampaignInfoVm | null>(null);
  readonly session = signal<LiveSessionVm | null>(null);
  readonly vitals = signal<readonly VitalsVm[]>([]);
  readonly playerSheet = signal<PlayerSheetVm | null>(null);
  /** The armor class of the Armadura Arcana that lasts on the player's character (0: none), which the sheet does not count. */
  readonly mageArmorAc = signal(0);
  /** The armor class of the beast's book while the druid is a beast (the vitals show it in place of the druid's own). */
  readonly beastAc = signal<number | null>(null);
  readonly partyInfo = signal<ReadonlyMap<string, PartyMemberInfoVm>>(new Map());

  /** The session's current map, the image on show, and the map itself. */
  readonly currentMapId = signal<string | null>(null);
  readonly shownImage = signal<ShownImageVm | null>(null);
  /** The master's "Deixar com os jogadores" switch for the image on show. */
  readonly shownKeep = signal(false);
  /** The images the master left with the players (MR-028). */
  readonly leftImages = signal<readonly ShownImageVm[]>([]);
  /** What a player's screen reader hears when the master shows or stops. */
  readonly shownNotice = signal('');
  readonly campaignMaps = signal<readonly MapMessage[]>([]);

  /** Goes up when what a player sees may have changed: the master's "Ver como" reads again (the master hears `vision_changed` too, for his light warning). */
  readonly visionTick = signal(0);
  /** The character the master looks as ("Ver como"), the line about that view, and why it went back to "Todos". */
  readonly viewAs = signal<string | null>(null);
  readonly viewNote = signal('');
  readonly viewGone = signal('');
  /** Bumped when the stream says a character's creatures changed. */
  readonly creaturesTick = signal(0);
  /** Bumped when the casts of Revivify outside a combat may have changed: the master's question reads them again. */
  readonly revivifyTick = signal(0);
  /** This player's character lives again (`character_revived`): its id while "Você voltou à vida" is on the page. */
  readonly revivedId = signal('');
  /** Who cast Revivificar on it, when the table's log says; empty otherwise. */
  readonly revivedCaster = signal('');
  /** The master is calling this player (`calling_you`): "O mestre está chamando você." is on the page. */
  readonly calling = signal(false);
  /** Counts the `spell_casts_changed` hints (and each `ready`): the casts panel reads again when it moves. */
  readonly castsTick = signal(0);
  /** Counts the `group_check_changed` hints (and each `ready`): the group check's card reads again when it moves (W7-X). */
  readonly groupCheckTick = signal(0);
  /** A group check waits for this player's roll (the card says "Rolar"): the scene's "Ações" tab lights with it. */
  readonly groupWaiting = signal(false);
  /** Counts the `held_rolls_changed` hints (and each `ready`): the master's pending questions read again when it moves. */
  readonly heldRollsTick = signal(0);
  /** Goes up when a rest was taken: the effects on the characters are read again with the spell casts. */
  readonly restsTick = signal(0);
  readonly familiarNameNow = signal<string | null>(null);
  /** The puzzle a player has open (`?puzzle=ID`), in the place of the board; the master plays none. */
  readonly openPuzzle = signal<string | null>(null);
  /** The ended combat whose "Destaques" card this player closed (MR-032). */
  readonly highlightsClosed = signal<string | null>(null);
  readonly stream = signal<LiveStream | null>(null);

  readonly connection = computed(() => this.stream()?.status() ?? 'connecting');
  readonly lastUpdate = computed(() => this.stream()?.lastMessageAt() ?? null);
  readonly isMaster = computed(() => this.campaign()?.isMaster ?? false);
  /** The player's own character: the only one the server sends them. */
  readonly ownVitals = computed(() => this.vitals()[0] ?? null);
  readonly ownCharacterId = computed(() => this.vitals().at(0)?.characterId ?? '');
  readonly seeingFamiliar = computed(() => !!this.vitals().at(0)?.familiarSight);
  readonly effectsTick = computed(() => this.castsTick() + this.restsTick());
  /** The player's sheet with the armor class the combat uses: the better of the sheet's and the Armadura Arcana's. */
  readonly ownSheet = computed(() => {
    const sheet = this.playerSheet();
    const mage = this.mageArmorAc();
    return sheet && sheet.armorClass !== null && mage > sheet.armorClass
      ? { ...sheet, armorClass: mage }
      : sheet;
  });
  /** The armor class of the player's own character: the sheet's, or the base an effect gives (Armadura Arcana) when better. */
  readonly ownArmorClass = computed(() =>
    betterArmorClass(this.playerSheet()?.armorClass ?? null, this.ownVitals()?.armorClassBase),
  );
}
