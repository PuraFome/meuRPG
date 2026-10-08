import { Code, ConnectError } from '@connectrpc/connect';

import { InvalidFieldSchema } from '../../../gen/meurpg/characters/v1/characters_pb';
import { describeConnectError } from '../connect/connect-errors';

/** What the master was doing in the bestiary, to finish "Não deu para …". */
export type BestiaryAction = 'list' | 'read' | 'create';

/** The request field a refused call names in its `InvalidField` detail ("name", "idempotency_key"...), if any. */
export function invalidFieldOf(err: unknown): string | undefined {
  return ConnectError.from(err, Code.Unavailable).findDetails(InvalidFieldSchema)[0]?.field;
}

/**
 * Whether a refused "Criar NPC" means the NPC of this dialog already exists: the server answers a key it
 * already used for another request with `invalid_argument` on `idempotency_key`.
 */
export function npcAlreadyMade(err: unknown): boolean {
  return (
    ConnectError.from(err, Code.Unavailable).code === Code.InvalidArgument &&
    invalidFieldOf(err) === 'idempotency_key'
  );
}

/**
 * The Portuguese message for a failed call of the bestiary (MR-042): what happened and how to fix
 * it, by code and by the field of an `invalid_argument`, never by the message. `ListCreatures` and
 * `GetCreature` refuse a stranger with `not_found`; `CreateNpcFromCreature` refuses a player with
 * `permission_denied` and a bad request with `invalid_argument` naming the field.
 */
export function bestiaryErrorMessage(err: unknown, action: BestiaryAction): string {
  const what =
    action === 'create'
      ? 'criar o NPC'
      : action === 'read'
        ? 'abrir a ficha da criatura'
        : 'abrir o bestiário';
  let invalid = `Não deu para ${what}: confira a busca e tente de novo.`;
  if (action === 'create') {
    switch (invalidFieldOf(err)) {
      case 'name':
        invalid =
          'Não deu para criar o NPC: o nome precisa ter de 1 a 80 letras, numa linha só. Confira e tente de novo.';
        break;
      case 'creature_key':
        invalid = 'Essa criatura não está no bestiário. Volte à lista e escolha outra.';
        break;
      case 'kind':
        invalid = 'Escolha Minion ou NPC de história.';
        break;
      default:
        invalid = 'Não deu para criar o NPC: algo no pedido não vale. Feche e abra de novo.';
    }
  }
  return describeConnectError(err, {
    [Code.InvalidArgument]: invalid,
    [Code.NotFound]:
      action === 'read'
        ? 'Essa criatura não existe no bestiário, ou você não é mais da campanha.'
        : 'Essa campanha não existe, ou você não é mais membro dela. Volte para Minhas campanhas.',
    [Code.PermissionDenied]: 'Só o mestre da campanha usa o bestiário e faz NPCs.',
    [Code.ResourceExhausted]:
      'A campanha chegou ao limite de 1.000 personagens e NPCs. Apague um para criar outro.',
    [Code.Unavailable]: `Não deu para ${what}: o servidor não respondeu. Tente de novo.`,
  });
}
