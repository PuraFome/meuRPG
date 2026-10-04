import { Code, ConnectError } from '@connectrpc/connect';

import { GameSessionBlockedReason, GameSessionBlockedSchema } from '../../../gen/meurpg/play/v1/play_pb';
import {
  EncounterBlockedReason,
  type EncounterBlocked,
  EncounterBlockedSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { describeConnectError } from '../connect/connect-errors';
import { Recharge } from '../../../gen/meurpg/rules/v1/rules_pb';
import { circleLabel } from './combat-grid';
import { metersText } from '../units';

/** The typed detail of a `failed_precondition` from `CombatService`, or
 * `null` (another code, or another detail). Never read from the message. */
export function encounterBlocked(err: unknown): EncounterBlocked | null {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code !== Code.FailedPrecondition) {
    return null;
  }
  return connectErr.findDetails(EncounterBlockedSchema)[0] ?? null;
}

/** Whether the campaign has no open session (`GameSessionBlocked`). */
export function sessionClosed(err: unknown): boolean {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  return (
    connectErr.code === Code.FailedPrecondition &&
    connectErr.findDetails(GameSessionBlockedSchema)[0]?.reason ===
      GameSessionBlockedReason.NO_OPEN_SESSION
  );
}

/** When a feature's uses come back, for "Sem usos: volta num descanso curto". */
export function rechargeText(recharge: Recharge): string {
  switch (recharge) {
    case Recharge.SHORT_REST:
      return 'volta num descanso curto';
    case Recharge.LONG_REST:
      return 'volta num descanso longo';
    case Recharge.DAWN:
      return 'volta ao amanhecer';
    default:
      return 'só o mestre devolve';
  }
}

/** Why a move was refused, for the reach line under the map (E6-10). */
export function blockedMessage(blocked: EncounterBlocked): string {
  switch (blocked.reason) {
    case EncounterBlockedReason.ENCOUNTER_ALREADY_OPEN:
      return 'A sessão já tem um combate em andamento.';
    case EncounterBlockedReason.NO_CURRENT_MAP:
      return 'Escolha o mapa do combate antes de iniciar.';
    case EncounterBlockedReason.MAP_HAS_NO_GRID:
      return 'Esse mapa ainda não tem grade. Defina a grade para iniciar o combate.';
    case EncounterBlockedReason.NOT_IN_SETUP:
      return 'O combate já começou.';
    case EncounterBlockedReason.NOT_ACTIVE:
      return 'O combate não está em andamento.';
    case EncounterBlockedReason.ENCOUNTER_ENDED:
      return 'Esse combate já terminou.';
    case EncounterBlockedReason.INITIATIVE_MISSING:
      return 'Ainda falta a iniciativa de alguém.';
    case EncounterBlockedReason.INITIATIVE_ALREADY_SET:
      return 'Sua iniciativa já foi rolada.';
    case EncounterBlockedReason.WRONG_DICE_MODE:
      return 'A campanha mudou a forma de rolar os dados. Recarregue a página.';
    case EncounterBlockedReason.NOT_YOUR_TURN:
      return 'Não é a sua vez.';
    case EncounterBlockedReason.NOT_PLACED:
      return 'Esse combatente ainda não está no mapa.';
    case EncounterBlockedReason.TOO_FAR:
      return `Longe demais: faltam ${metersText(blocked.missingFt)}`;
    case EncounterBlockedReason.SQUARE_OCCUPIED:
      return 'Ocupado: escolha outro quadrado.';
    case EncounterBlockedReason.PLAYER_IN_COMBAT:
      return 'Um jogador só sai do combate antes de ele começar.';
    case EncounterBlockedReason.ACTION_USED:
      return 'Sua ação já foi usada neste turno.';
    case EncounterBlockedReason.BONUS_ACTION_USED:
      return 'Sua ação bônus já foi usada neste turno.';
    case EncounterBlockedReason.ATTACKS_USED:
      return 'Os ataques desta ação já foram usados.';
    case EncounterBlockedReason.TARGET_OUT_OF_REACH:
      return `Longe demais: faltam ${metersText(blocked.missingFt)} para chegar ao alvo.`;
    case EncounterBlockedReason.TARGET_DEFEATED:
      return 'Esse alvo já foi derrotado. Escolha outro.';
    case EncounterBlockedReason.PENDING_DAMAGE:
      return 'Ainda falta rolar ou aplicar o dano do ataque antes de passar o turno.';
    case EncounterBlockedReason.DAMAGE_ALREADY_ROLLED:
      return 'O dano desse ataque já foi rolado.';
    case EncounterBlockedReason.DAMAGE_NOT_ROLLED:
      return 'O dano ainda não foi rolado.';
    case EncounterBlockedReason.DAMAGE_RESOLVED:
      return 'Esse dano já foi aplicado ou descartado.';
    case EncounterBlockedReason.NOTHING_TO_UNDO:
      return 'Não há mais nada para desfazer: só a última ação pode ser desfeita.';
    case EncounterBlockedReason.COMBATANT_DOWN:
      return 'Quem está caído não age.';
    case EncounterBlockedReason.REACTION_PENDING:
      return 'Esse acerto espera a reação do alvo (Escudo). Espere o jogador ou responda por ele.';
    case EncounterBlockedReason.NOT_AWAITING_REACTION:
      return 'Esse acerto não espera mais uma reação. A tela foi atualizada.';
    case EncounterBlockedReason.REACTION_USED:
      return 'A reação já foi usada: ela volta no começo da sua vez.';
    case EncounterBlockedReason.NO_SLOT:
      return blocked.minLevel > 0
        ? `Não há espaço de ${circleLabel(blocked.minLevel)} ou maior livre.`
        : 'Não há espaço de magia livre.';
    case EncounterBlockedReason.NO_USES:
      return `Sem usos: ${rechargeText(blocked.recharge)}.`;
    case EncounterBlockedReason.DEATH_SAVE_DUE:
      return 'Role o teste contra a morte antes de encerrar o turno.';
    case EncounterBlockedReason.DEATH_SAVE_NOT_DUE:
      return 'Não há teste contra a morte para rolar agora. A tela foi atualizada.';
    case EncounterBlockedReason.NOT_DYING:
      return 'Esse personagem não falhou três testes contra a morte. A tela foi atualizada.';
    default:
      return 'O combate não está num estado que aceite isso. A tela foi atualizada.';
  }
}

/** The Portuguese message for a failed combat call: what happened and how to
 * fix it, by code and by typed detail (combat.proto lists what each call
 * returns). `what` finishes "Não deu para …". */
export function combatErrorMessage(err: unknown, what = 'fazer isso'): string {
  const blocked = encounterBlocked(err);
  if (blocked) {
    return blockedMessage(blocked);
  }
  if (sessionClosed(err)) {
    return 'A sessão acabou: o combate só muda durante a sessão.';
  }
  return describeConnectError(err, {
    [Code.InvalidArgument]: `Não deu para ${what}: confira os campos e tente de novo.`,
    [Code.NotFound]: 'Esse combate não existe mais, ou você não o vê. Recarregue a página.',
    [Code.PermissionDenied]: 'Você não pode fazer isso agora.',
    [Code.Aborted]: 'O combate mudou enquanto você agia. A tela foi atualizada; tente de novo.',
    [Code.Unavailable]: `Não deu para ${what}: o servidor não respondeu. Tente de novo.`,
  });
}
