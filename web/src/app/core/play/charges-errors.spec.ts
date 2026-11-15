import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import { ChargesBlockedReason, ChargesBlockedSchema } from '../../../gen/meurpg/play/v1/charges_pb';
import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
  ItemRechargeKind,
  ItemRechargeSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { chargesBlocked, chargesBlockedMessage, chargesErrorMessage } from './charges-errors';

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
  });
});

describe('chargesErrorMessage', () => {
  const blocked = (reason: CharacterBlockedReason) =>
    new ConnectError('x', Code.FailedPrecondition, undefined, [
      { desc: CharacterBlockedSchema, value: create(CharacterBlockedSchema, { reason }) },
    ]);
  const item = { itemName: 'Varinha de mísseis mágicos', left: 1 };

  it('says the charges of the item are short, with its name and what is left', () => {
    expect(chargesErrorMessage(blocked(CharacterBlockedReason.CHARGES_NOT_ENOUGH), item)).toBe(
      'Cargas a menos. A Varinha de mísseis mágicos só tem 1 carga.',
    );
    expect(
      chargesErrorMessage(blocked(CharacterBlockedReason.CHARGES_NOT_ENOUGH), { ...item, left: 3 }),
    ).toBe('Cargas a menos. A Varinha de mísseis mágicos só tem 3 cargas.');
  });

  it('says an item that comes back at dawn does so, as the board has it', () => {
    expect(
      chargesErrorMessage(blocked(CharacterBlockedReason.ITEM_NO_CHARGES), {
        itemName: 'Varinha de segredos',
        left: 0,
        recharge: create(ItemRechargeSchema, { kind: ItemRechargeKind.DAWN, regainDice: '1d3' }),
      }),
    ).toBe('Sem cargas. A Varinha de segredos volta ao amanhecer.');
  });

  it('says an item with no charges and a destroyed item by name', () => {
    expect(chargesErrorMessage(blocked(CharacterBlockedReason.ITEM_NO_CHARGES), item)).toBe(
      'Sem cargas. A Varinha de mísseis mágicos não tem cargas agora.',
    );
    expect(chargesErrorMessage(blocked(CharacterBlockedReason.ITEM_DESTROYED), item)).toBe(
      'Item destruído. A Varinha de mísseis mágicos não tem mais uso.',
    );
  });

  it('words a refusal of the charges service by its typed detail', () => {
    const err = new ConnectError('x', Code.FailedPrecondition, undefined, [
      {
        desc: ChargesBlockedSchema,
        value: create(ChargesBlockedSchema, { reason: ChargesBlockedReason.COMBAT_RUNNING }),
      },
    ]);
    expect(chargesErrorMessage(err, item)).toContain('combate');
  });

  it('says nothing changed when the call did not get through', () => {
    expect(chargesErrorMessage(new ConnectError('x', Code.Unavailable), item)).toBe(
      'Não foi possível gastar a carga. Nada mudou. Tente de novo.',
    );
  });

  it('words a failure to save a correction or a dawn, not a spend, when the caller says what it was doing', () => {
    const down = new ConnectError('x', Code.Unavailable);
    expect(chargesErrorMessage(down, item, 'salvar')).toBe(
      'Não foi possível salvar. Nada mudou. Tente de novo.',
    );
    expect(chargesErrorMessage(down, item, 'amanhecer')).toBe(
      'Não foi possível amanhecer. Nada mudou. Tente de novo.',
    );
  });

  it('falls back to the character sentences for the other refusals', () => {
    expect(chargesErrorMessage(blocked(CharacterBlockedReason.CHARACTER_DEAD), item)).toContain(
      'morto',
    );
  });
});
