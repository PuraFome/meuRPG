import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import { ChargesBlockedReason, ChargesBlockedSchema } from '../../../gen/meurpg/play/v1/charges_pb';
import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
  ItemRechargeKind,
  ItemRechargeSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import {
  EncounterBlockedReason,
  EncounterBlockedSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  GameSessionBlockedReason,
  GameSessionBlockedSchema,
} from '../../../gen/meurpg/play/v1/play_pb';
import {
  chargesBlocked,
  chargesBlockedMessage,
  chargesErrorMessage,
  consumableErrorMessage,
} from './charges-errors';

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

describe('consumableErrorMessage', () => {
  const encounter = (reason: EncounterBlockedReason) =>
    new ConnectError('x', Code.FailedPrecondition, undefined, [
      { desc: EncounterBlockedSchema, value: create(EncounterBlockedSchema, { reason }) },
    ]);
  const potion = { kind: 'potion' as const, targetName: 'Brisa' };

  it('says the receiver is not at the giver side any more, with their name, and that nothing was spent', () => {
    expect(
      consumableErrorMessage(encounter(EncounterBlockedReason.TARGET_NOT_ADJACENT), potion),
    ).toBe('Brisa não está mais ao seu lado. Nada foi gasto.');
  });

  it('says it for a receiver the sheet did not name', () => {
    expect(
      consumableErrorMessage(encounter(EncounterBlockedReason.TARGET_NOT_ADJACENT), {
        kind: 'potion',
      }),
    ).toBe('Quem ia receber não está mais ao seu lado. Nada foi gasto.');
  });

  it('says the item is already used and that the sheet was updated, by the kind of item', () => {
    const gone = encounter(EncounterBlockedReason.ITEM_GONE);
    expect(consumableErrorMessage(gone, potion)).toBe(
      'Essa poção já foi usada. A ficha foi atualizada.',
    );
    expect(consumableErrorMessage(gone, { kind: 'scroll' })).toBe(
      'Esse pergaminho já foi usado. A ficha foi atualizada.',
    );
  });

  it('says it is not the turn of who uses it, for the player and for the master', () => {
    const turn = encounter(EncounterBlockedReason.NOT_YOUR_TURN);
    expect(consumableErrorMessage(turn, potion)).toBe(
      'Só na sua vez: beber ou dar gasta a ação.',
    );
    expect(consumableErrorMessage(turn, { ...potion, actingFor: 'Tor' })).toBe(
      'Só na vez de Tor: beber ou dar gasta a ação.',
    );
  });

  it('says the action of the turn is spent', () => {
    const used = encounter(EncounterBlockedReason.ACTION_USED);
    expect(consumableErrorMessage(used, potion)).toBe('Você já usou a ação desta vez.');
    expect(consumableErrorMessage(used, { ...potion, actingFor: 'Tor' })).toBe(
      'Tor já usou a ação desta vez.',
    );
  });

  it('says who is down does not act but may still receive', () => {
    expect(
      consumableErrorMessage(encounter(EncounterBlockedReason.COMBATANT_DOWN), potion),
    ).toBe('Quem está caído não age, mas ainda pode receber uma poção.');
  });

  it('leaves the other refusals of a combat to the words the combat has', () => {
    expect(
      consumableErrorMessage(encounter(EncounterBlockedReason.REACTION_PENDING), potion),
    ).toBe(
      'Uma reação ainda espera resposta. Espere a resposta ou responda no lugar de quem reage.',
    );
  });

  it('says a dead character neither uses nor receives', () => {
    const dead = new ConnectError('x', Code.FailedPrecondition, undefined, [
      {
        desc: CharacterBlockedSchema,
        value: create(CharacterBlockedSchema, { reason: CharacterBlockedReason.CHARACTER_DEAD }),
      },
    ]);
    expect(consumableErrorMessage(dead, potion)).toBe(
      'Esse personagem está morto: não usa nem recebe itens. Nada foi gasto.',
    );
  });

  it('says there is no open session', () => {
    const closed = new ConnectError('x', Code.FailedPrecondition, undefined, [
      {
        desc: GameSessionBlockedSchema,
        value: create(GameSessionBlockedSchema, {
          reason: GameSessionBlockedReason.NO_OPEN_SESSION,
        }),
      },
    ]);
    expect(consumableErrorMessage(closed, potion)).toBe(
      'Sem sessão aberta: só dá para usar durante a sessão. Nada foi gasto.',
    );
  });

  it('says the typed faces are not those of the dice', () => {
    expect(consumableErrorMessage(new ConnectError('x', Code.InvalidArgument), potion)).toBe(
      'Os dados digitados não valem: cada um vai de 1 até o número de faces dele. Nada foi gasto.',
    );
  });

  it('says a receiver who is not there any more, by name', () => {
    expect(consumableErrorMessage(new ConnectError('x', Code.NotFound), potion)).toBe(
      'Brisa não está mais na mesa. Nada foi gasto.',
    );
  });

  it('says nothing was spent when the call did not get through, naming what was being used', () => {
    const down = new ConnectError('x', Code.Unavailable);
    expect(consumableErrorMessage(down, potion)).toBe(
      'Não deu para usar a poção. Nada foi gasto. Tente de novo.',
    );
    expect(consumableErrorMessage(down, { kind: 'scroll' })).toBe(
      'Não deu para usar o pergaminho. Nada foi gasto. Tente de novo.',
    );
  });
});
