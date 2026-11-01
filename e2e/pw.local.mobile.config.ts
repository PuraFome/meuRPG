import base from './playwright.mobile.config';
import { defineConfig } from '@playwright/test';

// Local only (not committed): WebKit runs as installed, so this only re-exports the mobile config.
export default defineConfig({ ...base });
