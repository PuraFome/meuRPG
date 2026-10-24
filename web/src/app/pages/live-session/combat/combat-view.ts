import {
  Component,
  DestroyRef,
  type TemplateRef,
  computed,
  contentChildren,
  effect,
  inject,
  input,
  output,
  untracked,
} from '@angular/core';
import {
  CombatEffect,
  CombatantSide,
  CombatantState,
  CoverDegree,
  type Encounter,
  EncounterStatus,
} from '../../../../gen/meurpg/play/v1/combat_pb';
import {
  type MineTab,
  actingTab,
  membersToEnd,
  ownCreatures,
  validTab,
} from '../../../core/combat/mine';
import { offersToAnswer, reactorIsMasters } from '../../../core/combat/opportunity';
import { sheetWindow } from '../../../core/combat/reactions';
import { questions } from '../../../core/combat/hidden-reveal';
import { heldOfferOf } from '../../../core/held-rolls/held-roll-offer';
import { roundLabel, turnBanner } from '../../../core/combat/combat-view';
import type { MasterQuestion } from '../../../core/held-rolls/held-questions';
import { COMBAT_VIEW_IMPORTS } from './combat-view-imports';
import { CombatViewPartsDirective } from './combat-view-parts';
import { MapTab } from './map-tabs/map-tab';
import { MasterLayout } from './master-layout';
import { type RiderPick } from './hit-rider/hit-rider-card';
import {
  type StartCombatData,
  StartCombatDialog,
} from '../../../shared/start-combat/start-combat-dialog';

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
export class CombatView extends CombatViewPartsDirective {
  /** Set the first time the combat is read: "Combate atualizado agora." only follows a reload. */
  private refreshChecked = false;

  /** The conversation the page draws in the master's rail ("Turno · Conversa"), or `null` where the page keeps it elsewhere. */
  readonly chat = input<TemplateRef<unknown> | null>(null);
  /** The conversation's lines that came while it was not shown: the count of its tab. */
  readonly chatUnread = input(0);
  /** The conversation is what the rail shows now (its tab is open): the page counts nothing as unread meanwhile. */
  readonly chatShown = output<boolean>();
  /** The panels the page projects under the map (fog, traps, treasure, puzzles). */
  protected readonly projected = contentChildren(MapTab);

  /** The tabs of the master's combat and which one is open. */
  protected readonly layout = new MasterLayout({
    waiting: computed(() => this.held.count() + (this.owed() ? 1 : 0) + (this.waitNote() ? 1 : 0)),
    unread: computed(() => this.chatUnread()),
    hasChat: computed(() => this.chat() !== null),
    projected: computed(() => this.projected()),
    combatants: computed(() => this.encounter()?.combatants.length ?? 0),
    theatre: computed(() => this.theatre()),
  });
  /** "Rodada 3", for the phone's line under the bar. */
  protected readonly roundText = computed(() => roundLabel(this.encounter()?.round ?? 0));
  /** Who comes after the one on turn. */
  protected readonly nextName = computed(() => {
    const e = this.encounter();
    return e ? (turnBanner(e).after?.name ?? '') : '';
  });

  /** The answer went to the order: on a small screen the order is its own tab, so it opens first. */
  protected override focusOrder(): void {
    if (!this.laptop()) {
      this.layout.phoneTab.set('order');
    }
    super.focusOrder();
  }

