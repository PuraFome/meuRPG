import { Component, DestroyRef, effect, inject, untracked } from '@angular/core';
import { CombatantState, EncounterStatus } from '../../../../gen/meurpg/play/v1/combat_pb';
import { actingTab, ownCreatures, validTab } from '../../../core/combat/mine';
import { offersToAnswer, reactorIsMasters } from '../../../core/combat/opportunity';
import { sheetWindow } from '../../../core/combat/reactions';
import { questions } from '../../../core/combat/hidden-reveal';
import { COMBAT_VIEW_IMPORTS } from './combat-view-imports';
import { CombatViewFlowsDirective } from './combat-view-flows';

/**
 * The combat on the session page (MR-013, E6-01 to E6-16): it picks the
 * screen by who is looking and where the combat is, and runs every call.
 *
 * - **Master:** SETUP is the initiative list with its side panel (E6-04);
 *   ACTIVE is the combat bar, the map and the order (E6-11, E6-12); ENDED is
 *   the summary (E6-16).
 * - **Player:** SETUP is "Role a iniciativa" (E6-03); ACTIVE is the turn
 *   banner with the order strip, the map and, projected in, the vitals
 *   (E6-05), or the "Mover" page (E6-10); ENDED is the summary.
 *
 * It never reads the combat by itself: the page does, after
 * `encounter_changed`. A call's answer is the combat as it is now, so it is
 * applied at once; a refusal that means "the screen is stale" reads it again.
 */
@Component({
  selector: 'app-combat-view',
  imports: COMBAT_VIEW_IMPORTS,
  templateUrl: './combat-view.html',
  styleUrl: './combat-view.scss',
})
export class CombatView extends CombatViewFlowsDirective {
  /** Set the first time the combat is read: "Combate atualizado agora." only follows a reload. */
  private refreshChecked = false;

