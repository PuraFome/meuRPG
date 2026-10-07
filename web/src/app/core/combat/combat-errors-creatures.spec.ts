import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  EncounterBlockedReason,
  EncounterBlockedSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { combatErrorMessage } from './combat-errors';

const blocked = (reason: EncounterBlockedReason) =>
  new ConnectError('x', Code.FailedPrecondition, undefined, [
    { desc: EncounterBlockedSchema, value: create(EncounterBlockedSchema, { reason }) },
  ]);

describe('the refusals of the creatures and Wild Shape, by reason (MR-037)', () => {
  it.each([
    [
      EncounterBlockedReason.WILD_SHAPE_NO_SPELLS,
      'Na forma de fera não dá para conjurar. Volte à forma normal e tente de novo.',
    ],
    [EncounterBlockedReason.ALREADY_IN_WILD_SHAPE, 'Você já está na forma de uma fera.'],
    [
      EncounterBlockedReason.TOO_MANY_COMBATANTS,
      'Não cabem mais combatentes neste combate. Nada foi gasto.',
    ],
    [EncounterBlockedReason.SUMMON_NEEDS_INITIATIVE, 'Falta o d20 da iniciativa das criaturas.'],
    [
      EncounterBlockedReason.CASTING_TIME_TOO_LONG,
      'Essa magia leva mais tempo do que um combate dá. Conjure fora do combate.',
    ],
  ])('%s', (reason, text) => {
    expect(combatErrorMessage(blocked(reason))).toBe(text);
  });
});
