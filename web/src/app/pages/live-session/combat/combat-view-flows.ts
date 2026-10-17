import { Directive } from '@angular/core';
import {
  CombatEffect,
  CombatantSide,
  CoverDegree,
  type Encounter,
  type OpportunityOffer,
} from '../../../../gen/meurpg/play/v1/combat_pb';
import { newKey } from '../../../core/combat/combat-client';
import { type MineTab, membersToEnd } from '../../../core/combat/mine';
import { currentCombatant } from '../../../core/combat/combat-view';
import { type RiderPick } from './hit-rider/hit-rider-card';
import { type MasterAnswer } from './opportunity/opportunity-card';
import {
  type StartCombatData,
  StartCombatDialog,
} from '../../../shared/start-combat/start-combat-dialog';
import { CombatViewPartsDirective } from './combat-view-parts';

/** The rest of the screen's calls: the form, the opportunity attacks, the sides and the move. */
@Directive()
export abstract class CombatViewFlowsDirective extends CombatViewPartsDirective {
  /** "Técnica da Mão Aberta" and "Golpe Atordoante": the monk's answer to an offer after a hit. */
  protected answerRider(pick: RiderPick): Promise<boolean> {
    return this.calls.run((e) =>
      this.api.useHitRider(this.campaignId(), e.id, pick.id, pick.choice),
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
          this.api.endTurn(this.campaignId(), current.id, member.id, false, current.round),
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
    return this.calls.run((e) => this.api.end(this.campaignId(), e.id));
  }

  protected reveal(change: { id: string; hidden: boolean }): Promise<boolean> {
    return this.calls.run((e) =>
      this.api.setHidden(this.campaignId(), e.id, change.id, change.hidden),
    );
  }

  /** The master: "Perdeu a concentração" (the app does not roll the save; the damage card reminds the DC). */
  protected loseConcentration(id: string): Promise<boolean> {
    return this.calls.run((e) =>
      this.api.setConditions(this.campaignId(), e.id, id, { endConcentration: true }),
    );
  }

  /** The master: "Encerrar Ajuda": the bonus goes, and the hit points lose only what passes the new maximum. */
  protected endAid(id: string): Promise<boolean> {
    return this.calls.run((e) =>
      this.api.endCombatEffect(this.campaignId(), e.id, id, CombatEffect.AID),
    );
  }

  protected remove(id: string): Promise<boolean> {
    return this.calls.run((e) => this.api.remove(this.campaignId(), e.id, id));
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

  /** The master answers for an NPC: "Não atacar", or "Atacar com <arma>". */
  protected answerOpportunity(a: MasterAnswer): Promise<boolean> | void {
    if (!a.attack) {
      return this.calls.run((e) =>
        this.api.declineOpportunity(this.campaignId(), e.id, a.offer.id),
      );
    }
    this.sheets.reactions.opportunityAttack(
      a.offer,
      a.attack.key,
      this.reactorOptions().get(a.offer.id),
      true,
    );
  }

  /** "Desfazer o movimento": the master takes back his last action when it is the move that made the offers. */
  protected undoMove(): Promise<boolean> {
    const entry = this.log.undoable();
    if (!entry) {
      return Promise.resolve(false);
    }
    return this.calls.run(async (e) => {
      const undone = await this.api.undo(this.campaignId(), e.id, entry.id);
      this.undone.mark();
      return undone;
    });
  }

  /** "Seguir sem esperar": the master passes over an offer the player does not answer. */
  protected skipOpportunity(offer: OpportunityOffer): Promise<boolean> {
    return this.calls.run((e) => this.api.skipOpportunity(this.campaignId(), e.id, offer.id));
  }

  /** "Oferecer a Toren": the master says the one on turn left that combatant's reach (a combat without a map). */
  protected async makeOffer(reactorId: string): Promise<void> {
    const e = this.encounter();
    const mover = e ? currentCombatant(e) : null;
    if (!e || !mover) {
      return;
    }
    this.offerError.set('');
    let key = this.offerKeys.get(reactorId);
    if (!key) {
      key = newKey();
      this.offerKeys.set(reactorId, key);
    }
    const ok = await this.calls.run((current) =>
      this.api.offerOpportunity(this.campaignId(), current.id, mover.id, reactorId, key),
    );
    if (ok) {
      this.offering.set(false);
    } else if (this.error()) {
      // The refusal stays in the form, where the master is looking (the page's notice has it too).
      this.offerError.set(this.error());
      this.error.set('');
      // A refused offer is not a retry: the next try is a new one.
      this.offerKeys.delete(reactorId);
    }
  }

  /** "Oferecer ataque de oportunidade": opens the form under its button (a new form, new keys), or closes it. */
  protected toggleOffer(): void {
    if (!this.offering()) {
      this.offerKeys = new Map();
      this.offerError.set('');
    }
    this.offering.set(!this.offering());
  }

  /** "Retirar a oferta": the master takes back an offer nobody answered; the reactor keeps its reaction. */
  protected withdrawOpportunity(offer: OpportunityOffer): Promise<boolean> {
    return this.calls.run((e) => this.api.withdrawOpportunity(this.campaignId(), e.id, offer.id));
  }

  /** The master's "Marcar como aliado" / "Voltar a ser inimigo". */
  protected setSide(change: { id: string; side: CombatantSide }): Promise<boolean> {
    return this.calls.run((e) => this.api.setSide(this.campaignId(), e.id, change.id, change.side));
  }

  /** The master's cover mark, applied at once. */
  protected setCover(change: { id: string; cover: CoverDegree }): Promise<boolean> {
    return this.calls.run((e) =>
      this.api.setCover(this.campaignId(), e.id, change.id, change.cover),
    );
  }

  // ---- moving ----

  /** "Levantar-se": the combatant (the player's own, a creature of theirs, or the one on the master's card) stands up from Prone. */
  protected standUp(id?: string): Promise<boolean> {
    const who = id ?? this.own()?.id;
    return who
      ? this.calls.run((e) => this.api.standUp(this.campaignId(), e.id, who))
      : Promise.resolve(false);
  }

  /** "Colocar no mapa": the server picks the square, by the rule that places the
   * NPCs when the combat begins (free floor, away from the players). */
  protected async place(id: string): Promise<void> {
    await this.calls.run((current) => this.api.place(this.campaignId(), current.id, id));
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
