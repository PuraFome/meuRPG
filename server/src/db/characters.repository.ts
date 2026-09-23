import { randomBytes } from 'node:crypto';
import { Injectable } from '@nestjs/common';
import type { Pool } from 'pg';
import type { QueryResult } from 'pg';
import { PgService } from './pg.service';

export type CharacterType = 'npc' | 'player' | 'boss' | 'minion';

export interface CharacterRecord {
  id: string;
  type: CharacterType;
  name: string;
  description: string;
  imageUrl: string | null;
  history: string | null;
  masterNotes: string | null;
  attributes: Record<string, number>;
  skills: string[];
  inventory: string[];
  quotes: string[];
  sheet: unknown | null;
  minion: { hp: number; attack: number } | null;
  createdAt: Date;
  updatedAt: Date;
}

export interface CreateCharacterInput {
  id?: string;
  type: CharacterType;
  name: string;
  description?: string;
  imageUrl?: string | null;
  history?: string | null;
  masterNotes?: string | null;
  attributes?: Record<string, number>;
  skills?: string[];
  inventory?: string[];
  quotes?: string[];
  sheet?: unknown;
  minion?: { hp: number; attack: number } | null;
}

export type UpdateCharacterPatch = Partial<
  Omit<CreateCharacterInput, 'id'>
>;

export interface JoinToken {
  token: string;
  type: 'player';
  expiresAt: Date;
}

/** 30 days, matching the session TTL used by auth tokens. */
const JOIN_TOKEN_TTL_MS = 30 * 24 * 60 * 60 * 1000;

interface CharacterRow {
  id: string;
  type: CharacterType;
  name: string;
  description: string;
  image_url: string | null;
  history: string | null;
  master_notes: string | null;
  attributes: Record<string, number>;
  skills: string[];
  inventory: string[];
  quotes: string[];
  sheet: unknown | null;
  minion: { hp: number; attack: number } | null;
  created_at: Date;
  updated_at: Date;
}

function mapRow(row: CharacterRow): CharacterRecord {
  return {
    id: row.id,
    type: row.type,
    name: row.name,
    description: row.description,
    imageUrl: row.image_url,
    history: row.history,
    masterNotes: row.master_notes,
    attributes: row.attributes,
    skills: row.skills,
    inventory: row.inventory,
    quotes: row.quotes,
    sheet: row.sheet,
    minion: row.minion,
    createdAt: row.created_at,
    updatedAt: row.updated_at,
  };
}

/**
 * node-postgres serializes JS arrays as Postgres array literals (`{...}`),
 * NOT JSON — every JSONB field must be explicitly JSON.stringify'd on write.
 * `undefined`/`null` pass through as SQL NULL (never the string "undefined").
 */
function jsonEncode(value: unknown): string | null {
  if (value === undefined || value === null) {
    return null;
  }
  return JSON.stringify(value);
}

@Injectable()
export class CharactersRepository {
  constructor(private readonly pg: PgService) {}

  private get pool(): Pool {
    return this.pg.getPool();
  }

  /**
   * Insert a character. When `input.id` is absent the DB generates a uuid;
   * `forcedType` (when given) overrides `input.type` — used by the join-link
   * flow to pin redeemed characters to `player`. Returns the full row.
   */
  async create(
    input: CreateCharacterInput,
    forcedType?: CharacterType,
  ): Promise<CharacterRecord> {
    const result: QueryResult<CharacterRow> = await this.pool.query(
      `INSERT INTO characters (id, type, name, description, image_url, history, master_notes, attributes, skills, inventory, quotes, sheet, minion)
       VALUES (COALESCE($1::uuid, gen_random_uuid()), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
       RETURNING *`,
      [
        input.id ?? null,
        forcedType ?? input.type,
        input.name,
        input.description ?? '',
        input.imageUrl ?? null,
        input.history ?? null,
        input.masterNotes ?? null,
        jsonEncode(input.attributes ?? {}),
        jsonEncode(input.skills ?? []),
        jsonEncode(input.inventory ?? []),
        jsonEncode(input.quotes ?? []),
        jsonEncode(input.sheet),
        jsonEncode(input.minion),
      ],
    );
    return mapRow(result.rows[0]);
  }

