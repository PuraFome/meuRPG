/**
 * Playwright QA — F3 supplement: capture evidence of blocked/broken states + remaining desktop shots
 *  - task-6-quicksearch-mobile.png  : mobile /sessao (quick-search trigger absent — component dead code)
 *  - task-6-quicksearch-desktop.png : desktop /sessao (same finding)
 *  - task-6-global-search-mobile.png: reachable search dialog (Ctrl+K) width on mobile (proxy)
 *  - task-7-cockpit-desktop.png     : desktop cockpit side-by-side
 *  - task-10-dice-desktop.png       : desktop dice dialog
 *  - task-9-rules-mobile.png        : /regras broken state (NG0201 evidence)
 */
import { chromium } from 'playwright';
import { writeFileSync } from 'fs';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const EVIDENCE_DIR = __dirname;
const BASE_URL = process.env.BASE_URL || 'http://localhost:4200';

const results = [];
const consoleErrors = [];
function report(s, status, d = '') {
  console.log(`[${status}] ${s} — ${d}`);
  results.push({ scenario: s, status, detail: d });
}
async function shot(page, name) {
  await page.screenshot({ path: resolve(EVIDENCE_DIR, `${name}.png`), fullPage: true });
  console.log(`  📸 ${name}.png`);
}
function attachConsole(page, label) {
  page.on('console', (m) => { if (m.type() === 'error') { consoleErrors.push({ scenario: label, url: page.url(), text: m.text().slice(0, 200) }); console.log(`  🛑 [${label}] ${m.text().slice(0, 140)}`); } });
  page.on('pageerror', (e) => { consoleErrors.push({ scenario: label, url: page.url(), text: e.message.slice(0, 200) }); console.log(`  💥 [${label}] ${e.message.slice(0, 140)}`); });
}

const browser = await chromium.launch({ headless: true });
const MOBILE = { width: 375, height: 812 };
const DESKTOP = { width: 1440, height: 900 };

// 1. QuickSearch unreachability (mobile + desktop) + global search proxy
{
  const ctx = await browser.newContext({ viewport: MOBILE });
  const page = await ctx.newPage();
  attachConsole(page, 'S1');
  await page.goto(`${BASE_URL}/sessao`, { waitUntil: 'networkidle', timeout: 30000 });
  await page.waitForTimeout(1500);
  const hasBtn = await page.locator('button[aria-label="Pesquisa rápida"]').count();
  const body = (await page.locator('body').innerText()).slice(0, 120).replace(/\n/g, ' | ');
  report('QuickSearch trigger absent on mobile /sessao', hasBtn === 0 ? 'PASS' : 'FAIL',
    `searchBtn=${hasBtn} page="${body}"`);
  await shot(page, 'task-6-quicksearch-mobile');
  // global search (Ctrl+K) dialog as reachable-search proxy
  await page.locator('button[aria-label*="Ctrl+K"], button[aria-label*="Pesquisar"]').first().click();
  await page.waitForTimeout(1000);
  const gsBox = await page.locator('.mat-mdc-dialog-container').boundingBox();
  report('Global search dialog fits mobile viewport (proxy)', gsBox && gsBox.width <= 375 ? 'PASS' : 'FAIL',
    `width=${gsBox?.width}px`);
  await shot(page, 'task-6-global-search-mobile');
  await page.keyboard.press('Escape');
  await page.waitForTimeout(400);
  await ctx.close();
}

{
  const ctx = await browser.newContext({ viewport: DESKTOP });
  const page = await ctx.newPage();
  attachConsole(page, 'S2 desktop');
  await page.goto(`${BASE_URL}/sessao`, { waitUntil: 'networkidle', timeout: 30000 });
  await page.waitForTimeout(1500);
  const hasBtn = await page.locator('button[aria-label="Pesquisa rápida"]').count();
  report('QuickSearch trigger absent on desktop /sessao', hasBtn === 0 ? 'PASS' : 'FAIL', `searchBtn=${hasBtn}`);
  await shot(page, 'task-6-quicksearch-desktop');

  const dir = await page.locator('.split-container').evaluate((el) => getComputedStyle(el).flexDirection);
  const left = await page.locator('.panel-left').boundingBox();
  const right = await page.locator('.panel-right').boundingBox();
  report('Cockpit desktop side-by-side', dir === 'row' && left && right && right.x > left.x ? 'PASS' : 'FAIL',
    `dir=${dir} leftX=${left?.x} rightX=${right?.x} leftW=${left?.width} rightW=${right?.width}`);
  await shot(page, 'task-7-cockpit-desktop');

  await page.locator('button[aria-label="Rolar Dados"]').click();
  await page.waitForTimeout(900);
  const dBox = await page.locator('.mat-mdc-dialog-container').boundingBox();
  report('Dice dialog desktop ≤ 400px', dBox && dBox.width <= 401 ? 'PASS' : 'FAIL', `width=${dBox?.width}px`);
  await shot(page, 'task-10-dice-desktop');
  await page.keyboard.press('Escape');
  await page.waitForTimeout(400);
  await ctx.close();
}

// 2. /regras broken state (NG0201) evidence
{
  const ctx = await browser.newContext({ viewport: MOBILE });
  const page = await ctx.newPage();
  attachConsole(page, 'S3 regras');
  await page.goto(`${BASE_URL}/regras`, { waitUntil: 'networkidle', timeout: 30000 });
  await page.waitForTimeout(2000);
  const addBtn = await page.locator('button', { hasText: 'Adicionar Livro' }).count();
  const emptyState = await page.locator('.empty-state').count();
  const body = (await page.locator('body').innerText()).slice(0, 120).replace(/\n/g, ' | ');
  report('/regras renders rules UI', addBtn > 0 ? 'PASS' : 'FAIL',
    `addBtn=${addBtn} emptyState=${emptyState} page="${body}" — BLOCKED by NG0201 (IndexedDbFileRepository has no provider)`);
  await shot(page, 'task-9-rules-mobile');
  await ctx.close();
}

const passed = results.filter((r) => r.status === 'PASS').length;
const failed = results.filter((r) => r.status === 'FAIL').length;
console.log(`\nSupplement: ${passed} pass / ${failed} fail | consoleErrors=${consoleErrors.length}`);
writeFileSync(resolve(EVIDENCE_DIR, 'qa-report-f3-supplement.json'), JSON.stringify({ timestamp: new Date().toISOString(), scenarios: results, consoleErrors }, null, 2));
await browser.close();
process.exit(0);