  constructor() {
    super();
    effect(() => {
      const e = this.encounter();
      untracked(() => {
        const held =
          !!e && e.status === EncounterStatus.ACTIVE && (e.turnHeld || questions(e).length > 0);
        if (e && !this.refreshChecked) {
          this.refreshChecked = true;
          this.refreshed.set(held ? 'Combate atualizado agora.' : '');
        } else if (!held) {
          this.refreshed.set('');
        }
      });
    });
    // "Ainda não" holds while the character stays dying: when they leave DYING the question is
    // new the next time they fall.
    effect(() => {
      const dying = new Set(
        (this.encounter()?.combatants ?? [])
          .filter((c) => c.state === CombatantState.DYING)
          .map((c) => c.id),
      );
      untracked(() => {
        const later = this.deathLater();
        if ([...later].some((id) => !dying.has(id))) {
          this.deathLater.set(new Set([...later].filter((id) => dying.has(id))));
        }
      });
    });
    // The offer's form belongs to a turn: when it passes, or the combat ends, it closes.
    effect(() => {
      void this.turnKey();
      untracked(() => this.offering.set(false));
    });
    // The options are asked for again whenever the combat changes (every
    // call and every event bumps its revision) and whoever is in the
    // spotlight changes. A turn that ends leaves nothing to show.
    effect(() => {
      const e = this.encounter();
      const who = this.subject();
      const campaignId = this.campaignId();
      if (!e || !who) {
        untracked(() => this.turn.clear());
        return;
      }
      void e.revision;
      untracked(() => void this.turn.load(this.api, campaignId, e.id, who.id));
    });
    // The log is read again on every `combat_log_changed` and after every
    // reconnection (the page bumps `logTick`).
    effect(() => {
      const e = this.encounter();
      const campaignId = this.campaignId();
      this.state().logTick();
      if (!e || e.status === EncounterStatus.SETUP) {
        untracked(() => this.log.clear());
        return;
      }
      untracked(() => void this.log.load(this.api, campaignId, e.id));
    });
    // What a turn said is not carried into the next one.
    let turnOf = '';
    effect(() => {
      const id = this.encounter()?.currentCombatantId ?? '';
      if (id !== turnOf) {
        turnOf = id;
        untracked(() => {
          this.actionNote.set('');
          this.deathResult.set('');
          this.masterDeathResult.set('');
          this.passAsking.set(null);
          this.moves.moveNote.set('');
        });
      }
    });
    // A reaction window the player answers (Escudo, Esquiva, Contramágica, the concentration save...) opens its sheet
    // by itself, once; a reload brings it back, as the window lives on the server.
    effect(() => {
      const e = this.encounter();
      const own = this.own();
      if (
        this.isMaster() ||
        !e ||
        !own ||
        e.status !== EncounterStatus.ACTIVE ||
        this.reactionOpen()
      ) {
        return;
      }
      const window = sheetWindow(e);
      if (window && !this.reactionHandled.has(window.id)) {
        this.reactionHandled.add(window.id);
        untracked(() => this.sheets.reactions.reaction(window));
      }
    });
    // A roll held for a question, Lucky or Bardic Inspiration (the player read the screen again, or left the sheet): the
    // question opens by itself, once.
    effect(() => {
      const e = this.encounter();
      const offer = this.ownLucky() ?? this.ownOffer();
      // The turn options arrive after the combat: the question opens when its attack is in them.
      const ready = this.options() !== null;
      // While an attack sheet is open it is the one that gets the answer of its own roll.
      if (
        this.isMaster() ||
        !e ||
        !offer ||
        !ready ||
        this.attackSheetOpen() ||
        e.status !== EncounterStatus.ACTIVE
      ) {
        return;
      }
      if (!this.inspirationHandled.has(offer.holdId)) {
        untracked(() => {
          if (this.openHeldRoll()) {
            this.inspirationHandled.add(offer.holdId);
          }
        });
      }
    });
    // The map's layers: read when the combat's map is known, and again when the map's `layers_revision` changes.
    effect(() => {
      const e = this.encounter();
      const map = this.mapState().map();
      const id = e && map && map.id === e.mapId ? map.id : null;
      const revision = map?.layersRevision ?? 0;
      untracked(() => void this.layersState.open(id, revision));
    });
    // An offer the player answers opens its prompt by itself, once (E9-13).
    effect(() => {
      const e = this.encounter();
      if (this.isMaster() || !e || e.status !== EncounterStatus.ACTIVE) {
        return;
      }
      for (const offer of offersToAnswer(e)) {
        if (!this.offersHandled.has(offer.id)) {
          this.offersHandled.add(offer.id);
          untracked(() => this.sheets.reactions.opportunity(offer));
        }
      }
    });
    // The turn alert: the turn passing to a group the player plays (vibration, chime, shake, title); the first look is only the baseline.
    effect(() => {
      const tab = this.tabs().find((t) => t.state === 'turn');
      const turn = tab ? { key: tab.id, label: tab.label } : null;
      untracked(() => this.turnAlert.observe(turn));
    });
    inject(DestroyRef).onDestroy(() => this.turnAlert.reset());
    // The tab follows the turn: when it comes to a group of the player's (the character or its creatures), the page shows it.
    let acting = '';
    effect(() => {
      const now = actingTab(this.tabs());
      if (now && now !== acting) {
        untracked(() => this.tabId.set(now));
      }
      acting = now;
    });
    // The tabs go away when the player is back to one combatant: the page is the character's again.
    effect(() => {
      const keep = validTab(this.tabs(), this.tabId());
      if (keep !== this.tabId()) {
        untracked(() => this.tabId.set(keep));
      }
    });
    // The creatures' own options (their attacks, their standard actions), asked again whenever the combat changes.
    effect(() => {
      const e = this.encounter();
      const campaignId = this.campaignId();
      const ids =
        e && !this.isMaster() && e.status === EncounterStatus.ACTIVE
          ? ownCreatures(e).map((c) => c.id)
          : [];
      if (!e || ids.length === 0) {
        untracked(() => this.creatureOptions.clear());
        return;
      }
      void e.revision;
      untracked(() => void this.creatureOptions.load(this.api, campaignId, e.id, ids));
    });
    // When the form ends without the druid asking (the beast fell to 0, the master ended it), the player is told, until they touch it:
    // the combat log's line says why and how much damage passed on (the server's number; nothing is worked out here). The lines that
    // were there when the combat was read are not news, so a reconnect or a re-read never repeats a notice.
    effect(() => {
      const id = this.encounter()?.id ?? '';
      const own = this.own();
      const rounds = this.log.rounds();
      const loaded = this.log.loaded();
      untracked(() => {
        this.notices.readForm(
          id,
          own?.id ?? '',
          loaded,
          rounds.flatMap((r) => r.entries),
        );
      });
    });
    // When the concentration that held creatures ends (a lost save, another spell, the master), the player is told too. It reads the
    // combat's own data, for this combat only: another combat starts without it, and an undo that brings the creatures (or the spell)
    // back takes it away.
    effect(() => {
      const e = this.encounter();
      const spell = this.own()?.concentrationSpellNamePt ?? '';
      const creatures =
        e && !this.isMaster() ? ownCreatures(e).filter((c) => !!c.summonGroupId) : [];
      untracked(() => {
        this.notices.readConcentration(e?.id ?? '', spell, creatures);
      });
    });
    // The master's prompts need each NPC reactor's attacks with their numbers.
    effect(() => {
      const e = this.encounter();
      if (!this.isMaster() || !e) {
        return;
      }
      for (const offer of this.toAnswer()) {
        if (
          reactorIsMasters(e, offer) &&
          !untracked(() => this.reactorOptions().has(offer.id) || this.reactorReading.has(offer.id))
        ) {
          untracked(() => void this.loadReactor(offer));
        }
      }
    });
  }
}
