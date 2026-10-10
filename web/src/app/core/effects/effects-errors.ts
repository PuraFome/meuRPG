import { Code } from '@connectrpc/connect';

import { combatErrorMessage, encounterBlocked, sessionClosed } from '../combat/combat-errors';
import { describeConnectError } from '../connect/connect-errors';

/**
 * Where the call was made: the master's panel of a combat, the one of the characters outside a combat, the exhaustion
 * dialog, or "Dar efeito" (an effect given to characters outside a combat).
 */
export type EffectsScope = 'combat' | 'character' | 'exhaustion' | 'give';

/**
 * The Portuguese message for a failed call of `LastingEffectService`, by code and typed detail (the service's comment
 * lists them). `what` finishes "Não deu para …". A refusal of the encounter's state (`EncounterBlocked`) and a closed
 * session read as in the rest of the combat; the codes the effects calls add are worded here.
 */
export function effectsErrorMessage(err: unknown, what: string, scope: EffectsScope): string {
  if (encounterBlocked(err) || sessionClosed(err)) {
    return combatErrorMessage(err, what);
  }
  return describeConnectError(err, {
    [Code.InvalidArgument]: `Não deu para ${what}: confira os campos e tente de novo.`,
    [Code.NotFound]:
      scope === 'combat'
        ? 'Esse efeito, ou o combate, não existe mais. A tela foi atualizada.'
        : 'Esse efeito, ou o personagem, não existe mais. A tela foi atualizada.',
    ...(scope === 'give'
      ? {
          [Code.FailedPrecondition]:
            'Um dos personagens está em um combate. Dê o efeito por lá, em "Adicionar efeito".',
        }
      : {}),
    [Code.PermissionDenied]: 'Só o mestre faz isso.',
    [Code.Aborted]:
      scope === 'exhaustion'
        ? 'O nível de exaustão mudou enquanto você agia. Feche e abra de novo para ver o nível de agora.'
        : 'Os efeitos mudaram enquanto você agia. A tela foi atualizada; tente de novo.',
    [Code.Unavailable]: `Não deu para ${what}: o servidor não respondeu. Tente de novo.`,
  });
}
