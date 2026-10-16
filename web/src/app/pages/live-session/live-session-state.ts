import { computed, signal } from '@angular/core';

import type { Map as MapMessage } from '../../../gen/meurpg/maps/v1/maps_pb';
import type { CluePlayer } from '../../core/maps/scene-clues';
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
import type { PickRow } from '../../shared/person-pick/person-pick';
import type { ViewAsPerson } from '../../shared/fog-map/view-as-list';

/**
 * Where the page stands. `live` covers reconnecting too: the numbers stay
 * on screen, readable, while the status line says "Reconectando…".
 */
export type Phase = 'loading' | 'live' | 'no-access' | 'no-session' | 'ended' | 'error';

/** What the session page holds in signals, and the counters of its reads (the page's own state, with no service in it). */
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

  /** Goes up when what a player sees may have changed: the master's "Ver como" reads again (his stream never hears `vision_changed`). */
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
  /** The player's bonuses for "Procurar armadilhas" (the derived sheet). */
  readonly trapSkills = computed(() => this.playerSheet()?.skills ?? null);
  /** The players' names by character, for "de Caio" on the session's highlights (master only). */
  readonly playerNames = computed(
    () =>
      new Map(
        [...this.partyInfo()].flatMap(([id, p]) =>
          p.playerName ? [[id, p.playerName] as const] : [],
        ),
      ),
  );
  /** The master's "Ver como" list: the living player characters, with their players. */
  readonly viewAsPeople = computed<readonly ViewAsPerson[]>(() =>
    this.vitals().map((v) => ({
      id: v.characterId,
      name: v.name,
      sub: this.partyInfo().get(v.characterId)?.playerName ?? '',
    })),
  );
  /** The master's player characters, for who gets a clue (a deleted account has no player). */
  readonly cluePlayers = computed<readonly CluePlayer[]>(() =>
    this.vitals()
      .filter((v) => v.playerUserId !== '')
      .map((v) => ({
        id: v.characterId,
        name: v.name,
        playerName: this.partyInfo().get(v.characterId)?.playerName ?? '',
      })),
  );
  /** The living player characters as rows to pick from ("Quem encontrou"): the class and the player under the name. */
  readonly finderRows = computed<readonly PickRow[]>(() =>
    this.vitals()
      .filter((v) => v.playerUserId !== '')
      .map((v) => {
        const info = this.partyInfo().get(v.characterId);
        return {
          id: v.characterId,
          name: v.name,
          sub: [info?.classSummary ?? '', info?.playerName ? `de ${info.playerName}` : '']
            .filter(Boolean)
            .join(', '),
        };
      }),
  );

  /** The campaign of the current load; `null` once it turned out the page can't show it (no access, a pending member, an error). */
  campaignLoad: Promise<CampaignInfoVm | null> = Promise.resolve(null);
  loadedSheetFor: string | null = null;
  partyInfoIds = new Set<string>();
  /** Moves with each load: an answer that comes for an older one is dropped. */
  generation = 0;
  /** Moves with each read of the images left with the players, and with each one taken back here. */
  leftSeq = 0;
  /** Counts the stream's news of the current map and of the shown image: a snapshot that began before one of them
   * does not overwrite it. */
  mapEvents = 0;
  imageEvents = 0;
  callingTimer: ReturnType<typeof setTimeout> | undefined;
  visionTimer: ReturnType<typeof setTimeout> | undefined;
  /** Whether the open-sessions list named the campaign the last time the waiting effect looked. */
  listedCampaign: { id: string; listed: boolean } = { id: '', listed: false };
}
