import { Injectable } from '@nestjs/common';
import type { Pool, QueryResult } from 'pg';
import { PgService } from './pg.service';

export type MapKind = 'world' | 'city' | 'dungeon' | 'local';

export interface MapRecord {
  id: string;
  userId: string;
  name: string;
  description: string | null;
  kind: MapKind;
  backgroundImage: string | null;
  width: number;
  height: number;
  grid: unknown;
  fogOfWar: unknown;
  layers: unknown;
  markers: unknown;
  submaps: unknown;
  dungeon: unknown;
  createdAt: Date;
  updatedAt: Date;
}

export interface CreateMapInput {
  id?: string;
  name: string;
  description?: string | null;
  kind?: MapKind;
  backgroundImage?: string | null;
  width?: number;
  height?: number;
  grid?: unknown;
  fogOfWar?: unknown;
  layers?: unknown;
  markers?: unknown;
  submaps?: unknown;
  dungeon?: unknown;
}

export type UpdateMapPatch = Partial<Omit<CreateMapInput, 'id'>>;

interface MapRow {
  id: string;
  user_id: string;
  name: string;
  description: string | null;
  kind: MapKind;
  background_image: string | null;
  width: number;
  height: number;
  grid: unknown;
  fog_of_war: unknown;
  layers: unknown;
  markers: unknown;
  submaps: unknown;
  dungeon: unknown;
  created_at: Date;
  updated_at: Date;
}

function mapRow(row: MapRow): MapRecord {
  return {
    id: row.id,
    userId: row.user_id,
    name: row.name,
    description: row.description,
    kind: row.kind,
    backgroundImage: row.background_image,
    width: row.width,
    height: row.height,
    grid: row.grid,
    fogOfWar: row.fog_of_war,
    layers: row.layers,
    markers: row.markers,
    submaps: row.submaps,
    dungeon: row.dungeon,
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
export class MapsRepository {
  constructor(private readonly pg: PgService) {}

  private get pool(): Pool {
    return this.pg.getPool();
  }

  /** Insert a map owned by `ownerUserId`; the DB generates a uuid when `input.id` is absent. */
  async create(input: CreateMapInput, ownerUserId: string): Promise<MapRecord> {
    const result: QueryResult<MapRow> = await this.pool.query(
      `INSERT INTO maps (id, user_id, name, description, kind, background_image, width, height, grid, fog_of_war, layers, markers, submaps, dungeon)
       VALUES (COALESCE($1::uuid, gen_random_uuid()), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
       RETURNING *`,
      [
        input.id ?? null,
        ownerUserId,
        input.name,
        input.description ?? null,
        input.kind ?? 'world',
        input.backgroundImage ?? null,
        input.width ?? 1024,
        input.height ?? 768,
        jsonEncode(input.grid ?? {}),
        jsonEncode(input.fogOfWar ?? {}),
        jsonEncode(input.layers ?? []),
        jsonEncode(input.markers ?? []),
        jsonEncode(input.submaps ?? []),
        jsonEncode(input.dungeon ?? {}),
      ],
    );
    return mapRow(result.rows[0]);
  }

  /** Fetch a map by id owned by `ownerUserId`. Returns `null` when unknown or inaccessible. */
  async findByIdForUser(
    id: string,
    ownerUserId: string,
  ): Promise<MapRecord | null> {
    const result: QueryResult<MapRow> = await this.pool.query(
      `SELECT * FROM maps WHERE id = $1 AND user_id = $2`,
      [id, ownerUserId],
    );
    const row = result.rows[0];
    return row ? mapRow(row) : null;
  }

  /** List maps owned by a user, newest first. */
  async findAllForUser(ownerUserId: string): Promise<MapRecord[]> {
    const result: QueryResult<MapRow> = await this.pool.query(
      `SELECT * FROM maps WHERE user_id = $1 ORDER BY created_at DESC`,
      [ownerUserId],
    );
    return result.rows.map(mapRow);
  }

  /**
   * Patch only the provided (non-undefined) fields; `updated_at` is always
   * bumped. Returns the updated row, or `null` when no map matches the owner.
   */
  async updateByIdForUser(
    id: string,
    ownerUserId: string,
    patch: UpdateMapPatch,
  ): Promise<MapRecord | null> {
    const sets: string[] = [];
    const values: unknown[] = [];
    let param = 1;
    const push = (column: string, value: unknown): void => {
      sets.push(`${column} = $${param++}`);
      values.push(value);
    };

    if (patch.name !== undefined) push('name', patch.name);
    if (patch.description !== undefined) push('description', patch.description);
    if (patch.kind !== undefined) push('kind', patch.kind);
    if (patch.backgroundImage !== undefined)
      push('background_image', patch.backgroundImage);
    if (patch.width !== undefined) push('width', patch.width);
    if (patch.height !== undefined) push('height', patch.height);
    if (patch.grid !== undefined) push('grid', jsonEncode(patch.grid));
    if (patch.fogOfWar !== undefined)
      push('fog_of_war', jsonEncode(patch.fogOfWar));
    if (patch.layers !== undefined) push('layers', jsonEncode(patch.layers));
    if (patch.markers !== undefined) push('markers', jsonEncode(patch.markers));
    if (patch.submaps !== undefined) push('submaps', jsonEncode(patch.submaps));
    if (patch.dungeon !== undefined) push('dungeon', jsonEncode(patch.dungeon));

    sets.push('updated_at = now()');
    values.push(id, ownerUserId);

    const result: QueryResult<MapRow> = await this.pool.query(
      `UPDATE maps SET ${sets.join(', ')}
       WHERE id = $${param++} AND user_id = $${param} RETURNING *`,
      values,
    );
    const row = result.rows[0];
    return row ? mapRow(row) : null;
  }

  /** Delete a map owned by the user. Returns `true` when a row was actually removed. */
  async deleteByIdForUser(id: string, ownerUserId: string): Promise<boolean> {
    const result = await this.pool.query(
      `DELETE FROM maps WHERE id = $1 AND user_id = $2`,
      [id, ownerUserId],
    );
    return (result.rowCount ?? 0) > 0;
  }
}
