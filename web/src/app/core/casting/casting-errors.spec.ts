import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { CastingBlockedReason, CastingBlockedSchema } from '../../../gen/meurpg/play/v1/casting_pb';
import {
  EncounterBlockedReason,
  EncounterBlockedSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';

import { castingErrorText } from './casting-errors';

const REFUSAL = 'Com armadura ou escudo sem proficiência não dá para conjurar.';

describe('the refusal of a cast by armor without proficiency', () => {
  it('reads the casting reason', () => {
    const err = new ConnectError('x', Code.FailedPrecondition, undefined, [
      {
        desc: CastingBlockedSchema,
        value: create(CastingBlockedSchema, { reason: CastingBlockedReason.ARMOR_NOT_PROFICIENT }),
      },
    ]);
    expect(castingErrorText(err)).toBe(REFUSAL);
  });

  it('reads the encounter reason of a summon or a cast in a combat', () => {
    const err = new ConnectError('x', Code.FailedPrecondition, undefined, [
      {
        desc: EncounterBlockedSchema,
        value: create(EncounterBlockedSchema, {
          reason: EncounterBlockedReason.ARMOR_NOT_PROFICIENT,
        }),
      },
    ]);
    expect(castingErrorText(err)).toBe(REFUSAL);
  });
});
