import { Component, TemplateRef, computed, viewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';

import type { Encounter } from '../../../gen/meurpg/play/v1/combat_pb';
import type { VitalsVm } from '../../core/play/live-session.types';
import { applyVitals } from '../../core/play/vitals';
import { setPageSubject } from '../../core/title/page-title';
import { PHONE_QUERY, mediaQuery } from '../../shared/map-view/media-query';
import { ConversationSlot } from './conversation/conversation-slot';
import { pageVisible } from './conversation/page-visible';
import { wireLiveSessionEffects } from './live-session-effects';
import { CALLING_NOTICE_MS, LiveSessionEvents } from './live-session-events';
import { LiveSessionParts, injectLiveSessionServices } from './live-session-deps';
import { LIVE_SESSION_IMPORTS } from './live-session-imports';
import { LiveSessionLoader } from './live-session-loader';
import { LiveSessionSheets } from './live-session-sheets';
import { LiveSessionState } from './live-session-state';
import { LiveSessionView } from './live-session-view';

export { CALLING_NOTICE_MS };

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
  imports: LIVE_SESSION_IMPORTS,
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
  protected readonly combat = this.parts.combat;
  protected readonly scene = this.parts.scene;
  protected readonly puzzles = this.parts.puzzles;
  protected readonly notes = this.parts.notes;
  protected readonly conversation = this.parts.conversation;
  protected readonly view = new LiveSessionView(this.state, this.parts);
  /** Where the conversation sits: a column beside the stage, or the phone's "Conversa" tab. */
  protected readonly slot = new ConversationSlot({
    live: computed(() => this.state.phase() === 'live' && this.state.session() !== null),
    phone: mediaQuery(PHONE_QUERY),
    pageVisible: pageVisible(this.services.document, this.services.destroyRef),
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
