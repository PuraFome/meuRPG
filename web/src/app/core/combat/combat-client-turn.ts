import {
  type Encounter,
  EncounterMode,
  HitRiderChoice,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  type JoinSpec,
  type MonsterHp,
  type StartExtras,
  type AddMonstersResult,
  type InitiativeRoll,
} from './combat-client-types';
import { toHitPoints, toParticipant, need } from './combat-client-wire';
import { CombatClientBase } from './combat-client-base';

/** The calls that set a combat up and pass its turns. */
export abstract class CombatTurnCalls extends CombatClientBase {
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

  /** "A sua fúria vai acabar?": `endRage` true lets the rage end and passes the turn, false goes back to the turn. */
  async answerRageEnd(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    endRage: boolean,
  ): Promise<Encounter> {
    const res = await this.keyed(
      ['answerRageEnd', campaignId, encounterId, combatantId, endRage],
      (sent) =>
        this.client.answerRageEnd({
          campaignId,
          encounterId,
          combatantId,
          endRage,
          idempotencyKey: sent,
        }),
    );
    return need(res.encounter, 'AnswerRageEnd');
  }

  /** `UseHitRider`: the monk's answer to an offer after a hit (Open Hand technique, Golpe Atordoante). */
  async useHitRider(
    campaignId: string,
    encounterId: string,
    riderId: string,
    choice: HitRiderChoice,
  ): Promise<Encounter> {
    const res = await this.keyed(
      ['useHitRider', campaignId, encounterId, riderId, choice],
      (sent) =>
        this.client.useHitRider({ campaignId, encounterId, riderId, choice, idempotencyKey: sent }),
    );
    return need(res.encounter, 'UseHitRider');
  }

  /** "Encerrar fúria": the bonus action that ends a rage. */
  async endRage(campaignId: string, encounterId: string, combatantId: string): Promise<Encounter> {
    const res = await this.keyed(['endRage', campaignId, encounterId, combatantId], (sent) =>
      this.client.endRage({ campaignId, encounterId, combatantId, idempotencyKey: sent }),
    );
    return need(res.encounter, 'EndRage');
  }
}
