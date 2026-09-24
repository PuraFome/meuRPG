import { Injectable } from '@nestjs/common';
import type { Pool } from 'pg';
import type { QueryResult } from 'pg';
import { PgService } from './pg.service';

export type UserRole = 'master' | 'visitor';

@Injectable()
export class UsersRepository {
  constructor(private readonly pg: PgService) {}

  private get pool(): Pool {
    return this.pg.getPool();
  }

  /**
   * Insert a user keyed by Google `sub`, or update the profile columns on
   * conflict (LWW on email/name). Returns the user `id` (uuid).
   * Collects only sub, email, name (NOT picture) per data minimization.
   */
  async upsertByGoogleSub(
    sub: string,
    email: string,
    name: string,
  ): Promise<string> {
    const result: QueryResult<{ id: string }> = await this.pool.query(
      `INSERT INTO users (google_sub, email, name)
       VALUES ($1, $2, $3)
       ON CONFLICT (google_sub) DO UPDATE
         SET email = $2, name = $3
       RETURNING id`,
      [sub, email, name],
    );
    return result.rows[0].id;
  }

  /**
   * Resolve a user `id` from a Google `sub`. Returns `null` when unknown.
   */
  async findBySub(sub: string): Promise<string | null> {
    const result: QueryResult<{ id: string }> = await this.pool.query(
      `SELECT id FROM users WHERE google_sub = $1`,
      [sub],
    );
    return result.rows[0]?.id ?? null;
  }

  /**
   * Record the consent timestamp for the user identified by Google `sub`.
   */
  async saveConsent(sub: string, consentAt: Date): Promise<void> {
    await this.pool.query(
      `UPDATE users SET consent_at = $2 WHERE google_sub = $1`,
      [sub, consentAt],
    );
  }

  /**
   * Set the access role of the user `id`. Used to downgrade an invited player
   * to `visitor` once they redeem a join link.
   */
  async setRole(userId: string, role: UserRole): Promise<void> {
    await this.pool.query(`UPDATE users SET role = $2 WHERE id = $1`, [
      userId,
      role,
    ]);
  }

  /**
   * Delete the user identified by Google `sub` (cascades `auth_sessions` via
   * `ON DELETE CASCADE`). Used for LGPD Art. 16 account deletion.
   */
  async deleteBySub(sub: string): Promise<void> {
    await this.pool.query('DELETE FROM users WHERE google_sub = $1', [sub]);
  }
}