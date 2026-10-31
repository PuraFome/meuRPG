import os from 'node:os';
import path from 'node:path';

import { defineConfig } from '@playwright/test';

// Tests that fail on purpose, run by tests/diagnostics.spec.ts to see what a failed test attaches. No stack, no
// global setup: the pages are made here. The shared config's `use` is repeated where it matters.
export default defineConfig({
  testDir: '.',
  // Not the suite's test-results: a run empties its output folder, and this one runs inside the suite.
  outputDir: path.join(os.tmpdir(), 'meurpg-diagnostics-fixture'),
  testMatch: /fails\.spec\.ts/,
  retries: 0,
  workers: 1,
  timeout: 5_000,
  use: { actionTimeout: 2_000, trace: 'retain-on-failure', screenshot: 'only-on-failure' },
});
