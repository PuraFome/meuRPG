import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import { InviteState } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  inviteApprovalLabel,
  inviteDateLabel,
  inviteStateTag,
  inviteUsesLabel,
} from './invite-copy';

describe('invite copy', () => {
  it('counts uses with the right plural', () => {
    expect(inviteUsesLabel({ useCount: 1, maxUses: 3 })).toBe('1 de 3 usos');
    expect(inviteUsesLabel({ useCount: 0, maxUses: 1 })).toBe('0 de 1 uso');
  });

  it('dates an active invite by its expiry, an expired one by when it expired, and the rest by creation', () => {
    const expiresAt = timestampFromDate(new Date(2026, 9, 6, 20, 14));
    const createdAt = timestampFromDate(new Date(2026, 8, 28, 9, 5));
    const at = { expiresAt, createdAt };
    expect(inviteDateLabel({ ...at, state: InviteState.ACTIVE })).toBe(
      'Vence em 06/10/2026\u00a0às\u00a020:14',
    );
    expect(inviteDateLabel({ ...at, state: InviteState.EXPIRED })).toBe(
      'Venceu em 06/10/2026\u00a0às\u00a020:14',
    );
    expect(inviteDateLabel({ ...at, state: InviteState.USED_UP })).toBe(
      'Criado em 28/09/2026\u00a0às\u00a009:05',
    );
    // Never "Revogado em": the tag already says it (e2e: one "revogado" per row).
    expect(inviteDateLabel({ ...at, state: InviteState.REVOKED })).toBe(
      'Criado em 28/09/2026\u00a0às\u00a009:05',
    );
    expect(
      inviteDateLabel({ state: InviteState.ACTIVE, expiresAt: undefined, createdAt: undefined }),
    ).toBe('');
  });

  it('says whether whoever joins waits for approval', () => {
    expect(inviteApprovalLabel({ requiresApproval: true })).toBe('Exige aprovação do mestre');
    expect(inviteApprovalLabel({ requiresApproval: false })).toBe('Entra direto na campanha');
  });

  it("tags the state with MR-002's word, capitalised", () => {
    expect(inviteStateTag(InviteState.ACTIVE)).toBe('Ativo');
    expect(inviteStateTag(InviteState.USED_UP)).toBe('Usado');
    expect(inviteStateTag(InviteState.EXPIRED)).toBe('Expirado');
    expect(inviteStateTag(InviteState.REVOKED)).toBe('Revogado');
  });
});
