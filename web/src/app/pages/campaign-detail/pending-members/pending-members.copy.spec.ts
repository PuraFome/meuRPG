import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import { PendingMember } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { pendingMemberRows, shortDate } from './pending-members.copy';

function pending(name: string, joined: Date, expires: Date): PendingMember {
  return {
    userId: 'u2',
    displayName: name,
    joinedAt: timestampFromDate(joined),
    expiresAt: timestampFromDate(expires),
  } as PendingMember;
}

describe('pending members copy', () => {
  const now = new Date(2026, 9, 2);

  it('writes day/month, and the year only when it is not the current one', () => {
    expect(shortDate(new Date(2026, 8, 28), now)).toBe('28/09');
    expect(shortDate(new Date(2025, 11, 5), now)).toBe('05/12/2025');
  });

  it('says when they joined and when they leave on their own', () => {
    const [row] = pendingMemberRows(
      [pending('Lia', new Date(2026, 8, 28, 12), new Date(2026, 9, 28, 12))],
      now,
    );
    expect(row.name).toBe('Lia');
    expect(row.line).toBe(
      'Entrou pelo convite em 28/09, ainda sem personagem. Sai da campanha em 28/10, se nada mudar.',
    );
    expect(row.expiresOn).toBe('28/10');
  });

  it('never shows an empty name', () => {
    const [row] = pendingMemberRows(
      [pending(' ', new Date(2026, 8, 28), new Date(2026, 9, 28))],
      now,
    );
    expect(row.name).toBe('Jogador sem nome');
  });
});
