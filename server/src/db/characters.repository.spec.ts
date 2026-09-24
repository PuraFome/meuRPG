import * as fs from 'node:fs';
import * as path from 'node:path';

/**
 * Self-contained env bootstrap: if DATABASE_URL is not already exported,
 * load it from server/.env so the spec can run via `npx vitest run` without
 * extra shell wiring. Never overrides an already-set variable.
 */
if (!process.env.DATABASE_URL) {
  const envPath = path.resolve(__dirname, '../../.env');
  if (fs.existsSync(envPath)) {
    for (const line of fs.readFileSync(envPath, 'utf-8').split('\n')) {
      const match = line.match(/^\s*([A-Z0-9_]+)\s*=\s*(.*)\s*$/);
      if (match && !process.env[match[1]]) {
        process.env[match[1]] = match[2].replace(/^["']|["']$/g, '');
      }
    }
  }
}

import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { PgService } from './pg.service';
import { CharactersRepository } from './characters.repository';
import { UsersRepository } from './users.repository';

const liveDescribe = process.env.DATABASE_URL ? describe : describe.skip;

liveDescribe('CharactersRepository (live DB)', () => {
  let pg: PgService;
  let repo: CharactersRepository;
  let users: UsersRepository;
  let ownerId: string;
  let otherOwnerId: string;
  const ownerSub = `spec-owner-${Date.now()}`;
  const otherOwnerSub = `spec-other-${Date.now()}`;

  beforeAll(async () => {
    pg = new PgService();
    repo = new CharactersRepository(pg);
    users = new UsersRepository(pg);
    ownerId = await users.upsertByGoogleSub(
      ownerSub,
      'spec-owner@example.com',
      'Spec Owner',
    );
    otherOwnerId = await users.upsertByGoogleSub(
      otherOwnerSub,
      'spec-other@example.com',
      'Spec Other',
    );
  });

  afterAll(async () => {
    // Deleting the throwaway users cascades their characters and join tokens.
    await users.deleteBySub(ownerSub).catch(() => undefined);
    await users.deleteBySub(otherOwnerSub).catch(() => undefined);
    await pg.getPool().end();
  });

  it('create -> findByIdForUser round-trip keeps nested JSONB intact', async () => {
    const created = await repo.create(
      {
        type: 'player',
        name: 'Spec Hero',
        description: 'A hero from the spec suite',
        imageUrl: 'https://example.com/hero.png',
        history: 'Raised by wolves',
        masterNotes: 'Secretly the villain',
        attributes: { str: 10, dex: 14, cha: 8 },
        skills: ['stealth', 'perception'],
        inventory: ['rope', 'torch'],
        quotes: ['I have a plan'],
        sheet: { race: 'elf', class: 'ranger', level: 3 },
        minion: null,
      },
      ownerId,
    );

    expect(created.id).toBeTruthy();
    expect(created.userId).toBe(ownerId);
    expect(created.type).toBe('player');
    expect(created.createdAt).toBeInstanceOf(Date);
    expect(created.updatedAt).toBeInstanceOf(Date);

    const found = await repo.findByIdForUser(created.id, ownerId);
    expect(found).not.toBeNull();
    expect(found!.name).toBe('Spec Hero');
    expect(found!.imageUrl).toBe('https://example.com/hero.png');
    expect(found!.masterNotes).toBe('Secretly the villain');
    // CockroachDB jsonb returns canonical sorted keys — toEqual, never
    // order-sensitive JSON.stringify comparison.
    expect(found!.attributes).toEqual({ str: 10, dex: 14, cha: 8 });
    expect(found!.skills).toEqual(['stealth', 'perception']);
    expect(found!.inventory).toEqual(['rope', 'torch']);
    expect(found!.quotes).toEqual(['I have a plan']);
    expect(found!.sheet).toEqual({ race: 'elf', class: 'ranger', level: 3 });
    expect(found!.minion).toBeNull();
  });

  it('create with forcedType overrides input.type', async () => {
    const created = await repo.create(
      { type: 'npc', name: 'Forced Player' },
      ownerId,
      'player',
    );
    expect(created.type).toBe('player');
  });

  it('findByIdForUser does not expose another owner’s character', async () => {
    const created = await repo.create(
      { type: 'npc', name: 'Owner A Secret' },
      ownerId,
    );

    const found = await repo.findByIdForUser(created.id, otherOwnerId);
    expect(found).toBeNull();
  });

  it('findByIdForUser returns null for unknown id', async () => {
    const found = await repo.findByIdForUser(
      '00000000-0000-0000-0000-000000000000',
      ownerId,
    );
    expect(found).toBeNull();
  });

  it('findAllForUser only lists the owner’s characters', async () => {
    const mine = await repo.create({ type: 'npc', name: 'Mine' }, ownerId);

    const mineList = await repo.findAllForUser(ownerId);
    const otherList = await repo.findAllForUser(otherOwnerId);

    expect(mineList.map((c) => c.id)).toContain(mine.id);
    expect(otherList.map((c) => c.id)).not.toContain(mine.id);
    expect(mineList.every((c) => c.userId === ownerId)).toBe(true);
  });

  it('updateByIdForUser patches only provided fields and bumps updated_at', async () => {
    const created = await repo.create(
      { type: 'npc', name: 'Before Patch', attributes: { hp: 5 } },
      ownerId,
    );

    const patched = await repo.updateByIdForUser(created.id, ownerId, {
      name: 'After Patch',
      inventory: ['key'],
    });
    expect(patched).not.toBeNull();
    expect(patched!.name).toBe('After Patch');
    expect(patched!.inventory).toEqual(['key']);
    // Untouched fields survive.
    expect(patched!.attributes).toEqual({ hp: 5 });
    expect(patched!.type).toBe('npc');
    expect(patched!.updatedAt.getTime()).toBeGreaterThanOrEqual(
      created.updatedAt.getTime(),
    );
  });

  it('updateByIdForUser cannot touch another owner’s row', async () => {
    const created = await repo.create(
      { type: 'npc', name: 'Not Yours' },
      ownerId,
    );

    const patched = await repo.updateByIdForUser(created.id, otherOwnerId, {
      name: 'Hijacked',
    });
    expect(patched).toBeNull();
  });

  it('updateByIdForUser returns null for unknown id', async () => {
    const patched = await repo.updateByIdForUser(
      '00000000-0000-0000-0000-000000000000',
      ownerId,
      { name: 'Ghost' },
    );
    expect(patched).toBeNull();
  });

  it('deleteByIdForUser returns true for owned row, false for unknown/other owner', async () => {
    const created = await repo.create({ type: 'minion', name: 'Ephemeral' }, ownerId);
    expect(await repo.deleteByIdForUser(created.id, otherOwnerId)).toBe(false);
    expect(await repo.deleteByIdForUser(created.id, ownerId)).toBe(true);
    expect(await repo.deleteByIdForUser(created.id, ownerId)).toBe(false);
  });

  it('join token create/find round-trip records the creator', async () => {
    const { token, expiresAt, type, createdBy } =
      await repo.createJoinToken(ownerId);
    expect(token).toBeTruthy();
    expect(type).toBe('player');
    expect(createdBy).toBe(ownerId);
    expect(expiresAt).toBeInstanceOf(Date);

    const found = await repo.findJoinToken(token);
    expect(found).not.toBeNull();
    expect(found!.token).toBe(token);
    expect(found!.type).toBe('player');
    expect(found!.createdBy).toBe(ownerId);
    expect(found!.expiresAt).toBeInstanceOf(Date);
  });

  it('a character redeemed from a join token is owned by the redeemer and visible to the inviter', async () => {
    const { token } = await repo.createJoinToken(ownerId);
    const found = await repo.findJoinToken(token);
    expect(found).not.toBeNull();

    const redeemed = await repo.create(
      { type: 'npc', name: 'Redeemed' },
      otherOwnerId,
      'player',
      found!.createdBy,
    );
    expect(redeemed.userId).toBe(otherOwnerId);
    expect(redeemed.masterUserId).toBe(ownerId);
    expect(redeemed.type).toBe('player');

    const guestList = await repo.findAllForUser(otherOwnerId);
    const masterList = await repo.findAllForUser(ownerId);
    expect(guestList.map((c) => c.id)).toContain(redeemed.id);
    expect(masterList.map((c) => c.id)).toContain(redeemed.id);
  });

  it('expired join token (ttlMs: -60000) returns null, no throw', async () => {
    const { token } = await repo.createJoinToken(ownerId, -60_000);
    await expect(repo.findJoinToken(token)).resolves.toBeNull();
  });

  it('unknown join token returns null', async () => {
    await expect(
      repo.findJoinToken('definitely-not-a-real-token'),
    ).resolves.toBeNull();
  });
});
