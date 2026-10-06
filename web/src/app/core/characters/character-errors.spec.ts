import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { characterBlockedMessage, describeCharacterError, switchedOffKey, switchedOffMessage } from './character-errors';

function blockedError(reason: CharacterBlockedReason, characterId = 'char-1'): ConnectError {
  return new ConnectError('blocked', Code.FailedPrecondition, undefined, [
    { desc: CharacterBlockedSchema, value: { reason, characterId } },
  ]);
}

describe('characterBlockedMessage', () => {
  it('names every CharacterBlocked reason', () => {
    expect(characterBlockedMessage('sheet_locked')).toContain('travada');
    expect(characterBlockedMessage('character_dead')).toContain('morto');
    expect(characterBlockedMessage('living_character_exists')).toContain('já tem um personagem');
    expect(characterBlockedMessage('story_locked')).toContain('Permitir editar a história');
    expect(characterBlockedMessage('not_pending')).toContain('já foi aprovado');
    expect(characterBlockedMessage('awaiting_approval')).toContain('Aprove ou recuse');
  });

  it('falls back to a generic message when the reason is unknown', () => {
    expect(characterBlockedMessage(undefined)).toBe('Não foi possível concluir a ação agora.');
  });
});

describe('describeCharacterError', () => {
  it('reads the CharacterBlocked detail off the error itself: SHEET_LOCKED', () => {
    expect(describeCharacterError(blockedError(CharacterBlockedReason.SHEET_LOCKED))).toContain(
      'travada',
    );
  });

  it('reads the CharacterBlocked detail off the error itself: CHARACTER_DEAD', () => {
    expect(describeCharacterError(blockedError(CharacterBlockedReason.CHARACTER_DEAD))).toContain(
      'morto',
    );
  });

  it('reads the CharacterBlocked detail off the error itself: LIVING_CHARACTER_EXISTS', () => {
    expect(
      describeCharacterError(blockedError(CharacterBlockedReason.LIVING_CHARACTER_EXISTS)),
    ).toContain('já tem um personagem');
  });

  it('reads the CharacterBlocked detail off the error itself: STORY_LOCKED', () => {
    expect(describeCharacterError(blockedError(CharacterBlockedReason.STORY_LOCKED))).toContain(
      'Permitir editar a história',
    );
  });

  it('reads the CharacterBlocked detail off the error itself: NOT_PENDING and AWAITING_APPROVAL (MR-024)', () => {
    expect(describeCharacterError(blockedError(CharacterBlockedReason.NOT_PENDING))).toContain(
      'já foi aprovado',
    );
    expect(describeCharacterError(blockedError(CharacterBlockedReason.AWAITING_APPROVAL))).toContain(
      'Aprove ou recuse',
    );
  });

  it('falls back to a generic message for failed_precondition with no detail', () => {
    expect(describeCharacterError(new ConnectError('blocked', Code.FailedPrecondition))).toBe(
      'Não foi possível concluir a ação agora.',
    );
  });

  it('maps aborted to "a ficha mudou, recarregue"', () => {
    const err = new ConnectError('stale revision', Code.Aborted);
    expect(describeCharacterError(err)).toContain('A ficha mudou');
  });

  it('maps not_found, permission_denied and invalid_argument', () => {
    expect(describeCharacterError(new ConnectError('x', Code.NotFound))).toBe(
      'Personagem não encontrado.',
    );
    expect(describeCharacterError(new ConnectError('x', Code.PermissionDenied))).toBe(
      'Você não tem permissão para fazer isso.',
    );
    expect(describeCharacterError(new ConnectError('x', Code.InvalidArgument))).toBe(
      'Confira os campos da ficha.',
    );
  });

  it('falls back to the generic unavailable message for anything else', () => {
    expect(describeCharacterError(new Error('network down'))).toContain(
      'Não foi possível falar com o servidor',
    );
  });
});

describe('an option the master switched off (RN-23, SWITCHED_OFF_CONTENT)', () => {
  const off = (key: string) =>
    new ConnectError('blocked', Code.FailedPrecondition, undefined, [
      { desc: CharacterBlockedSchema, value: { reason: CharacterBlockedReason.SWITCHED_OFF_CONTENT, contentKey: key } },
    ]);

  it('reads the key of the refused choice off the typed detail', () => {
    expect(switchedOffKey(off('race:tiefling'))).toBe('race:tiefling');
    expect(switchedOffKey(off(''))).toBe('');
  });

  it('is null for any other error, so no other refusal is read as this one', () => {
    expect(switchedOffKey(blockedError(CharacterBlockedReason.SHEET_LOCKED))).toBeNull();
    expect(switchedOffKey(new ConnectError('x', Code.Aborted))).toBeNull();
    expect(switchedOffKey(new Error('offline'))).toBeNull();
  });

  it('says it in words: the option is not available any more, pick another', () => {
    expect(switchedOffMessage('Tiefling')).toBe('Tiefling não está mais disponível para os jogadores. Escolha outra.');
    expect(describeCharacterError(off('race:tiefling'))).toBe('Esta opção não está mais disponível para os jogadores. Escolha outra.');
  });
});
