import { Injectable, inject } from '@angular/core';
import type { MessageInitShape } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';

import {
  type AttackRoll,
  CombatService,
  type DeathSave,
  type CoverDegree,
  type CombatantSide,
  type DiceRoll,
  type Encounter,
  EncounterMode,
  type GetCombatHighlightsResponse,
  type GetMoveOptionsResponse,
  type GetTurnOptionsResponse,
  type ListCombatLogResponse,
  JumpKind,
  MonsterHitPoints,
  type ParticipantSchema,
  type PendingDamage,
  type PreviewSpellAreaResponse,
  type ReactionOutcome,
  type SpellCast,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { newKey } from '../connect/idempotency';
import type { Square } from './combat-grid';
import { CONNECT_TRANSPORT } from '../connect/transport';

// The key maker moved to `core/connect`; the combat screens still import it from here.
export { newKey };

/** Who joins a combat: a character, how many copies (NPCs) and whether the
 * copies start hidden (unset: the server's default, hidden for an NPC). */
export interface JoinSpec {
  readonly characterId: string;
  readonly count?: number;
  readonly hidden?: boolean;
}

/** One creature of the bestiary, with how many come and the base name (empty: the creature's Portuguese name). */
export interface MonsterGroupSpec {
  readonly creatureKey: string;
  readonly count: number;
  readonly name?: string;
}

/** The hit points of monsters: the creature's average, or each one rolls its dice (RN-29). */
export type MonsterHp = 'average' | 'rolled';

/** What a start brings besides the party: the monsters of a saved encounter ("Começar este combate", MR-043) and the
 * battle point it was started from. Hit points and hidden apply to every group. */
export interface StartExtras {
  readonly monsters?: readonly MonsterGroupSpec[];
  readonly monsterHp?: MonsterHp;
  readonly monstersHidden?: boolean;
  /** A BATTLE point of the campaign the combat starts from (never with the combat without a map). */
  readonly mapPointId?: string;
  /** "Com mapa" or "Sem mapa (teatro da mente)" (RN-25). Left out, the server reads the table's "combate com mapa" rule
   * (RN-24); THEATRE sends no map point, and the monsters have no squares. */
  readonly mode?: EncounterMode;
}

/** What `AddMonsters` answers: the combat and the ids of the new combatants, in the order of their names. */
export interface AddMonstersResult {
  readonly encounter: Encounter;
  readonly combatantIds: readonly string[];
}

/** How a combatant's d20 comes (`SubmitInitiative`): the app rolls it, or a
 * face typed from a physical die. */
export type InitiativeRoll = { readonly inApp: true } | { readonly face: number };

/** How a d20 of an attack comes: the app rolls it, or a typed face. */
export type AttackDie = InitiativeRoll;

/** How the damage comes: the app rolls it, or the sum of the physical dice
 * (without the modifier; the server adds it). */
export type DamageDie = { readonly inApp: true } | { readonly sum: number };
/** How the pool of Sono or Leque Cromático comes: rolled by the server, or the sum of the physical dice. */
export type PoolDie = { readonly inApp: true } | { readonly poolSum: number };

/** What an attack answers: the combat, the roll and the damage it opened. */
export interface AttackResult {
  readonly encounter: Encounter;
  readonly roll: AttackRoll;
  readonly pending: PendingDamage | undefined;
}

/** What a move answers: the combat, and whether it stopped short or provoked. */
export interface MoveResult {
  readonly encounter: Encounter;
  readonly stoppedEarly: boolean;
  /** The move stopped before a locked door (RN-26): the page says "A porta está trancada." */
  readonly lockedDoor: boolean;
  readonly provoked: boolean;
}

/** What "Gastar movimento" answers: the combat, and what the combatant has left (tenths of a foot). */
export interface SpendResult {
  readonly encounter: Encounter;
  readonly movementLeftDft: number;
}

/** What a damage call answers: the combat and the damage as it is now. For an
 * area spell's one roll, `cast` has the other damages of the same cast that
 * the roll settled too. */
export interface DamageResult {
  readonly encounter: Encounter;
  readonly pending: PendingDamage;
  readonly cast: readonly PendingDamage[];
}

/** The slot a spell is cast with (`SpellSlot`); `null` for a cantrip. */
export interface SlotRef {
  readonly level: number;
  readonly pact: boolean;
}

/** One target of a cast; `darts` is Magic Missile's, 0 for any other spell. */
export interface CastTarget {
  readonly combatantId: string;
  readonly darts: number;
}

/** What a cast answers: the combat and what the spell did. */
export interface CastResult {
  readonly encounter: Encounter;
  readonly cast: SpellCast;
  /** A summoning spell: the combatants that joined the combat, in the order of the creatures chosen. */
  readonly summoned: readonly string[];
  /** An area placed on the map: where it landed and the squares it covers, as the caller knows the map. */
  readonly area: { readonly origin: Square | null; readonly squares: readonly Square[] } | null;
  /** The hidden creatures the area hit: the master's only (empty for a player). */
  readonly hiddenHits: readonly string[];
  /** The question the cast opened for the master ("Perguntar a cada vez"): the master's only. */
  readonly pendingRevealId: string;
}

/** Where an area spell is placed: a point for a sphere or a cylinder, a direction for a cone, a line or a cube. */
export type AreaChoice =
  | { readonly origin: Square }
  | { readonly direction: { readonly dx: number; readonly dy: number } };

/** What else a cast may carry: the placed area, and the master's choice on the hidden creatures it hits. */
export interface CastExtras {
  readonly area?: AreaChoice | null;
  /** The master only: whether the hidden creatures the area hits appear to the players. */
  readonly revealHidden?: boolean;
}

function areaOneof(area: AreaChoice | null | undefined) {
  if (!area) {
    return { case: undefined };
  }
  return 'origin' in area
    ? { case: 'origin' as const, value: { col: area.origin.col, row: area.origin.row } }
    : { case: 'direction' as const, value: { dx: area.direction.dx, dy: area.direction.dy } };
}

/** What a summoning spell brings (`SummonChoice`): the option, a content key per creature and, optionally, a name each. */
export interface SummonRequest {
  readonly option: number;
  readonly creatureKeys: readonly string[];
  readonly names?: readonly string[];
}

/** What a death save answers: the combat and the save. */
export interface DeathSaveResult {
  readonly encounter: Encounter;
  readonly save: DeathSave;
}

/** What a standard or feature action answers: the combat, the roll of an
 * action that rolls (Retomar o Fôlego) and the hit points it healed. */
export interface ActionResult {
  readonly encounter: Encounter;
  readonly roll: DiceRoll | undefined;
  readonly healed: number | undefined;
}

/** What the master changes in "Condições…": the new set of conditions (unset
 * leaves them) and whether the concentration ends. */
export interface ConditionChange {
  readonly keys?: readonly string[];
  readonly endConcentration?: boolean;
}

/** What Escudo did to a hit (`UseReaction`). */
export interface ReactionResult {
  readonly encounter: Encounter;
  readonly outcome: ReactionOutcome;
}

/** One adjustment of an NPC's hit points ("Dano/Cura"): at most one of the
 * three, and, apart or together, new temporary hit points. */
export interface HpAdjust {
  readonly change?: { readonly kind: 'damage' | 'heal' | 'exact'; readonly value: number };
  readonly temporary?: number;
}

/**
 * Thin wrapper around the generated `CombatService` client (MR-013), in the
 * same shape as `MapsClient`. `providedIn: 'root'`, and only lazy code
 * imports it, so the generated combat code stays out of the initial bundle.
 * Every write sends an idempotency key made for that call: a double tap
 * never runs twice, and a call the app repeats on purpose is a new action.
 * Callers map errors to Portuguese with `combat-errors.ts`.
 */
@Injectable({ providedIn: 'root' })
export class CombatClient {
  private readonly client = createClient(CombatService, inject(CONNECT_TRANSPORT));
  /** The key of each change still waiting for its answer, by the request it carries. */
  private readonly sending = new Map<string, string>();

  /**
   * Sends a change under the key of its request: the same request again (a second tap, or a try after a lost
   * answer) keeps the key, so the server answers with what the first call did; other values are another change
   * with a new key. Once the change worked the next one, even with the same values (two equal hits), is new.
   * A `given` key is the caller's, kept across its own retries.
   */
  private async keyed<T>(
    what: readonly unknown[],
    call: (key: string) => Promise<T>,
    given?: string,
  ): Promise<T> {
    if (given !== undefined) {
      return call(given);
    }
    const print = JSON.stringify(what, (_name, value: unknown) =>
      typeof value === 'bigint' ? value.toString() : value,
    );
    let key = this.sending.get(print);
    if (key === undefined) {
      key = newKey();
      this.sending.set(print, key);
    }
    const res = await call(key);
    this.sending.delete(print);
    return res;
  }

  /** The latest combat of the open session, or `null` while it had none. */
  async get(campaignId: string): Promise<Encounter | null> {
    return (await this.client.getEncounter({ campaignId })).encounter ?? null;
  }

  async start(
    campaignId: string,
    name: string,
    participants: readonly JoinSpec[],
    idempotencyKey: string,
    extras: StartExtras = {},
  ): Promise<Encounter> {
    const theatre = extras.mode === EncounterMode.THEATRE;
    const res = await this.client.startEncounter({
      campaignId,
      idempotencyKey,
      name,
      participants: participants.map(toParticipant),
      mapPointId: theatre ? '' : (extras.mapPointId ?? ''),
      // Left out, the server reads the table's "combate com mapa" rule (RN-24).
      ...(extras.mode === undefined ? {} : { mode: extras.mode }),
      ...(extras.monsters && extras.monsters.length > 0
        ? {
            monsters: extras.monsters.map((m) => ({
              creatureKey: m.creatureKey,
              count: m.count,
              name: m.name ?? '',
            })),
            monsterHitPoints: toHitPoints(extras.monsterHp),
            monstersHidden: extras.monstersHidden ?? true,
          }
        : {}),
    });
    return need(res.encounter, 'StartEncounter');
  }

  /** "Pôr no combate" (`AddMonsters`): 1 to 10 monsters of one SRD creature. The caller makes the key once per add and sends it
   * again on a retry with the same parameters. */
  async addMonsters(
    campaignId: string,
    encounterId: string,
    add: {
      readonly creatureKey: string;
      readonly count: number;
      readonly name: string;
      readonly hp: MonsterHp;
      readonly hidden: boolean;
    },
    idempotencyKey: string,
  ): Promise<AddMonstersResult> {
    const res = await this.client.addMonsters({
      campaignId,
      encounterId,
      idempotencyKey,
      creatureKey: add.creatureKey,
      count: add.count,
      name: add.name,
      hitPoints: toHitPoints(add.hp),
      hidden: add.hidden,
    });
    return { encounter: need(res.encounter, 'AddMonsters'), combatantIds: res.combatantIds };
  }

  async submitInitiative(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    roll: InitiativeRoll,
  ): Promise<Encounter> {
    const res = await this.keyed(
      ['submitInitiative', campaignId, encounterId, combatantId, roll],
      (sent) =>
        this.client.submitInitiative({
          campaignId,
          encounterId,
          combatantId,
          idempotencyKey: sent,
          roll:
            'inApp' in roll
              ? { case: 'rollInApp', value: true }
              : { case: 'd20Face', value: roll.face },
        }),
    );
    return need(res.encounter, 'SubmitInitiative');
  }

  async setOrder(
    campaignId: string,
    encounterId: string,
    combatantIds: readonly string[],
  ): Promise<Encounter> {
    const res = await this.keyed(
      ['setInitiativeOrder', campaignId, encounterId, combatantIds],
      (sent) =>
        this.client.setInitiativeOrder({
          campaignId,
          encounterId,
          idempotencyKey: sent,
          combatantIds: [...combatantIds],
        }),
    );
    return need(res.encounter, 'SetInitiativeOrder');
  }

  async begin(campaignId: string, encounterId: string): Promise<Encounter> {
    const res = await this.keyed(['beginCombat', campaignId, encounterId], (sent) =>
      this.client.beginCombat({
        campaignId,
        encounterId,
        idempotencyKey: sent,
      }),
    );
    return need(res.encounter, 'BeginCombat');
  }

  /** `expectedCombatantId` is whose turn the screen thinks it is: a tap from a screen that has not caught up with the
   * turn is refused (`aborted`), never skipping another one; the same tap sent again keeps its key, so the server
   * answers with the first. Empty only when nobody is on turn. */
  async endTurn(
    campaignId: string,
    encounterId: string,
    expectedCombatantId: string,
    discardPendingDamage = false,
    expectedRound = 0,
  ): Promise<Encounter> {
    const res = await this.keyed(
      [
        'endTurn',
        campaignId,
        encounterId,
        expectedCombatantId,
        discardPendingDamage,
        expectedRound,
      ],
      (sent) =>
        this.client.endTurn({
          campaignId,
          encounterId,
          idempotencyKey: sent,
          expectedCombatantId,
          discardPendingDamage,
          expectedRound,
        }),
    );
    return need(res.encounter, 'EndTurn');
  }

  /** `MoveCombatant`: a walk to a square, or a jump (`jump`: a long one to the
   * square, or a high one by `jumpHeightDft` tenths of a foot). `stoppedEarly`
   * is "something you did not see stopped you"; `provoked` that an opportunity
   * offer now waits. The master's drags are plain moves: only `forced` (the
   * "Movimento forçado" box: a teleport, a push or a pull) skips the offers.
   * `key` is the caller's, kept across the retries of one jump (a lost answer
   * must not charge it twice). */
  async move(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    col: number,
    row: number,
    jump?: { readonly kind: 'long' } | { readonly kind: 'high'; readonly heightDft: number },
    key?: string,
    forced = false,
  ): Promise<MoveResult> {
    const res = await this.keyed(
      ['moveCombatant', campaignId, encounterId, combatantId, col, row, jump, forced],
      (sent) =>
        this.client.moveCombatant({
          campaignId,
          encounterId,
          combatantId,
          idempotencyKey: sent,
          col,
          row,
          jump: jump
            ? jump.kind === 'long'
              ? JumpKind.LONG
              : JumpKind.HIGH
            : JumpKind.UNSPECIFIED,
          jumpHeightDft: jump?.kind === 'high' ? jump.heightDft : 0,
          forced,
        }),
      key,
    );
    return {
      encounter: need(res.encounter, 'MoveCombatant'),
      stoppedEarly: res.stoppedEarly,
      lockedDoor: res.lockedDoor,
      provoked: res.provoked,
    };
  }

  /** "Colocar no mapa": the server picks the square of a combatant that has none
   * (free floor, away from the players), so the screen never guesses one. */
  async place(campaignId: string, encounterId: string, combatantId: string): Promise<Encounter> {
    const res = await this.keyed(['placeCombatant', campaignId, encounterId, combatantId], (sent) =>
      this.client.moveCombatant({
        campaignId,
        encounterId,
        combatantId,
        idempotencyKey: sent,
        place: true,
      }),
    );
    return need(res.encounter, 'MoveCombatant');
  }

  /** `GetMoveOptions`: where the combatant can go in one straight move, the
   * cost of each square and why the others inside the circle are refused. */
  moveOptions(
    campaignId: string,
    encounterId: string,
    combatantId: string,
  ): Promise<GetMoveOptionsResponse> {
    return this.client.getMoveOptions({ campaignId, encounterId, combatantId });
  }

  /** The master's "Aliado" (PARTY) or back to enemy (ENEMY). */
  async setSide(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    side: CombatantSide,
  ): Promise<Encounter> {
    const res = await this.keyed(
      ['setCombatantSide', campaignId, encounterId, combatantId, side],
      (sent) =>
        this.client.setCombatantSide({
          campaignId,
          encounterId,
          combatantId,
          idempotencyKey: sent,
          side,
        }),
    );
    return need(res.encounter, 'SetCombatantSide');
  }

  /** The master's "Marcar cobertura": the cover the map does not show. */
  async setCover(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    cover: CoverDegree,
  ): Promise<Encounter> {
    const res = await this.keyed(
      ['setCombatantCover', campaignId, encounterId, combatantId, cover],
      (sent) =>
        this.client.setCombatantCover({
          campaignId,
          encounterId,
          combatantId,
          idempotencyKey: sent,
          cover,
        }),
    );
    return need(res.encounter, 'SetCombatantCover');
  }

  /** "Não atacar": turns an opportunity offer down. */
  async declineOpportunity(
    campaignId: string,
    encounterId: string,
    offerId: string,
  ): Promise<Encounter> {
    const res = await this.keyed(['declineOpportunity', campaignId, encounterId, offerId], (sent) =>
      this.client.declineOpportunity({
        campaignId,
        encounterId,
        opportunityOfferId: offerId,
        idempotencyKey: sent,
      }),
    );
    return need(res.encounter, 'DeclineOpportunity');
  }

  /** The master's "Seguir sem esperar": passes over an offer nobody answers. */
  async skipOpportunity(
    campaignId: string,
    encounterId: string,
    offerId: string,
  ): Promise<Encounter> {
    const res = await this.keyed(['skipOpportunity', campaignId, encounterId, offerId], (sent) =>
      this.client.skipOpportunity({
        campaignId,
        encounterId,
        opportunityOfferId: offerId,
        idempotencyKey: sent,
      }),
    );
    return need(res.encounter, 'SkipOpportunity');
  }

  /** "Gastar movimento" (a combat without a map, RN-25): whole feet, never more than what is left. The server says what is left. */
  async spendMovement(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    distanceFt: number,
    idempotencyKey: string,
  ): Promise<SpendResult> {
    const res = await this.client.spendMovement({
      campaignId,
      encounterId,
      combatantId,
      distanceFt,
      idempotencyKey,
    });
    return {
      encounter: need(res.encounter, 'SpendMovement'),
      movementLeftDft: res.movementLeftDft,
    };
  }

  /** The master's "Oferecer ataque de oportunidade" (a combat without a map): who left whose reach. */
  async offerOpportunity(
    campaignId: string,
    encounterId: string,
    moverId: string,
    reactorId: string,
    idempotencyKey: string,
  ): Promise<Encounter> {
    const res = await this.client.offerOpportunity({
      campaignId,
      encounterId,
      moverId,
      reactorId,
      idempotencyKey,
    });
    return need(res.encounter, 'OfferOpportunity');
  }

  /** "Retirar a oferta": an offer nobody answered is taken back; the reactor keeps its reaction. */
  async withdrawOpportunity(
    campaignId: string,
    encounterId: string,
    offerId: string,
  ): Promise<Encounter> {
    const res = await this.keyed(
      ['withdrawOpportunity', campaignId, encounterId, offerId],
      (sent) =>
        this.client.withdrawOpportunity({
          campaignId,
          encounterId,
          opportunityOfferId: offerId,
          idempotencyKey: sent,
        }),
    );
    return need(res.encounter, 'WithdrawOpportunity');
  }

  async setHidden(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    hidden: boolean,
  ): Promise<Encounter> {
    const res = await this.keyed(
      ['setCombatantHidden', campaignId, encounterId, combatantId, hidden],
      (sent) =>
        this.client.setCombatantHidden({
          campaignId,
          encounterId,
          combatantId,
          idempotencyKey: sent,
          hidden,
        }),
    );
    return need(res.encounter, 'SetCombatantHidden');
  }

  async add(
    campaignId: string,
    encounterId: string,
    participants: readonly JoinSpec[],
    key?: string,
  ): Promise<Encounter> {
    const res = await this.keyed(
      ['addCombatants', campaignId, encounterId, participants],
      (sent) =>
        this.client.addCombatants({
          campaignId,
          encounterId,
          idempotencyKey: sent,
          participants: participants.map(toParticipant),
        }),
      key,
    );
    return need(res.encounter, 'AddCombatants');
  }

  async remove(campaignId: string, encounterId: string, combatantId: string): Promise<Encounter> {
    const res = await this.keyed(
      ['removeCombatant', campaignId, encounterId, combatantId],
      (sent) =>
        this.client.removeCombatant({
          campaignId,
          encounterId,
          combatantId,
          idempotencyKey: sent,
        }),
    );
    return need(res.encounter, 'RemoveCombatant');
  }

  /** What a combatant can do now ("Sua vez"), with the targets of each
   * attack and the damage still to roll or apply. */
  turnOptions(
    campaignId: string,
    encounterId: string,
    combatantId: string,
  ): Promise<GetTurnOptionsResponse> {
    return this.client.getTurnOptions({ campaignId, encounterId, combatantId });
  }

  /** The attack roll. `key` is made once per attack by the caller, so a retry
   * after a lost answer never rolls twice. */
  async rollAttack(
    campaignId: string,
    encounterId: string,
    attackerId: string,
    attackKey: string,
    targetId: string,
    die: AttackDie,
    key: string,
    asReaction = false,
    opportunityOfferId = '',
  ): Promise<AttackResult> {
    const res = await this.client.rollAttack({
      campaignId,
      encounterId,
      attackerId,
      attackKey,
      targetId,
      idempotencyKey: key,
      roll:
        'inApp' in die ? { case: 'rollInApp', value: true } : { case: 'd20Face', value: die.face },
      asReaction,
      opportunityOfferId,
    });
    return {
      encounter: need(res.encounter, 'RollAttack'),
      roll: need(res.roll, 'RollAttack'),
      pending: res.pendingDamage,
    };
  }

  async rollDamage(
    campaignId: string,
    encounterId: string,
    pendingDamageId: string,
    die: DamageDie,
    key: string,
  ): Promise<DamageResult> {
    const res = await this.client.rollDamage({
      campaignId,
      encounterId,
      pendingDamageId,
      idempotencyKey: key,
      roll:
        'inApp' in die ? { case: 'rollInApp', value: true } : { case: 'typedSum', value: die.sum },
    });
    return {
      encounter: need(res.encounter, 'RollDamage'),
      pending: need(res.pendingDamage, 'RollDamage'),
      cast: res.castPendingDamages,
    };
  }

  /** `amount` is the master's last word: the damage to apply instead of the
   * rolled one (0 to 9,999). `key` is made once per apply, so a retry never
   * applies twice. */
  async applyDamage(
    campaignId: string,
    encounterId: string,
    pendingDamageId: string,
    amount?: number,
    key?: string,
  ): Promise<DamageResult> {
    const res = await this.keyed(
      ['applyPendingDamage', campaignId, encounterId, pendingDamageId, amount],
      (sent) =>
        this.client.applyPendingDamage({
          campaignId,
          encounterId,
          pendingDamageId,
          idempotencyKey: sent,
          amount,
        }),
      key,
    );
    return {
      encounter: need(res.encounter, 'ApplyPendingDamage'),
      pending: need(res.pendingDamage, 'ApplyPendingDamage'),
      cast: [],
    };
  }

  async discardDamage(
    campaignId: string,
    encounterId: string,
    pendingDamageId: string,
  ): Promise<DamageResult> {
    const res = await this.keyed(
      ['discardPendingDamage', campaignId, encounterId, pendingDamageId],
      (sent) =>
        this.client.discardPendingDamage({
          campaignId,
          encounterId,
          pendingDamageId,
          idempotencyKey: sent,
        }),
    );
    return {
      encounter: need(res.encounter, 'DiscardPendingDamage'),
      pending: need(res.pendingDamage, 'DiscardPendingDamage'),
      cast: [],
    };
  }

  /** The master answers for the target: cast Escudo with `slot`. */
  async useReaction(
    campaignId: string,
    encounterId: string,
    pendingDamageId: string,
    slot: { level: number; pact: boolean },
    key: string,
  ): Promise<ReactionResult> {
    const res = await this.client.useReaction({
      campaignId,
      encounterId,
      pendingDamageId,
      slot,
      idempotencyKey: key,
    });
    return { encounter: need(res.encounter, 'UseReaction'), outcome: res.outcome };
  }

  /** The master lets the hit go ("Seguir sem Escudo"). */
  async declineReaction(
    campaignId: string,
    encounterId: string,
    pendingDamageId: string,
    key: string,
  ): Promise<Encounter> {
    const res = await this.client.declineReaction({
      campaignId,
      encounterId,
      pendingDamageId,
      idempotencyKey: key,
    });
    return need(res.encounter, 'DeclineReaction');
  }

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
  ): Promise<ActionResult> {
    const res = await this.keyed(
      ['takeAction', campaignId, encounterId, combatantId, actionKey, die],
      (sent) =>
        this.client.takeAction({
          campaignId,
          encounterId,
          combatantId,
          actionKey,
          idempotencyKey: sent,
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
    die: AttackDie | PoolDie | null,
    key: string,
    summon?: SummonRequest,
    damageTypeKey = '',
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
      roll: !die
        ? { case: undefined }
        : 'inApp' in die
          ? { case: 'rollInApp', value: true }
          : 'poolSum' in die
            ? { case: 'poolSum', value: die.poolSum }
            : { case: 'd20Face', value: die.face },
      summon: summon
        ? {
            option: summon.option,
            creatureKeys: [...summon.creatureKeys],
            names: [...(summon.names ?? [])],
          }
        : undefined,
      area: areaOneof(extras.area),
      // Left out unless the master chose: the table rule decides then.
      ...(extras.revealHidden === undefined ? {} : { revealHidden: extras.revealHidden }),
    });
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

function toHitPoints(hp: MonsterHp | undefined): MonsterHitPoints {
  return hp === 'rolled' ? MonsterHitPoints.ROLLED : MonsterHitPoints.AVERAGE;
}

function toParticipant(spec: JoinSpec): MessageInitShape<typeof ParticipantSchema> {
  return { characterId: spec.characterId, count: spec.count ?? 1, hidden: spec.hidden };
}

function need<T>(value: T | undefined, call: string): T {
  if (value === undefined) {
    throw new Error(`${call} answered without its result`);
  }
  return value;
}
