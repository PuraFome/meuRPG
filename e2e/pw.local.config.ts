import base from './playwright.config';
import { defineConfig } from '@playwright/test';

// Local only (not committed): this VM's Chromium is at /opt/pw-browsers/chromium.
export default defineConfig({
  ...base,
  projects: (base.projects ?? []).map((p) => ({
    ...p,
    use: { ...p.use, channel: undefined, launchOptions: { executablePath: '/opt/pw-browsers/chromium', args: ['--no-sandbox'] } },
  })),
});
