import { Injectable } from '@angular/core';
import {
  type CombatEffect,
  type Encounter,
  type GetCombatHighlightsResponse,
  type ListCombatLogResponse,
  type PreviewSpellAreaResponse,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { InspirationQuestion } from '../resources/bardic-inspiration';
import { LuckyQuestion } from './lucky-question';
import {
  type AttackDie,
  type PairDie,
  type ModeChoice,
  type DamageDie,
  type PoolDie,
  type SlotRef,
  type MetamagicSpec,
  type CastTarget,
  type CastResult,
  type AreaChoice,
  type CastExtras,
  type SummonRequest,
  type DeathSaveResult,
  type ActionResult,
  type ConditionChange,
  type HpAdjust,
} from './combat-client-types';
import { castExtrasBody, castRollOneof, areaOneof, need } from './combat-client-wire';
import { CombatAttackCalls } from './combat-client-attack';
// The key maker lives in `core/connect`, the wire types and `attackResult` in the siblings; the combat screens import them all from here.
export { newKey } from '../connect/idempotency';
export * from './combat-client-types';
export { attackResult } from './combat-client-wire';

/**
 * Thin wrapper around the generated `CombatService` client (MR-013), in the
 * same shape as `MapsClient`. `providedIn: 'root'`, and only lazy code
 * imports it, so the generated combat code stays out of the initial bundle.
 * Every write sends an idempotency key made for that call: a double tap
 * never runs twice, and a call the app repeats on purpose is a new action.
 * Callers map errors to Portuguese with `combat-errors.ts`.
 */
@Injectable({ providedIn: 'root' })
export class CombatClient extends CombatAttackCalls {
  /** A standard action ("standard:dash") or a feature's ("feature:second-wind").
   * `die` is for the one that rolls (Retomar o Fôlego: the d10); `key` is made
   * once per action, so a retry after a lost answer never spends it twice. */
  async takeAction(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    actionKey: string,
    die?: DamageDie,
    key?: string,
    useExtraAction = false,
    frenzy = false,
  ): Promise<ActionResult> {
    const res = await this.keyed(
      ['takeAction', campaignId, encounterId, combatantId, actionKey, die, useExtraAction, frenzy],
      (sent) =>
        this.client.takeAction({
          campaignId,
          encounterId,
          combatantId,
          actionKey,
          idempotencyKey: sent,
          // The extra action an effect gives (Velocidade) pays for it instead of the action (RN-22).
          useExtraAction,
          // The Berserker's Fúria in a frenzy (SRD 5.1, Frenzy); ignored by every other action.
          frenzy,
          roll: !die
            ? { case: undefined }
            : 'inApp' in die
              ? { case: 'rollInApp', value: true }
              : { case: 'typedSum', value: die.sum },
        }),
      key,
    );
    return { encounter: need(res.encounter, 'TakeAction'), roll: res.roll, healed: res.healed };
  }

  /** `CastSpell`. `slot` is `null` for a cantrip; `die` is the d20 of a spell
   * attack (one per target in the app, or a typed face for one target), or the
   * pool of a spell that reads hit points (rolled by the server, or the typed sum). */
  async castSpell(
    campaignId: string,
    encounterId: string,
    casterId: string,
    spellKey: string,
    slot: SlotRef | null,
    targets: readonly CastTarget[],
    die: AttackDie | PoolDie | PairDie | null,
    key: string,
    summon?: SummonRequest,
    damageTypeKey = '',
    mode?: Omit<ModeChoice, 'requestId'>,
    metamagic: readonly MetamagicSpec[] = [],
    extras: CastExtras = {},
  ): Promise<CastResult> {
    const res = await this.client.castSpell({
      campaignId,
      encounterId,
      casterId,
      spellKey,
      slot: slot ?? undefined,
      targets: targets.map((t) => ({ combatantId: t.combatantId, darts: t.darts })),
      idempotencyKey: key,
      damageTypeKey,
      roll: castRollOneof(die),
      d20Faces: die && 'faces' in die ? [...die.faces] : [],
      ...(mode ? { rollMode: mode.mode, modeReason: mode.reason } : {}),
      metamagic: metamagic.map((m) => ({
        key: m.key,
        targetIds: [...m.targetIds],
        carefulIds: [...m.carefulIds],
        heightenedId: m.heightenedId,
      })),
      summon: summon
        ? {
            option: summon.option,
            creatureKeys: [...summon.creatureKeys],
            names: [...(summon.names ?? [])],
          }
        : undefined,
      area: areaOneof(extras.area),
      ...castExtrasBody(extras),
    });
    if (res.luckyOffer) {
      // A spell attack of a halfling whose d20 shows a 1: the d20 is kept, nothing was spent, and Lucky asks first.
      throw new LuckyQuestion(res.luckyOffer, need(res.encounter, 'CastSpell'));
    }
    if (res.inspirationOffer) {
      // A spell attack of a character that holds a die: the d20 is kept, nothing was spent.
      throw new InspirationQuestion(res.inspirationOffer, need(res.encounter, 'CastSpell'));
    }
    return {
      encounter: need(res.encounter, 'CastSpell'),
      cast: need(res.cast, 'CastSpell'),
      summoned: res.summonedCombatantIds,
      area: res.area
        ? {
            origin: res.area.origin ? { col: res.area.origin.col, row: res.area.origin.row } : null,
            squares: res.area.squares.map((q) => ({ col: q.col, row: q.row })),
          }
        : null,
      hiddenHits: res.hiddenHits,
      pendingRevealId: res.pendingRevealId,
    };
  }

  /** Revivify in a combat: the dead creature is a `deadTargetId` (never a combatant target), the diamonds are ticked, the key is the caller's. */
  async castRevivify(
    campaignId: string,
    encounterId: string,
    casterId: string,
    slot: SlotRef,
    deadTargetId: string,
    key: string,
  ): Promise<CastResult> {
    const res = await this.client.castSpell({
      campaignId,
      encounterId,
      casterId,
      spellKey: 'spell:revivify',
      slot,
      targets: [{ deadTargetId }],
      materialConfirmed: true,
      idempotencyKey: key,
    });
    return {
      encounter: need(res.encounter, 'CastSpell'),
      cast: need(res.cast, 'CastSpell'),
      summoned: res.summonedCombatantIds,
      area: null,
      hiddenHits: [],
      pendingRevealId: '',
    };
  }

  /** `PreviewSpellArea`: who an area spell placed here reaches and each one's cover, spending nothing. `area` is `null` for a
   * sphere centered on the caster, which has nothing to choose. */
  async previewSpellArea(
    campaignId: string,
    encounterId: string,
    casterId: string,
    spellKey: string,
    slot: SlotRef | null,
    area: AreaChoice | null,
  ): Promise<PreviewSpellAreaResponse> {
    return this.client.previewSpellArea({
      campaignId,
      encounterId,
      casterId,
      spellKey,
      slot: slot ?? undefined,
      area: areaOneof(area),
    });
  }

  /** The master's answer to a question of `Encounter.pending_hidden_reveals`: reveal the hidden creatures the area hit, or keep them hidden. */
  async resolveHiddenReveal(
    campaignId: string,
    encounterId: string,
    pendingRevealId: string,
    reveal: boolean,
  ): Promise<Encounter> {
    const res = await this.keyed(
      ['resolveHiddenReveal', campaignId, encounterId, pendingRevealId, reveal],
      (sent) =>
        this.client.resolveHiddenReveal({
          campaignId,
          encounterId,
          pendingRevealId,
          reveal,
          idempotencyKey: sent,
        }),
    );
    return need(res.encounter, 'ResolveHiddenReveal');
  }

  async rollDeathSave(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    die: AttackDie,
    key: string,
  ): Promise<DeathSaveResult> {
    const res = await this.client.rollDeathSave({
      campaignId,
      encounterId,
      combatantId,
      idempotencyKey: key,
      roll:
        'inApp' in die ? { case: 'rollInApp', value: true } : { case: 'd20Face', value: die.face },
    });
    return {
      encounter: need(res.encounter, 'RollDeathSave'),
      save: need(res.deathSave, 'RollDeathSave'),
    };
  }

  /** The master confirms a death (three failed death saves). It cannot be undone. */
  async confirmDeath(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    key: string,
  ): Promise<Encounter> {
    const res = await this.client.confirmDeath({
      campaignId,
      encounterId,
      combatantId,
      idempotencyKey: key,
    });
    return need(res.encounter, 'ConfirmDeath');
  }

  /** "Condições…": the labels (the master) and ending the concentration (the
   * master, or a player for their own character). */
  async setConditions(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    change: ConditionChange,
    key?: string,
  ): Promise<Encounter> {
    const res = await this.keyed(
      ['setCombatantConditions', campaignId, encounterId, combatantId, change],
      (sent) =>
        this.client.setCombatantConditions({
          campaignId,
          encounterId,
          combatantId,
          idempotencyKey: sent,
          conditions: change.keys ? { keys: [...change.keys] } : undefined,
          endConcentration: change.endConcentration ?? false,
        }),
      key,
    );
    return need(res.encounter, 'SetCombatantConditions');
  }

  /** The master's "Encerrar Ajuda": ends a lasting effect on a combatant (only Ajuda has an action; the Escudo Arcano ends by itself). */
  async endCombatEffect(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    effect: CombatEffect,
  ): Promise<Encounter> {
    const res = await this.keyed(
      ['endCombatEffect', campaignId, encounterId, combatantId, effect],
      (sent) =>
        this.client.endCombatEffect({
          campaignId,
          encounterId,
          combatantId,
          effect,
          idempotencyKey: sent,
        }),
    );
    return need(res.encounter, 'EndCombatEffect');
  }

  /** The master's "Dano/Cura" on an NPC. */
  async adjustHitPoints(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    adjust: HpAdjust,
    key: string,
  ): Promise<Encounter> {
    const c = adjust.change;
    const res = await this.client.adjustCombatantHitPoints({
      campaignId,
      encounterId,
      combatantId,
      idempotencyKey: key,
      change: !c
        ? { case: undefined }
        : c.kind === 'damage'
          ? { case: 'damage', value: c.value }
          : c.kind === 'heal'
            ? { case: 'heal', value: c.value }
            : { case: 'hitPoints', value: c.value },
      hitPointsTemporary: adjust.temporary,
    });
    return need(res.encounter, 'AdjustCombatantHitPoints');
  }

  /** "Desfazer última ação": `expectedEventId` is the log's undoable event. */
  async undo(campaignId: string, encounterId: string, expectedEventId: string): Promise<Encounter> {
    const res = await this.keyed(
      ['undoLastAction', campaignId, encounterId, expectedEventId],
      (sent) =>
        this.client.undoLastAction({
          campaignId,
          encounterId,
          expectedEventId,
          idempotencyKey: sent,
        }),
    );
    return need(res.encounter, 'UndoLastAction');
  }

  /** The combat log, latest first, as the caller may see it. */
  log(campaignId: string, encounterId: string): Promise<ListCombatLogResponse> {
    return this.client.listCombatLog({ campaignId, encounterId });
  }

  /** "Destaques do combate" (MR-032): who did the most in a combat that ended.
   * The master's answer also has the table of every player's numbers. */
  highlights(campaignId: string, encounterId: string): Promise<GetCombatHighlightsResponse> {
    return this.client.getCombatHighlights({ campaignId, encounterId });
  }

  async end(campaignId: string, encounterId: string): Promise<Encounter> {
    const res = await this.keyed(['endEncounter', campaignId, encounterId], (sent) =>
      this.client.endEncounter({
        campaignId,
        encounterId,
        idempotencyKey: sent,
      }),
    );
    return need(res.encounter, 'EndEncounter');
  }
}
