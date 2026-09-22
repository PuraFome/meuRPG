import { Pool } from 'pg';
import { PgService } from './pg.service';

/**
 * Reads schema.sql, splits on semicolons, and executes each statement
 * via a pg.Pool. Idempotent (safe to re-run).
 */
export async function migrate(pgService: PgService): Promise<void> {
  const pool = pgService.getPool();
  const schemaPath = new URL('./schema.sql', import.meta.url).pathname;
  const fs = await import('fs');
  const schema = fs.readFileSync(schemaPath, 'utf-8');
  const statements = schema.split(';').filter((s) => s.trim());

  for (const statement of statements) {
    await pool.query(statement);
  }
}

/** Standalone entrypoint for `npm run migrate`. */
async function main(): Promise<void> {
  const databaseUrl = process.env.DATABASE_URL;
  if (!databaseUrl || databaseUrl.trim() === '') {
    throw new Error(
      '[migrate] DATABASE_URL is missing or empty. Set it in server/.env before running.',
    );
  }
  const pgService = new PgService();
  await migrate(pgService);
  console.log('[migrate] Database migration applied successfully.');
}

main().catch((err) => {
  console.error('[migrate] Migration failed:', err.message);
  process.exit(1);
});