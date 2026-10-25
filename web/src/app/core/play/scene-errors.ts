import { Code, ConnectError } from '@connectrpc/connect';

import {
  GameSessionBlockedReason,
  GameSessionBlockedSchema,
} from '../../../gen/meurpg/play/v1/play_pb';
import {
  SceneBlockedReason,
  type SceneBlocked,
  SceneBlockedSchema,
} from '../../../gen/meurpg/play/v1/scene_pb';
import { pendingQuestionReasonText } from '../combat/combat-errors';
import { describeConnectError } from '../connect/connect-errors';
import { resourceBlocked } from '../resources/resources-errors';

/** The typed detail of a `failed_precondition` from a scene call, or `null`.
 * Never read from the message. */
export function sceneBlocked(err: unknown): SceneBlocked | null {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code !== Code.FailedPrecondition) {
    return null;
  }
  return connectErr.findDetails(SceneBlockedSchema)[0] ?? null;
}

/** What each refusal says, in words the master or the player can act on. */
export function sceneBlockedMessage(reason: SceneBlockedReason): string {
  switch (reason) {
    case SceneBlockedReason.NO_ACTIONS:
      return 'Essa cena não tem ações. Adicione ações no editor do mapa.';
    case SceneBlockedReason.NO_OPEN_SCENE:
      return 'O mestre fechou a cena. Espere ele abrir de novo.';
    case SceneBlockedReason.ALREADY_ROLLED:
      return 'Você não tem mais tentativas nessa ação. O mestre pode dar mais uma.';
    case SceneBlockedReason.WRONG_DICE_MODE:
      return 'A campanha mudou a forma de rolar os dados. Recarregue a página.';
    case SceneBlockedReason.NO_CHARACTER:
      return 'Você não tem um personagem vivo nesta campanha para rolar.';
    case SceneBlockedReason.STAGE_FULL:
      return 'A cena comporta 4 NPCs. Tire um para pôr outro.';
    case SceneBlockedReason.SCENE_NOT_PARTIAL:
      return 'A cena está aberta para toda a mesa: não há quem precise entrar.';
    default:
      return 'A cena não está num estado que aceite isso.';
  }
}

/** The Portuguese message for a failed scene call, by code and typed detail
 * (play.proto lists what each call returns). `what` finishes "Não deu para …". */
export function sceneErrorMessage(err: unknown, what = 'fazer isso'): string {
  const blocked = sceneBlocked(err);
  if (blocked) {
    return sceneBlockedMessage(blocked.reason);
  }
  // Another roll of the character waits for its question (the halfling's Lucky, a Bardic Inspiration die).
  const pending = pendingQuestionReasonText(resourceBlocked(err)?.reason);
  if (pending) {
    return pending;
  }
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (
    connectErr.code === Code.FailedPrecondition &&
    connectErr.findDetails(GameSessionBlockedSchema)[0]?.reason ===
      GameSessionBlockedReason.NO_OPEN_SESSION
  ) {
    return 'A sessão acabou: a cena só muda durante a sessão.';
  }
  return describeConnectError(err, {
    [Code.InvalidArgument]: `Não deu para ${what}: confira o número e tente de novo.`,
    [Code.NotFound]: 'Essa cena não existe mais, ou você não a vê. Recarregue a página.',
    [Code.PermissionDenied]: 'Você não pode fazer isso agora.',
    [Code.Unavailable]: `Não deu para ${what}: o servidor não respondeu. Tente de novo.`,
  });
}

/** What a failed stage call (`PutOnStage`, `TakeOffStage`, `SetSpeaker`) says
 * to the master, by code and typed detail. A scene that closed meanwhile is
 * not "the master closed the scene" here: the master did it. */
export function stageErrorMessage(err: unknown): string {
  const blocked = sceneBlocked(err);
  if (blocked?.reason === SceneBlockedReason.NO_OPEN_SCENE) {
    return 'Não há cena aberta, então não há palco. Abra uma cena para pôr NPCs em cena.';
  }
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code === Code.NotFound) {
    return 'Esse NPC não está mais na campanha ou não está em cena. A tela foi atualizada.';
  }
  return sceneErrorMessage(err, 'mudar o palco');
}

/** What a failed "Quem está na cena" call (`SetSceneAudience`, `SetScenePlayersMayJoin`) says to the master. */
export function sceneAudienceErrorMessage(err: unknown): string {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code === Code.InvalidArgument) {
    return 'Alguém dessa lista não joga mais nesta campanha. A lista foi atualizada: escolha de novo.';
  }
  return sceneErrorMessage(err, 'mudar quem está na cena');
}

/** What a failed "Mostrar" / "Mudar quem vê" call says to the master (MR-028). */
export function imageAudienceErrorMessage(err: unknown): string {
  const blocked = sceneBlocked(err);
  if (blocked?.reason === SceneBlockedReason.SCENE_NOT_PARTIAL) {
    return 'A cena não está aberta só para alguns jogadores. Mostre a imagem para a mesa toda ou escolha quem vê.';
  }
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code === Code.InvalidArgument) {
    return 'Alguém dessa lista não joga mais nesta campanha. A lista foi atualizada: escolha de novo.';
  }
  if (connectErr.code === Code.NotFound) {
    return 'Nenhuma imagem está sendo mostrada. A tela foi atualizada.';
  }
  return sceneErrorMessage(err, 'mudar quem vê a imagem');
}
