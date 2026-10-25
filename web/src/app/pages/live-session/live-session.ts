import { Component, TemplateRef, computed, effect, signal, viewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';

import { MapPointKind } from '../../../gen/meurpg/maps/v1/maps_pb';
import { type Encounter, EncounterStatus } from '../../../gen/meurpg/play/v1/combat_pb';
import type { VitalsVm } from '../../core/play/live-session.types';
import { applyVitals } from '../../core/play/vitals';
import { setPageSubject } from '../../core/title/page-title';
import { PHONE_QUERY, mediaQuery } from '../../shared/map-view/media-query';
import { CombatLaunch } from './combat/combat-launch';
import { ConversationPanel } from './conversation/conversation-panel';
import { ConversationSlot } from './conversation/conversation-slot';
import { pageVisible } from './conversation/page-visible';
import type { SessionTab } from './conversation/session-tabs';
import { MasterBoard } from './master-board/master-board';
import type { MapToolCounts } from './session-map/session-map';
import type { TabItem } from '../../shared/tab-list/tab-list';
import { wireLiveSessionEffects } from './live-session-effects';
import { CALLING_NOTICE_MS, LiveSessionEvents } from './live-session-events';
import { LiveSessionParts, injectLiveSessionServices } from './live-session-deps';
import { LIVE_SESSION_IMPORTS } from './live-session-imports';
import { LiveSessionLoader } from './live-session-loader';
import { LiveSessionSheets } from './live-session-sheets';
import { LiveSessionState } from './live-session-state';
import { LiveSessionView } from './live-session-view';

export { CALLING_NOTICE_MS };

/** A tablet: the master's Conversa is a tab of the board, with no rail (under 1024 px) and no pinned nav (from 768 px). */
const NARROW_QUERY = '(min-width: 768px) and (max-width: 1023.98px)';

/**
 * The session page, `/campaigns/:id/session` (MR-011, MR-012, RN-02, RN-06,
 * RN-07; artboards E5-02 to E5-08).
 *
 * It asks for the campaign (its name, and whether the person is its
 * master), then opens the live stream (`LiveStream`: ADR-0005's client
 * rules). On every `ready` it reads the snapshot (`GetLiveSession`), and
 * applies each `vitals_changed` whose revision is newer. The player sees
 * their own character's vitals; the master sees the party, with "Ajustar".
 *
 * The map half (MR-012): the snapshot carries the current map and the
 * shown image (MR-028), and the stream's `current_map_changed`,
 * `map_changed`, `token_moved` and `shown_image_changed` keep them current
 * without a reload. `map_changed` reads the map again; if that answers
 * `not_found`, a player lost sight of it and gets the empty state.
 *
 * The page only composes: its signals are `LiveSessionState`, what it derives is `LiveSessionView`, the reads are
 * `LiveSessionLoader`, the stream's news `LiveSessionEvents` and the sheets it opens `LiveSessionSheets`.
 */
@Component({
  selector: 'app-live-session',
  // The conversation panel is named here, not in the shared list, so the compiler can see that only the
  // `@defer` block uses it and gives it a chunk of its own.
  imports: [...LIVE_SESSION_IMPORTS, ConversationPanel],
  templateUrl: './live-session.html',
  styleUrl: './live-session.scss',
})
export class LiveSession {
  private readonly services = injectLiveSessionServices();
  protected readonly state = new LiveSessionState();
  private readonly parts = new LiveSessionParts(this.services, this.state);
  protected readonly mapState = this.parts.mapState;
  protected readonly trapBoard = this.parts.trapBoard;
  protected readonly toasts = this.parts.toasts;
  protected readonly fog = this.parts.fog;
  protected readonly dark = this.parts.dark;
  protected readonly combat = this.parts.combat;
  protected readonly scene = this.parts.scene;
  protected readonly puzzles = this.parts.puzzles;
  protected readonly notes = this.parts.notes;
  protected readonly freeDice = this.parts.freeDice;
  protected readonly conversation = this.parts.conversation;
  protected readonly view = new LiveSessionView(this.state, this.parts);
  /** "Iniciar combate", from the session bar (the master, outside a combat). */
  protected readonly launch = new CombatLaunch(
    this.services.dialog,
    mediaQuery(PHONE_QUERY),
    () => this.state.campaignId(),
    () => this.mapState,
    (encounter) => this.combatStarted(encounter),
  );
  /** The conversation is a tab of the board and another one is shown (the board tells it, after the slot is made). */
  private readonly chatBehind = signal(false);
  /**
   * The combat's rail draws the conversation ("Turno · Conversa" for the master, "Ações · Conversa" for the player) from 1024px; the
   * page keeps it elsewhere otherwise. The master's is the running combat that is not on a page of its own; the player's, the combat on screen.
   */
  private readonly laptop = mediaQuery('(min-width: 1024px)');
  protected readonly railChat = computed(
    () =>
      this.laptop() &&
      (this.state.isMaster()
        ? this.combat.encounter()?.status === EncounterStatus.ACTIVE && !this.combat.fullPage()
        : this.view.combatOnScreen()),
  );
  /** Where the conversation sits: a column beside the stage, or the phone's "Conversa" tab. */
  protected readonly slot = new ConversationSlot({
    live: computed(() => this.state.phase() === 'live' && this.state.session() !== null),
    phone: mediaQuery(PHONE_QUERY),
    unread: this.conversation.unread,
    railMode: this.railChat,
    pageVisible: pageVisible(this.services.document, this.services.destroyRef),
    behind: this.chatBehind,
    isMaster: this.state.isMaster,
    campaignId: this.state.campaignId,
    ownCharacterId: this.state.ownCharacterId,
    userId: computed(() => {
      const me = this.services.auth.state();
      return me.status === 'signed-in' ? me.user.id : '';
    }),
    stageCharacterIds: computed(() =>
      this.scene
        .stage()
        .map((npc) => npc.characterId)
        .filter((id) => id !== ''),
    ),
  });
  protected readonly ownEffects = this.view.ownEffects;
  /** The master's board outside a combat: "Cena", "Mapa" and "Grupo", in the address (`?vista=`). */
  protected readonly board = new MasterBoard({
    route: this.services.route,
    router: this.services.router,
    sceneOpen: computed(() => this.scene.scene() !== null),
    rolls: computed(() => this.scene.scene()?.rolls.length ?? 0),
    openedHere: computed(() => this.scene.focusNext() === 'title'),
    phone: computed(() => this.slot.phone()),
    narrow: mediaQuery(NARROW_QUERY),
    unread: this.conversation.unread,
  });
  protected readonly behindFollows = effect(() =>
    this.chatBehind.set(this.board.chatTab() && this.board.tab() !== 'conversa'),
  );
  /** The master's board puts the conversation in a rail from 1024px (C3-01): the master's page outside a combat. */
  protected readonly railed = computed(() => this.state.isMaster() && !this.view.combatOnScreen());
  /** The rail's one tab: the tab list draws nothing for it. */
  protected readonly railTabs: readonly TabItem[] = [{ id: 'conversa', label: 'Conversa' }];
  /** The conditions the effects in play put on each character, for the party's rows. */
  protected readonly partyConditions = signal<ReadonlyMap<string, readonly string[]>>(new Map());
  /** "Teste em grupo" of the open scene was pressed: the group check's form is open. */
  protected readonly askingGroup = signal(false);
  /** The map's fold on a phone, open or folded. */
  protected readonly mapFoldOpen = signal(false);
  /** How many encounters the map's battle points keep (the "Encontros" tab counts them). */
  protected readonly encounterCount = signal(0);
  /** What the tab set under the master's map counts. */
  protected readonly mapToolCounts = computed<MapToolCounts>(() => {
    const points = this.mapState.points();
    const kind = (k: MapPointKind) => points.filter((p) => p.kind === k).length;
    return {
      tokens: this.mapState.tokens().length,
      traps: kind(MapPointKind.TRAP),
      treasure: kind(MapPointKind.TREASURE),
      encounters: this.encounterCount(),
    };
  });
  /** The phone's pinned nav: the master's "Cena" and "Grupo" are the board's tabs; "Conversa" opens the conversation. */
  protected readonly phoneTab = computed<SessionTab>(() => {
    if (this.slot.tab() === 'chat') {
      return 'chat';
    }
    return this.railed() && this.board.tab() === 'grupo' ? 'group' : 'scene';
  });
  protected readonly loader = new LiveSessionLoader(this.services, this.state, this.parts);
  protected readonly events: LiveSessionEvents = new LiveSessionEvents(
    this.services,
    this.state,
    this.parts,
    this.view,
    this.loader,
  );
  protected readonly sheets = new LiveSessionSheets(
    this.services,
    this.state,
    this.parts,
    this.view,
    this.loader,
  );

  /** The app bar's "Anotações" button, drawn by the bar (see `SessionNotes`). */
  protected readonly notesBar = viewChild<TemplateRef<unknown>>('notesBar');

  constructor() {
    // The tab's title carries the campaign's name once it is loaded.
    setPageSubject(() => this.state.campaign()?.name);
    const { route } = this.services;
    route.paramMap.pipe(takeUntilDestroyed()).subscribe((params) => {
      const id = params.get('id');
      if (id) {
        this.load(id);
      }
    });
    route.queryParamMap
      .pipe(takeUntilDestroyed())
      .subscribe((params) => this.state.openPuzzle.set(params.get('puzzle')));
    this.services.destroyRef.onDestroy(() => {
      this.parts.toasts.clear();
      this.events.dispose();
      this.loader.dispose();
      this.services.sessionNotes.bar.set(null);
    });
    wireLiveSessionEffects(
      this.services,
      this.state,
      this.parts,
      this.view,
      this.slot,
      this.notesBar,
      (id) => this.load(id),
    );
  }

  /** A tap on the phone's nav: the master's "Cena" and "Grupo" choose a tab of the board, the rest is the conversation slot's. */
  protected phoneNav(tab: SessionTab): void {
    if (this.railed() && tab !== 'chat') {
      this.board.choose(tab === 'group' ? 'grupo' : 'cena');
      this.slot.choose('scene');
      return;
    }
    this.slot.choose(tab === 'group' ? 'scene' : tab);
  }

  /** What a screen reader hears of the "Anotações" button when the visible words are cut short on a narrow phone: how many clues are new. */
  protected notesAria(fresh: number): string {
    return fresh > 0 ? `Anotações, ${fresh} ${fresh === 1 ? 'nova' : 'novas'}` : 'Anotações';
  }

  private load(campaignId: string): void {
    this.events.open(campaignId, this.loader.begin(campaignId));
  }

  protected retry(): void {
    this.load(this.state.campaignId());
  }

  protected sessionEndedHere(): void {
    this.loader.ended();
  }

  /** "Fechar" on the players' "Destaques" card: it closes the summary under it too, which has no button of its own while the card is up. */
  protected closeHighlights(): void {
    const e = this.view.highlightsFor();
    if (e) {
      this.state.highlightsClosed.set(e.id);
      this.combat.dismissEnded();
    }
  }

  /** "Iniciar combate" answered: the combat is on screen at once. */
  protected combatStarted(encounter: Encounter): void {
    this.combat.apply(encounter);
  }

  /** A rest or a spent hit die changed these characters: their new numbers, as the server answered (the stream says the same to the other screens). */
  protected takeVitals(taken: readonly VitalsVm[]): void {
    this.state.vitals.update((list) => taken.reduce((all, v) => applyVitals(all, v), list));
    this.state.restsTick.update((n) => n + 1);
  }

  /** "Ver como": the character the master looks as; the line about that view clears until the next read. */
  protected setViewAs(id: string | null): void {
    this.state.viewAs.set(id);
    this.state.viewNote.set('');
    this.state.viewGone.set('');
  }

  /** The character he looked as died or left the campaign: back to "Todos", and say so. */
  protected viewAsGone(): void {
    this.state.viewAs.set(null);
    this.state.viewNote.set('');
    this.state.viewGone.set('Esse personagem morreu ou saiu da campanha. Voltamos para “Todos”.');
  }
}
