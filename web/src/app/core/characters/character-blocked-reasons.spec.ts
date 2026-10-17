import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { describeCharacterError } from './character-errors';

/** What `describeCharacterError` says when it has no sentence for a reason. */
const GENERIC = 'Não foi possível concluir a ação agora.';

/** Reasons that are meant to reach the person with the generic sentence, each with why. */
const GENERIC_ON_PURPOSE = new Map<CharacterBlockedReason, string>([
  [CharacterBlockedReason.UNSPECIFIED, 'the zero value: the server never sends it on purpose'],
]);

function blocked(reason: CharacterBlockedReason): ConnectError {
  return new ConnectError('blocked', Code.FailedPrecondition, undefined, [
    { desc: CharacterBlockedSchema, value: { reason } },
  ]);
}

/** Every value of the generated enum, so a reason added to characters.proto is checked without touching this spec. */
const REASONS = Object.entries(CharacterBlockedReason)
  .filter((entry): entry is [string, CharacterBlockedReason] => typeof entry[1] === 'number')
  .map(([name, value]) => ({ name, value }));

describe('CharacterBlockedReason sentences', () => {
  it('reads the reasons from the generated enum', () => {
    expect(REASONS.length).toBeGreaterThan(20);
  });

  it('has a Portuguese sentence of its own for every reason the proto declares', () => {
    const missing = REASONS.filter(
      ({ value }) =>
        !GENERIC_ON_PURPOSE.has(value) && describeCharacterError(blocked(value)) === GENERIC,
    ).map(({ name }) => name);
    expect(missing).toEqual([]);
  });

  it('keeps apologies out of every sentence', () => {
    for (const { name, value } of REASONS) {
      expect(describeCharacterError(blocked(value)), name).not.toMatch(
        /desculp|infelizmente|sentimos/i,
      );
    }
  });
});
