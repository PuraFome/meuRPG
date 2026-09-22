import { createHash, randomBytes } from 'crypto';
import { Injectable } from '@nestjs/common';
import type { Pool } from 'pg';
import { PgService } from './pg.service';

export interface HandshakeSecrets {
  stateSecret: string;
  nonce: string;
  codeVerifier: string;
}

const HANDSHAKE_TTL_MS = 10 * 60 * 1000;
const SESSION_TTL_MS = 30 * 24 * 60 * 60 * 1000;

@Injectable()
export class AuthTokensRepository {
  constructor(private readonly pg: PgService) {}

  private get pool(): Pool {
    return this.pg.getPool();
  }

  /**
   * Persist a new OAuth handshake and return its `id`. The caller embeds
   * `${id}.${stateSecret}` in the OAuth `state` parameter.
   */
  async createHandshake(secrets: Omit<HandshakeSecrets, never>): Promise<string> {
    const id = randomBytes(16).toString('base64url');
    const expiresAt = new Date(Date.now() + HANDSHAKE_TTL_MS);
    await this.pool.query(
      `INSERT INTO oauth_handshakes (id, state_secret, nonce, code_verifier, expires_at)
       VALUES ($1, $2, $3, $4, $5)`,
      [id, secrets.stateSecret, secrets.nonce, secrets.codeVerifier, expiresAt],
    );
    return id;
  }

  /**
   * Single-use consume: the handshake row is deleted atomically on read, so a
   * callback can never be replayed. Returns `null` when unknown or expired.
   */
  async consumeHandshake(id: string): Promise<HandshakeSecrets | null> {
    const result = await this.pool.query<{
      state_secret: string;
      nonce: string;
      code_verifier: string;
    }>(
      `DELETE FROM oauth_handshakes
       WHERE id = $1 AND expires_at > now()
       RETURNING state_secret, nonce, code_verifier`,
      [id],
    );
    const row = result.rows[0];
    if (!row) {
      return null;
    }
    return {
      stateSecret: row.state_secret,
      nonce: row.nonce,
      codeVerifier: row.code_verifier,
    };
  }

  async createSession(userId: string): Promise<string> {
    const token = `ma_${randomBytes(32).toString('base64url')}`;
    const tokenHash = this.hashToken(token);
    const expiresAt = new Date(Date.now() + SESSION_TTL_MS);
    await this.pool.query(
      `INSERT INTO auth_sessions (token_hash, user_id, expires_at)
       VALUES ($1, $2, $3)`,
      [tokenHash, userId, expiresAt],
    );
    return token;
  }

  /**
   * Resolve the user behind a bearer token. Returns `null` when the token is
   * unknown or expired.
   */
  async findUserByToken(token: string): Promise<{ sub: string; email: string; name: string } | null> {
    const result = await this.pool.query<{
      google_sub: string;
      email: string | null;
      name: string | null;
    }>(
      `SELECT u.google_sub, u.email, u.name
       FROM auth_sessions s
       JOIN users u ON u.id = s.user_id
       WHERE s.token_hash = $1 AND s.expires_at > now()`,
      [this.hashToken(token)],
    );
    const row = result.rows[0];
    if (!row) {
      return null;
    }
    return {
      sub: row.google_sub,
      email: row.email ?? '',
      name: row.name ?? '',
    };
  }

  async deleteSession(token: string): Promise<void> {
    await this.pool.query(`DELETE FROM auth_sessions WHERE token_hash = $1`, [
      this.hashToken(token),
    ]);
  }

  private hashToken(token: string): string {
    return createHash('sha256').update(token).digest('hex');
  }
}