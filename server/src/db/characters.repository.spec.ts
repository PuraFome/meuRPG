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

const liveDescribe = process.env.DATABASE_URL ? describe : describe.skip;

liveDescribe('CharactersRepository (live DB)', () => {
  let pg: PgService;
  let repo: CharactersRepository;
  const createdIds: string[] = [];

  beforeAll(() => {
    pg = new PgService();
    repo = new CharactersRepository(pg);
  });

  afterAll(async () => {
    for (const id of createdIds) {
      await repo.deleteById(id).catch(() => undefined);
    }
    await pg.getPool().end();
  });

  it('create -> findById round-trip keeps nested JSONB intact', async () => {
    const created = await repo.create({
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
    });
    createdIds.push(created.id);

    expect(created.id).toBeTruthy();
    expect(created.type).toBe('player');
    expect(created.createdAt).toBeInstanceOf(Date);
    expect(created.updatedAt).toBeInstanceOf(Date);

    const found = await repo.findById(created.id);
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
      'player',
    );
    createdIds.push(created.id);
    expect(created.type).toBe('player');
  });

  it('findById returns null for unknown id', async () => {
    const found = await repo.findById('00000000-0000-0000-0000-000000000000');
    expect(found).toBeNull();
  });

  it('updateById patches only provided fields and bumps updated_at', async () => {
    const created = await repo.create({
      type: 'npc',
      name: 'Before Patch',
      attributes: { hp: 5 },
    });
    createdIds.push(created.id);

    const patched = await repo.updateById(created.id, {
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

  it('updateById returns null for unknown id', async () => {
    const patched = await repo.updateById(
      '00000000-0000-0000-0000-000000000000',
      { name: 'Ghost' },
    );
    expect(patched).toBeNull();
  });

  it('deleteById returns true for existing row, false for unknown', async () => {
    const created = await repo.create({ type: 'minion', name: 'Ephemeral' });
    expect(await repo.deleteById(created.id)).toBe(true);
    expect(await repo.deleteById(created.id)).toBe(false);
  });

  it('join token create/find round-trip', async () => {
    const { token, expiresAt, type } = await repo.createJoinToken();
    expect(token).toBeTruthy();
    expect(type).toBe('player');
    expect(expiresAt).toBeInstanceOf(Date);

    const found = await repo.findJoinToken(token);
    expect(found).not.toBeNull();
    expect(found!.token).toBe(token);
    expect(found!.type).toBe('player');
    expect(found!.expiresAt).toBeInstanceOf(Date);
  });

  it('expired join token (ttlMs: -60000) returns null, no throw', async () => {
    const { token } = await repo.createJoinToken(-60_000);
    await expect(repo.findJoinToken(token)).resolves.toBeNull();
  });

  it('unknown join token returns null', async () => {
    await expect(
      repo.findJoinToken('definitely-not-a-real-token'),
    ).resolves.toBeNull();
  });
});