  /** Who the held attack was against, for the question's words. */
  protected heldTarget(q: MasterQuestion): string {
    const target = q.controller.offer()?.targetId;
    return (target && this.encounter()?.combatants.find((c) => c.id === target)?.label) || '';
  }

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
    // The rail's conversation tab is open (and the rail is there): the page counts nothing as unread.
    effect(() => {
      const shown =
        this.isMaster() &&
        this.laptop() &&
        this.chat() !== null &&
        this.layout.railTab() === 'chat' &&
        this.active();
      untracked(() => this.chatShown.emit(shown));
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
        const later = this.deaths.later();
        if ([...later].some((id) => !dying.has(id))) {
          this.deaths.later.set(new Set([...later].filter((id) => dying.has(id))));
        }
      });
    });
    // The offer's form belongs to a turn: when it passes, or the combat ends, it closes.
    effect(() => {
      void this.turnKey();
      untracked(() => this.offers.offering.set(false));
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
          this.deaths.result.set('');
          this.deaths.masterResult.set('');
          this.deaths.asking.set(null);
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
      const offer = this.ownHeld();
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
    // The master's questions follow the combat: the rolls that wait for their roller sit in the roller's row. The roller reads
    // their own question the same way, to tell the master answered it when it is gone.
    effect(() => {
      const e = this.encounter();
      const master = this.isMaster();
      const own = this.own();
      untracked(() => {
        const live = !!e && e.status === EncounterStatus.ACTIVE;
        // The initiative of a player waits in SETUP for the die it may use, so the master's questions run there too.
        const asks = live || (!!e && e.status === EncounterStatus.SETUP);
        if (master) {
          this.held.sync(asks ? e.combatants.flatMap((c) => heldOfferOf(c) ?? []) : []);
        } else if (live) {
          this.vanish.observe(own ? (heldOfferOf(own) ?? null) : null);
        }
      });
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
        if (!this.offers.handled.has(offer.id)) {
          this.offers.handled.add(offer.id);
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
    inject(DestroyRef).onDestroy(() => {
      this.turnAlert.reset();
      this.held.dispose();
    });
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
          !untracked(
            () => this.offers.reactorOptions().has(offer.id) || this.offers.reading.has(offer.id),
          )
        ) {
          untracked(() => void this.offers.loadReactor(offer));
        }
      }
    });
  }

  /** "Técnica da Mão Aberta" and "Golpe Atordoante": the monk's answer to an offer after a hit. */
  protected answerRider(pick: RiderPick): Promise<boolean> {
    return this.calls.run((e) =>
      this.api.turn.useHitRider(this.campaignId(), e.id, pick.id, pick.choice),
    );
  }

  /** "Encerrar a parte dos Lobos" / "Encerrar a vez do Nanquim": the part of every member of the tab that still acts, one call each (a joint turn ends part by part). */
  protected async endTab(tab: MineTab): Promise<void> {
    for (const member of tab.members) {
      const e = this.encounter();
      // Read again before each call: the answer to the one before may have moved the turn on.
      if (!e || !membersToEnd(e, { ...tab, members: [member] }).length) {
        continue;
      }
      if (
        !(await this.calls.run((current) =>
          this.api.turn.endTurn(this.campaignId(), current.id, member.id, false, current.round),
        ))
      ) {
        return;
      }
    }
  }
  /** A damage was settled: the master's card stays for its note. */
  protected settled(): void {
    this.settledTurn.set(this.subject()?.id ?? '');
  }

  protected endCombat(): Promise<boolean> {
    return this.calls.run((e) => this.api.turn.end(this.campaignId(), e.id));
  }

  protected reveal(change: { id: string; hidden: boolean }): Promise<boolean> {
    return this.calls.run((e) =>
      this.api.moves.setHidden(this.campaignId(), e.id, change.id, change.hidden),
    );
  }

  /** The master: "Perdeu a concentração" (the app does not roll the save; the damage card reminds the DC). */
  protected loseConcentration(id: string): Promise<boolean> {
    return this.calls.run((e) =>
      this.api.combatants.setConditions(this.campaignId(), e.id, id, { endConcentration: true }),
    );
  }

  /** The master: "Encerrar Ajuda": the bonus goes, and the hit points lose only what passes the new maximum. */
  protected endAid(id: string): Promise<boolean> {
    return this.calls.run((e) =>
      this.api.combatants.endCombatEffect(this.campaignId(), e.id, id, CombatEffect.AID),
    );
  }

  protected remove(id: string): Promise<boolean> {
    return this.calls.run((e) => this.api.moves.remove(this.campaignId(), e.id, id));
  }

  protected adjustFor(characterId: string): void {
    const v = this.vitals().find((x) => x.characterId === characterId);
    if (v) {
      this.adjust.emit(v);
    }
  }

  /** "Adicionar combatente": the start dialog, with the NPCs only. */
  protected addCombatants(): void {
    const e = this.encounter();
    if (!e) {
      return;
    }
    const phone = this.phone();
    this.dialog
      .open<StartCombatDialog, StartCombatData, Encounter>(StartCombatDialog, {
        data: {
          campaignId: this.campaignId(),
          mode: 'add',
          map: null,
          encounterId: e.id,
          existing: e.combatants.length,
        },
        width: phone ? '100vw' : '760px',
        maxWidth: phone ? '100vw' : 'calc(100vw - 32px)',
        height: phone ? '100dvh' : undefined,
        maxHeight: phone ? '100dvh' : '92dvh',
        ariaLabelledBy: 'start-title',
        autoFocus: 'first-tabbable',
      })
      .afterClosed()
      .subscribe((added) => added && this.state().apply(added));
  }
  // ---- opportunity attacks (E9-13) ----

  /** "Desfazer o movimento": the master takes back his last action when it is the move that made the offers. */
  protected undoMove(): Promise<boolean> {
    const entry = this.log.undoable();
    if (!entry) {
      return Promise.resolve(false);
    }
    return this.calls.run(async (e) => {
      const undone = await this.api.turn.undo(this.campaignId(), e.id, entry.id);
      this.undone.mark();
      return undone;
    });
  }

  /** The master's "Marcar como aliado" / "Voltar a ser inimigo". */
  protected setSide(change: { id: string; side: CombatantSide }): Promise<boolean> {
    return this.calls.run((e) =>
      this.api.moves.setSide(this.campaignId(), e.id, change.id, change.side),
    );
  }

  /** The master's cover mark, applied at once. */
  protected setCover(change: { id: string; cover: CoverDegree }): Promise<boolean> {
    return this.calls.run((e) =>
      this.api.moves.setCover(this.campaignId(), e.id, change.id, change.cover),
    );
  }

  // ---- moving ----

  /** "Levantar-se": the combatant (the player's own, a creature of theirs, or the one on the master's card) stands up from Prone. */
  protected standUp(id?: string): Promise<boolean> {
    const who = id ?? this.own()?.id;
    return who
      ? this.calls.run((e) => this.api.moves.standUp(this.campaignId(), e.id, who))
      : Promise.resolve(false);
  }

  /** "Colocar no mapa": the server picks the square, by the rule that places the
   * NPCs when the combat begins (free floor, away from the players). */
  protected async place(id: string): Promise<void> {
    await this.calls.run((current) => this.api.moves.place(this.campaignId(), current.id, id));
  }

  // ---- the sheets and moving are bound by the template through `sheets` and `moves` ----

  /** "Ver pelos olhos do familiar": only with a familiar to look through. */
  protected askFamiliarEyes(): void {
    const name = this.familiar();
    if (name) {
      this.sheets.actions.familiarEyes(name);
    }
  }

  protected leave(): void {
    this.state().dismissEnded();
  }
}
