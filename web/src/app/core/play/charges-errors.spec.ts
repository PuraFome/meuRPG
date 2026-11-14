import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import { CharacterBlockedReason } from '../../../gen/meurpg/characters/v1/characters_pb';
import { ChargesBlockedReason, ChargesBlockedSchema } from '../../../gen/meurpg/play/v1/charges_pb';
import { chargesBlocked, chargesBlockedMessage, itemBlockedMessage } from './charges-errors';

describe('charges errors', () => {
  it('reads the typed detail of a failed precondition and nothing else', () => {
    const detail = create(ChargesBlockedSchema, {
      reason: ChargesBlockedReason.REGAIN_FACE_NEEDED,
      needed: 6,
    });
    const err = new ConnectError('x', Code.FailedPrecondition, undefined, [
      { desc: ChargesBlockedSchema, value: detail },
    ]);
    expect(chargesBlocked(err)?.needed).toBe(6);
    expect(chargesBlocked(new ConnectError('x', Code.NotFound))).toBeNull();
  });

  it('says what each refusal means', () => {
    const say = (reason: ChargesBlockedReason) =>
      chargesBlockedMessage(create(ChargesBlockedSchema, { reason, needed: 4 }));
    expect(say(ChargesBlockedReason.REGAIN_FACE_NEEDED)).toContain('d4');
    expect(say(ChargesBlockedReason.COMBAT_RUNNING)).toContain('combate');
    expect(say(ChargesBlockedReason.ALREADY_RECOVERED)).toContain('recolhida');
    expect(say(ChargesBlockedReason.UNSPECIFIED)).toContain('agora');
    expect(itemBlockedMessage(CharacterBlockedReason.CHARGES_NOT_ENOUGH)).toContain('tantas');
    expect(itemBlockedMessage(CharacterBlockedReason.ITEM_DESTROYED)).toContain('destruído');
    expect(itemBlockedMessage(CharacterBlockedReason.ITEM_NO_CHARGES)).toContain('cargas');
    expect(itemBlockedMessage(CharacterBlockedReason.CHARACTER_DEAD)).toBeNull();
  });
});
