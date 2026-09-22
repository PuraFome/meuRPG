import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { Pool } from 'pg';

/**
 * Redact a DATABASE_URL so the password never reaches logs.
 * postgresql://user:pass@host:26257/db?sslmode=verify-full
 *   -> postgresql://***@host:26257/db
 */
function redactDatabaseUrl(url: string): string {
  try {
    const u = new URL(url);
    return `${u.protocol}//***@${u.host}${u.pathname}`;
  } catch {
    return 'postgresql://*** (unparseable)';
  }
}

@Injectable()
export class PgService implements OnModuleInit {
  private readonly logger = new Logger(PgService.name);
  private readonly databaseUrl: string;
  private readonly pool: Pool;

  constructor() {
    const databaseUrl = process.env.DATABASE_URL;
    if (!databaseUrl || databaseUrl.trim() === '') {
      throw new Error(
        '[PgService] DATABASE_URL is missing or empty. Set it in server/.env ' +
          '(copy from server/.env.example) before starting the backend. Aborting bootstrap.',
      );
    }
    this.databaseUrl = databaseUrl;
    this.pool = new Pool({
      connectionString: this.databaseUrl,
      application_name: 'meu-rpg-backend',
      max: 5,
      idleTimeoutMillis: 30000,
      connectionTimeoutMillis: 10000,
    });
  }

  /** Expose the underlying pg.Pool for downstream tasks (runTxn, repositories). */
  getPool(): Pool {
    return this.pool;
  }

  async onModuleInit(): Promise<void> {
    this.logger.log(
      `validating database connection to ${redactDatabaseUrl(this.databaseUrl)}`,
    );
    try {
      await this.pool.query('SELECT 1');
      this.logger.log('db connected');
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      this.logger.error(
        `[PgService] Database connection validation failed (SELECT 1): ${message}. Aborting bootstrap.`,
      );
      throw new Error(
        `[PgService] Database connection validation failed (SELECT 1): ${message}`,
      );
    }
  }
}