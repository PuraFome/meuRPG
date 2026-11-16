import type { Combatant, Encounter } from '../../../../gen/meurpg/play/v1/combat_pb';
import type { CombatClient } from '../../../core/combat/combat-client';
import type { MoveResult } from '../../../core/combat/combat-client-types';
import type { JumpRequest } from './move-page/move-page';

/**
 * The jump's call (E9-06): a long jump goes to the square, a high one stays where it is and rises. The same jump again (a lost answer)
 * is a retry: it keeps its `key` and is not charged twice.
 */
export function jumpCall(
  api: Pick<CombatClient, 'moves'>,
  campaignId: string,
  e: Encounter,
  own: Combatant,
  req: JumpRequest,
  key: string,
): Promise<MoveResult> {
  return req.kind === 'long'
    ? api.moves.move(
        campaignId,
        e.id,
        own.id,
        req.square.col,
        req.square.row,
        { kind: 'long' },
        key,
      )
    : api.moves.move(
        campaignId,
        e.id,
        own.id,
        own.col,
        own.row,
        { kind: 'high', heightDft: req.heightDft },
        key,
      );
}

/**
 * The walk's call of the "Mover" page. The master's page of a readied move ("Usar pelo jogador") sends `release_readied`, so the walk is the
 * release; every other walk is plain, and a master's drag of the readier stays one (#857).
 */
export function walkCall(
  api: Pick<CombatClient, 'moves'>,
  campaignId: string,
  e: Encounter,
  own: Combatant,
  to: { readonly col: number; readonly row: number },
  releasing: boolean,
): Promise<MoveResult> {
  return releasing
    ? api.moves.move(
        campaignId,
        e.id,
        own.id,
        to.col,
        to.row,
        undefined,
        undefined,
        false,
        '',
        true,
      )
    : api.moves.move(campaignId, e.id, own.id, to.col, to.row);
}
