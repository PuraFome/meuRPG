// Waits until the stack answers /readyz, so `make e2e` and CI can start the
// tests right after `docker compose up -d`. The api service only starts once
// the database, the migrations and devidp are ready (compose.yaml).
import { readFileSync } from 'node:fs';
import https from 'node:https';

import type { FullConfig } from '@playwright/test';

const timeoutMs = 120_000;

/**
 * The status of a GET. An https stack with a certificate of its own (the mobile smoke's front) is checked against the
 * PEM in E2E_TLS_CA; certificate checks stay on.
 */
export async function status(url: URL): Promise<number> {
  if (url.protocol !== 'https:') {
    return (await fetch(url)).status;
  }
  const caPath = process.env.E2E_TLS_CA;
  if (!caPath) {
    throw new FatalSetupError(`${url} is https: set E2E_TLS_CA to the PEM file of its certificate`);
  }
  const ca = readFileSync(caPath);
  return new Promise((resolve, reject) => {
    const request = https.get(url, { ca, timeout: 5_000 }, (res) => {
      res.resume();
      resolve(res.statusCode ?? 0);
    });
    request.on('timeout', () => request.destroy(new Error(`no answer from ${url}`)));
    request.on('error', reject);
  });
}

/** A mistake in the setup itself, which waiting does not fix. */
class FatalSetupError extends Error {}

export default async function globalSetup(config: FullConfig): Promise<void> {
  const use = config.projects[0]?.use;
  const baseURL = use?.baseURL ?? 'http://localhost:8080';
  const deadline = Date.now() + timeoutMs;
  let last = '';
  while (Date.now() < deadline) {
    try {
      const code = await status(new URL('/readyz', baseURL));
      if (code >= 200 && code < 300) {
        return;
      }
      last = `HTTP ${code}`;
    } catch (err) {
      if (err instanceof FatalSetupError) {
        throw err;
      }
      last = String(err);
    }
    await new Promise((resolve) => setTimeout(resolve, 1_000));
  }
  throw new Error(`the stack at ${baseURL} was not ready after ${timeoutMs / 1000}s (${last}); is it up? Try \`make up\`.`);
}
