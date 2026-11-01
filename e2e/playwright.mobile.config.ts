import { defineConfig, devices } from '@playwright/test';

import base from './playwright.config';

// The mobile smoke on WebKit (Safari) and an iPhone: one spec, run with `npx playwright test -c playwright.mobile.config.ts`
// (npm run test:mobile). Chrome runs the same spec in the default config.
// WebKit keeps no Secure cookie over http, and the app's cookies are all Secure, so these projects use https:
// e2e/tls-front.mjs fronts the stack, which was started with E2E_PUBLIC_ORIGIN (compose) or PUBLIC_ORIGIN (native.sh) set to this origin.
// The front's certificate is its own, so only these projects ignore certificate errors.
// The spec signs in itself in every context it opens: there is no setup project here.
// Retries, reporters and the rest are the base config's.
const baseURL = process.env.E2E_BASE_URL ?? 'https://localhost:8443';
const smoke = /mobile-smoke\.spec\.ts/;

export default defineConfig({
  ...base,
  use: { ...base.use, baseURL },
  projects: [
    { name: 'webkit', testMatch: smoke, use: { ...devices['Desktop Safari'], ignoreHTTPSErrors: true } },
    { name: 'iphone', testMatch: smoke, use: { ...devices['iPhone 14'], ignoreHTTPSErrors: true } },
  ],
});
