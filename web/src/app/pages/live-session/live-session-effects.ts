import { type Signal, type TemplateRef, effect, untracked } from '@angular/core';

import { foundToastTitle } from '../../core/traps/treasure-text';
import { firstLine } from '../../core/traps/trap-text';
import type { ConversationSlot } from './conversation/conversation-slot';
import type { LiveSessionParts, LiveSessionServices } from './live-session-deps';
import type { LiveSessionState } from './live-session-state';
import type { LiveSessionView } from './live-session-view';

/** The page's effects; call it in the injection context (the component's constructor). `notesBar` is the app bar's
 * "Anotações" button, drawn by the bar (see `SessionNotes`); `load` starts a load of a campaign (the stream included). */
export function wireLiveSessionEffects(
  services: LiveSessionServices,
  state: LiveSessionState,
  parts: LiveSessionParts,
  view: LiveSessionView,
  slot: ConversationSlot,
  notesBar: Signal<TemplateRef<unknown> | undefined>,
  load: (campaignId: string) => void,
): void {
  // Whether the open-sessions list named the campaign the last time the waiting effect looked.
  let listedCampaign = { id: '', listed: false };

  // A player's session page hands the app bar the "Anotações" button (the
  // bar only draws it); the master has no notes.
  effect(() => {
    const show = state.phase() === 'live' && !state.isMaster();
    const bar = notesBar();
    untracked(() => services.sessionNotes.bar.set(show ? (bar ?? null) : null));
  });

  // The conversation counts the lines that come while it is not on the reader's screen, and clears them when it is.
  effect(() => {
    const onScreen = slot.onScreen();
    untracked(() => parts.conversation.setVisible(onScreen));
  });

  // A treasure the master marks found while the player looks at the map: a toast (E9-09 4). The first read
  // of a map is the baseline, so a treasure found before they got here is not news.
  effect(() => {
    const map = parts.mapState.status() === 'ready' ? parts.mapState.map() : null;
    const points = parts.mapState.points();
    untracked(() => {
      const fresh = parts.treasureWatch.newlyFound(map?.id ?? null, points);
      if (!state.isMaster()) {
        for (const p of fresh) {
          parts.toasts.push('inventory_2', foundToastTitle(p), firstLine(p.description));
        }
      }
    });
  });

  // A map with the fog of war on: the vision is read when it is the current one, and closed when it is not.
  // It depends on the map's ID and its fog flag, not on each reading of the map: a hint costs one read.
  effect(() => {
    const id = view.fogMapId();
    untracked(() => {
      // Another map: "Ver como" starts over at "Todos".
      state.viewAs.set(null);
      state.viewGone.set('');
      void parts.fog.open(id);
    });
  });

  // The familiar a player looks through, for the name on the card "O que o Nanquim vê".
  effect(() => {
    const sight = state.ownVitals()?.familiarSight;
    const characterId = state.ownVitals()?.characterId ?? '';
    const campaignId = state.campaignId();
    untracked(() => {
      if (!sight || state.isMaster()) {
        state.familiarNameNow.set(null);
        return;
      }
      void services.familiarEyes
        .name(campaignId, characterId, sight.creatureId)
        .then((n) => state.familiarNameNow.set(n));
    });
  });

  // The beast's armor class from its book, read once for each beast the druid becomes.
  effect(() => {
    const key = state.ownVitals()?.wildShape?.beastKey ?? '';
    const campaignId = state.campaignId();
    untracked(() => {
      if (!key || state.isMaster()) {
        state.beastAc.set(null);
        return;
      }
      services.source.getCreatureArmorClass(campaignId, key).then(
        (ac) => state.beastAc.set(ac),
        () => state.beastAc.set(null),
      );
    });
  });

  // Waiting on the link before the master starts: the app's light poll
  // (RN-06) notices the new session, and the page opens it by itself.
  effect(() => {
    const id = state.campaignId();
    const listed = services.openSessions.liveCampaignIds().has(id);
    // Only a campaign that newly shows up in the list opens the page: a stale list still naming a session that
    // ended would otherwise load, find none, and load again without end.
    const appeared = listedCampaign.id === id && !listedCampaign.listed && listed;
    listedCampaign = { id, listed };
    if (state.phase() === 'no-session' && appeared) {
      untracked(() => load(id));
    }
  });
}
