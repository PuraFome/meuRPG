import { defineConfig, devices } from '@playwright/test';

// The stack under test: `make up` (deploy/local/compose.yaml) serves the app
// and the API on this origin, and signs in through devidp.
const baseURL = process.env.E2E_BASE_URL ?? 'http://localhost:8080';

// Locally, the tests drive the Google Chrome installed on the machine, so
// nothing is downloaded. In CI, they drive the Chromium that
// `npx playwright install chromium` downloads, whose version is pinned by
// package-lock.json. E2E_BROWSER_CHANNEL overrides both (an empty value
// means Playwright's own Chromium).
const channel = process.env.E2E_BROWSER_CHANNEL ?? (process.env.CI ? '' : 'chrome');

export default defineConfig({
  testDir: './tests',
  globalSetup: './global-setup.ts',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  // No retries: a flaky acceptance test is a bug to fix, not to hide.
  retries: 0,
  // Assertions wait up to 10 s instead of Playwright's default 5 s. Right
  // after `make e2e` rebuilds the stack and runs new migrations, the first
  // queries take 1 to 2 s each (measured in the api log on 29/09/2026), and
  // the parallel workers all hit that cold start at once. Warm, every
  // request answers in milliseconds, so this only absorbs the start.
  expect: { timeout: 10_000 },
  reporter: [['list'], ['html', { open: 'never' }], ...(process.env.CI ? [['github'] as const] : [])],
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      // Signs in once per test user and saves the state every other test
      // reuses (see tests/auth.setup.ts and support.ts's `authStatePath` /
      // `newSignedInContext`) — keeps the suite comfortably under
      // `/auth/login`'s rate limit instead of signing in per test.
      name: 'setup',
      testMatch: /auth\.setup\.ts/,
      use: { ...devices['Desktop Chrome'], channel: channel || undefined },
    },
    {
      name: 'chrome',
      testIgnore: /auth\.setup\.ts/,
      use: { ...devices['Desktop Chrome'], channel: channel || undefined },
      dependencies: ['setup'],
    },
  ],
});
