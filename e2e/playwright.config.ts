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
  reporter: [['list'], ['html', { open: 'never' }], ...(process.env.CI ? [['github'] as const] : [])],
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chrome',
      use: { ...devices['Desktop Chrome'], channel: channel || undefined },
    },
  ],
});