  /** Fetch a character by id. Returns `null` when unknown. */
  async findById(id: string): Promise<CharacterRecord | null> {
    const result: QueryResult<CharacterRow> = await this.pool.query(
      `SELECT * FROM characters WHERE id = $1`,
      [id],
    );
    const row = result.rows[0];
    return row ? mapRow(row) : null;
  }

  /** List characters, newest first; optionally filtered by `type`. */
  async findAll(type?: CharacterType): Promise<CharacterRecord[]> {
    const result: QueryResult<CharacterRow> = type
      ? await this.pool.query(
          `SELECT * FROM characters WHERE type = $1 ORDER BY created_at DESC`,
          [type],
        )
      : await this.pool.query(
          `SELECT * FROM characters ORDER BY created_at DESC`,
        );
    return result.rows.map(mapRow);
  }

  /**
   * Patch only the provided (non-undefined) fields; `updated_at` is always
   * bumped. Explicit `null` in the patch writes SQL NULL. Returns the updated
   * row, or `null` when no character matches `id`.
   */
  async updateById(
    id: string,
    patch: UpdateCharacterPatch,
  ): Promise<CharacterRecord | null> {
    const sets: string[] = [];
    const values: unknown[] = [];
    let param = 1;
    const push = (column: string, value: unknown): void => {
      sets.push(`${column} = $${param++}`);
      values.push(value);
    };

    if (patch.type !== undefined) push('type', patch.type);
    if (patch.name !== undefined) push('name', patch.name);
    if (patch.description !== undefined) push('description', patch.description);
    if (patch.imageUrl !== undefined) push('image_url', patch.imageUrl);
    if (patch.history !== undefined) push('history', patch.history);
    if (patch.masterNotes !== undefined)
      push('master_notes', patch.masterNotes);
    if (patch.attributes !== undefined)
      push('attributes', jsonEncode(patch.attributes));
    if (patch.skills !== undefined) push('skills', jsonEncode(patch.skills));
    if (patch.inventory !== undefined)
      push('inventory', jsonEncode(patch.inventory));
    if (patch.quotes !== undefined) push('quotes', jsonEncode(patch.quotes));
    if (patch.sheet !== undefined) push('sheet', jsonEncode(patch.sheet));
    if (patch.minion !== undefined) push('minion', jsonEncode(patch.minion));

    sets.push('updated_at = now()');
    values.push(id);

    const result: QueryResult<CharacterRow> = await this.pool.query(
      `UPDATE characters SET ${sets.join(', ')} WHERE id = $${param} RETURNING *`,
      values,
    );
    const row = result.rows[0];
    return row ? mapRow(row) : null;
  }

  /** Delete a character. Returns `true` when a row was actually removed. */
  async deleteById(id: string): Promise<boolean> {
    const result = await this.pool.query(
      `DELETE FROM characters WHERE id = $1`,
      [id],
    );
    return (result.rowCount ?? 0) > 0;
  }

  /**
   * Mint a REUSABLE share-link token (valid until expiry; `character_id`
   * stays NULL by design — it is backfilled once a character is created
   * through the link). Default TTL: 30 days.
   */
  async createJoinToken(ttlMs: number = JOIN_TOKEN_TTL_MS): Promise<JoinToken> {
    const token = randomBytes(32).toString('base64url');
    const expiresAt = new Date(Date.now() + ttlMs);
    await this.pool.query(
      `INSERT INTO character_join_tokens (token, type, expires_at)
       VALUES ($1, 'player', $2)`,
      [token, expiresAt],
    );
    return { token, expiresAt, type: 'player' };
  }

  /**
   * Resolve a join token. Returns `null` when the token is unknown or
   * expired. Tokens are NOT consumed — they stay redeemable until expiry.
   */
  async findJoinToken(token: string): Promise<JoinToken | null> {
    const result: QueryResult<{
      token: string;
      type: 'player';
      expires_at: Date;
    }> = await this.pool.query(
      `SELECT token, type, expires_at
       FROM character_join_tokens
       WHERE token = $1 AND expires_at > now()`,
      [token],
    );
    const row = result.rows[0];
    if (!row) {
      return null;
    }
    return { token: row.token, type: row.type, expiresAt: row.expires_at };
  }
}
