// Waits until the stack answers /readyz, so `make e2e` and CI can start the
// tests right after `docker compose up -d`. The api service only starts once
// the database, the migrations and devidp are ready (compose.yaml).
import type { FullConfig } from '@playwright/test';

const timeoutMs = 120_000;

export default async function globalSetup(config: FullConfig): Promise<void> {
  const baseURL = config.projects[0]?.use.baseURL ?? 'http://localhost:8080';
  const deadline = Date.now() + timeoutMs;
  let last = '';
  while (Date.now() < deadline) {
    try {
      const res = await fetch(new URL('/readyz', baseURL));
      if (res.ok) {
        return;
      }
      last = `HTTP ${res.status}`;
    } catch (err) {
      last = String(err);
    }
    await new Promise((resolve) => setTimeout(resolve, 1_000));
  }
  throw new Error(`the stack at ${baseURL} was not ready after ${timeoutMs / 1000}s (${last}); is it up? Try \`make up\`.`);
}
