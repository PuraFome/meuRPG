import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { claimFailure, reserveActionFailure } from './claim-errors';

function blocked(reason: CharacterBlockedReason, value: object = {}): ConnectError {
  return new ConnectError('blocked', Code.FailedPrecondition, undefined, [
    { desc: CharacterBlockedSchema, value: { reason, ...value } },
  ]);
}

describe('claimFailure (MR-049)', () => {
  it('reads every refusal of a link that does not work as the one same page', () => {
    // The server answers a link that is invalid, expired, used, revoked or lost in a race with the same not_found.
    expect(claimFailure(new ConnectError('this claim link cannot be used', Code.NotFound))).toEqual(
      {
        kind: 'unusable',
      },
    );
  });

  it('reads RN-03 by its typed detail and keeps what the page names', () => {
    expect(
      claimFailure(
        blocked(CharacterBlockedReason.LIVING_CHARACTER_EXISTS, {
          campaignId: 'camp-1',
          characterId: 'char-9',
          characterName: 'Ícaro',
        }),
      ),
    ).toEqual({
      kind: 'living',
      campaignId: 'camp-1',
      characterId: 'char-9',
      characterName: 'Ícaro',
    });
  });

  it('reads the master opening their own link', () => {
    expect(claimFailure(blocked(CharacterBlockedReason.CLAIM_OWN_LINK))).toEqual({
      kind: 'own-link',
    });
  });

  it('an unknown precondition is no reason to say more than "this link cannot be used"', () => {
    expect(claimFailure(new ConnectError('x', Code.FailedPrecondition))).toEqual({
      kind: 'unusable',
    });
  });

  it('a lost session sends the person to sign in again', () => {
    expect(claimFailure(new ConnectError('x', Code.Unauthenticated))).toEqual({
      kind: 'signed-out',
    });
  });

  it('a server failure or a plain network error is worth another try, with words of its own', () => {
    for (const err of [
      new ConnectError('db down', Code.Unavailable),
      new TypeError('Failed to fetch'),
    ]) {
      const f = claimFailure(err);
      expect(f.kind).toBe('transient');
      expect(f.kind === 'transient' && f.message).toContain('Tente de novo');
      // Never the server's own words.
      expect(f.kind === 'transient' && f.message).not.toContain('db down');
    }
  });

  it('too many tries says to wait, with the seconds', () => {
    const err = new ConnectError(
      'slow down',
      Code.ResourceExhausted,
      new Headers({ 'Retry-After': '3' }),
    );
    const f = claimFailure(err);
    expect(f.kind === 'transient' && f.message).toBe(
      'Muitas ações em pouco tempo. Espere 3 segundos e tente de novo.',
    );
  });
});

describe('reserveActionFailure (MR-049)', () => {
  it('names who used the link when a revoke lost to a claim', () => {
    const f = reserveActionFailure(
      blocked(CharacterBlockedReason.CLAIM_LINK_USED, { playerDisplayName: 'Lia' }),
    );
    expect(f.usedBy).toEqual({ name: 'Lia' });
  });

  it('says in words why an action on the row was refused', () => {
    expect(reserveActionFailure(blocked(CharacterBlockedReason.NOT_RESERVED)).message).toContain(
      'já tem dono',
    );
    expect(reserveActionFailure(blocked(CharacterBlockedReason.NOT_CLAIMED)).message).toContain(
      'não veio de um link',
    );
    expect(
      reserveActionFailure(blocked(CharacterBlockedReason.CHARACTER_IN_COMBAT)).message,
    ).toContain('Encerre o combate');
    expect(reserveActionFailure(new ConnectError('x', Code.PermissionDenied)).message).toBe(
      'Você não tem permissão para fazer isso.',
    );
  });
});
