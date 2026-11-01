// Waits until the stack answers /readyz, so `make e2e` and CI can start the
// tests right after `docker compose up -d`. The api service only starts once
// the database, the migrations and devidp are ready (compose.yaml).
import https from 'node:https';

import type { FullConfig } from '@playwright/test';

const timeoutMs = 120_000;

/** The status of a GET. An https stack with a certificate of its own (the mobile smoke's front) is accepted as it is. */
async function status(url: URL, ignoreHTTPSErrors: boolean): Promise<number> {
  if (url.protocol !== 'https:' || !ignoreHTTPSErrors) {
    return (await fetch(url)).status;
  }
  return new Promise((resolve, reject) => {
    https.get(url, { rejectUnauthorized: false }, (res) => {
      res.resume();
      resolve(res.statusCode ?? 0);
    }).on('error', reject);
  });
}

export default async function globalSetup(config: FullConfig): Promise<void> {
  const use = config.projects[0]?.use;
  const baseURL = use?.baseURL ?? 'http://localhost:8080';
  const deadline = Date.now() + timeoutMs;
  let last = '';
  while (Date.now() < deadline) {
    try {
      const code = await status(new URL('/readyz', baseURL), use?.ignoreHTTPSErrors ?? false);
      if (code >= 200 && code < 300) {
        return;
      }
      last = `HTTP ${code}`;
    } catch (err) {
      last = String(err);
    }
    await new Promise((resolve) => setTimeout(resolve, 1_000));
  }
  throw new Error(`the stack at ${baseURL} was not ready after ${timeoutMs / 1000}s (${last}); is it up? Try \`make up\`.`);
}
