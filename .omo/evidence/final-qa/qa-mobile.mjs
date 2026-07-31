/**
 * Playwright QA — F3 Real Manual QA — per-task scenarios (responsividade-correcao)
 *
 * Covers:
 *  - Task 1: Persistence (seed + reload; corrupt data survival)
 *  - Task 2: Home CTAs ("Começar Jornada" → /personagens/novo; "Explorar" → /personagens)
 *  - Task 3: Empty state action ("Criar Personagem" → /personagens/novo)
 *  - Task 4: Dialogs (dice roller open/close/reopen single overlay; quick search)
 *  - Task 6: QuickSearch width (mobile 375px / desktop 1440px)
 *  - Task 7: Cockpit split stacking (mobile column / desktop row)
 *  - Task 8: CharacterSheet dynamic rows wrap (mobile)
 *  - Task 9: RulesReader sidebar fallback (mobile)
 *  - Task 10: Dialogs width ≤ viewport (mobile) / desktop sizes
 *  - Regression re-check: dice roll (baseline playwright-qa.mjs FAIL on disabled Rolar btn)
 *  - Edge: /mapa console error (NG0951)
 */
import { chromium } from 'playwright';
import { writeFileSync } from 'fs';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const EVIDENCE_DIR = __dirname;
const BASE_URL = process.env.BASE_URL || 'http://localhost:4200';

const results = [];
const allConsoleErrors = [];

function report(scenario, status, detail = '') {
  const line = `[${status}] ${scenario}${detail ? ' — ' + detail : ''}`;
  console.log(line);
  results.push({ scenario, status, detail });
}

function attachConsole(page, label) {
  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      const entry = { url: page.url(), text: msg.text(), scenario: label };
      allConsoleErrors.push(entry);
      console.log(`  🛑 Console ERROR @ ${page.url()} [${label}]: ${msg.text().slice(0, 160)}`);
    }
  });
  page.on('pageerror', (err) => {
    const entry = { url: page.url(), text: err.message, scenario: label };
    allConsoleErrors.push(entry);
    console.log(`  💥 Page ERROR @ ${page.url()} [${label}]: ${err.message.slice(0, 160)}`);
  });
}

async function shot(page, name) {
  const path = resolve(EVIDENCE_DIR, `${name}.png`);
  await page.screenshot({ path, fullPage: true });
  console.log(`  📸 Screenshot saved: ${name}.png`);
  return path;
}

async function goto(page, path) {
  await page.goto(`${BASE_URL}${path}`, { waitUntil: 'networkidle', timeout: 30000 });
  await page.waitForTimeout(1500); // hydrate
}

async function dialogCount(page) {
  return page.locator('.mat-mdc-dialog-container').count();
}

async function overflow(page) {
  return page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
  }));
}

/** Minimal but VALID single-page PDF (correct xref offsets). */
function makePdf(text) {
  const esc = String(text).replace(/\\/g, '\\\\').replace(/\(/g, '\\(').replace(/\)/g, '\\)');
  const stream = `BT /F1 24 Tf 72 720 Td (${esc}) Tj ET`;
  const objects = {
    1: '<< /Type /Catalog /Pages 2 0 R >>',
    2: '<< /Type /Pages /Kids [3 0 R] /Count 1 >>',
    3: '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>',
    4: `<< /Length ${Buffer.byteLength(stream, 'ascii')} >>\nstream\n${stream}\nendstream`,
    5: '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>',
  };
  let pdf = '%PDF-1.4\n';
  const offsets = {};
  for (let i = 1; i <= 5; i++) {
    offsets[i] = Buffer.byteLength(pdf, 'ascii');
    pdf += `${i} 0 obj\n${objects[i]}\nendobj\n`;
  }
  const xrefStart = Buffer.byteLength(pdf, 'ascii');
  pdf += `xref\n0 6\n0000000000 65535 f \n`;
  for (let i = 1; i <= 5; i++) {
    pdf += `${String(offsets[i]).padStart(10, '0')} 00000 n \n`;
  }
  pdf += `trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n${xrefStart}\n%%EOF\n`;
  return Buffer.from(pdf, 'ascii');
}

const QA_CHAR = {
  id: 'qa-f3-char-1',
  name: 'QA F3 Teste',
  description: 'Personagem criado para QA manual F3',
  type: 'player',
  attributes: { forca: 12, destreza: 14, inteligencia: 10, carisma: 8 },
  skills: ['Atletismo', 'Furtividade', 'Percepção'],
  inventory: ['Espada longa', 'Poção de cura', 'Corda (15m)', 'Tocha'],
  quotes: ['Por mim e pela party!'],
  createdAt: '2026-07-31T00:00:00.000Z',
  updatedAt: '2026-07-31T00:00:00.000Z',
};

async function run() {
  console.log('\n═══════════════════════════════════════════');
  console.log('  QA-MOBILE — F3 per-task scenarios');
  console.log(`  Target: ${BASE_URL}`);
  console.log('═══════════════════════════════════════════\n');

  const browser = await chromium.launch({ headless: true });
  const DESKTOP = { width: 1440, height: 900 };
  const MOBILE = { width: 375, height: 812 };

  // ── A. TASK 1: Persistence ─────────────────────────────
  console.log('\n--- A. Persistence (Task 1) ---');

  // A1: seed after boot → reload → character survives
  {
    const ctx = await browser.newContext({ viewport: DESKTOP });
    const page = await ctx.newPage();
    attachConsole(page, 'A1 persistence');
    try {
      await goto(page, '/personagens');
      await page.evaluate((char) => {
        localStorage.setItem('meurpg_characters', JSON.stringify([char]));
      }, QA_CHAR);
      await page.reload({ waitUntil: 'networkidle', timeout: 30000 });
      await page.waitForTimeout(1500);
      const card = page.locator('.character-card .card-name').first();
      await card.waitFor({ state: 'visible', timeout: 6000 });
      const name = (await card.innerText()).trim();
      const url = page.url();
      report('Persistence: seeded character survives reload', name.includes('QA F3 Teste') ? 'PASS' : 'FAIL',
        `card "${name}" @ ${url}`);
      await shot(page, 'task-1-persistence');
      const ovf = await overflow(page);
      report('Persistence: no horizontal overflow after reload', ovf.scrollWidth <= ovf.clientWidth + 1 ? 'PASS' : 'FAIL',
        `scrollWidth=${ovf.scrollWidth} clientWidth=${ovf.clientWidth}`);
    } catch (err) {
      report('Persistence: seeded character survives reload', 'FAIL', err.message.slice(0, 200));
    }
    await ctx.close();
  }

  // A2: corrupt data → app boots without crash
  {
    const ctx = await browser.newContext({ viewport: DESKTOP });
    await ctx.addInitScript(() => {
      localStorage.setItem('meurpg_characters', '{corrupt');
    });
    const page = await ctx.newPage();
    const pageErrors = [];
    page.on('pageerror', (err) => pageErrors.push(err.message));
    attachConsole(page, 'A2 corrupt data');
    try {
      await goto(page, '/personagens');
      await page.waitForTimeout(1200);
      const toolbar = await page.locator('.app-toolbar, mat-toolbar').count();
      const bodyText = await page.locator('body').innerText();
      const crashed = pageErrors.length > 0;
      const hasEmptyState = (await page.locator('.empty-state').count()) > 0;
      report('Persistence: corrupt JSON does not crash app', !crashed && toolbar > 0 ? 'PASS' : 'FAIL',
        `toolbar=${toolbar} pageErrors=${pageErrors.length} emptyState=${hasEmptyState} text="${bodyText.slice(0, 60).replace(/\n/g, ' ')}"`);
      await shot(page, 'task-1-corrupt-data');
    } catch (err) {
      report('Persistence: corrupt JSON does not crash app', 'FAIL', err.message.slice(0, 200));
    }
    await ctx.close();
  }

  // ── B. TASK 2: Home CTAs ────────────────────────────────
  console.log('\n--- B. Home CTAs (Task 2) ---');
  {
    const ctx = await browser.newContext({ viewport: DESKTOP });
    const page = await ctx.newPage();
    attachConsole(page, 'B home CTAs');
    try {
      await goto(page, '/');
      const comecar = page.locator('button', { hasText: 'Começar Jornada' }).first();
      await comecar.click();
      await page.waitForTimeout(1200);
      const url1 = page.url();
      report('Home CTA: "Começar Jornada" navigates to /personagens/novo', url1.includes('/personagens/novo') ? 'PASS' : 'FAIL',
        `URL=${url1}`);
      await shot(page, 'task-2-comecar-jornada');

      await page.goBack({ waitUntil: 'networkidle', timeout: 30000 });
      await page.waitForTimeout(1200);
      const explorar = page.locator('button', { hasText: 'Explorar' }).first();
      await explorar.click();
      await page.waitForTimeout(1200);
      const url2 = page.url();
      report('Home CTA: "Explorar" navigates to /personagens', url2.includes('/personagens') ? 'PASS' : 'FAIL',
        `URL=${url2}`);
      await shot(page, 'task-2-explorar');
    } catch (err) {
      report('Home CTA navigation', 'FAIL', err.message.slice(0, 200));
    }
    await ctx.close();
  }

  // ── C. TASK 3: Empty state action ───────────────────────
  console.log('\n--- C. Empty state action (Task 3) ---');
  {
    const ctx = await browser.newContext({ viewport: DESKTOP });
    const page = await ctx.newPage();
    attachConsole(page, 'C empty state');
    try {
      await goto(page, '/personagens');
      const empty = page.locator('.empty-state');
      await empty.waitFor({ state: 'visible', timeout: 6000 });
      const msg = (await page.locator('.empty-message').innerText()).trim();
      const btn = empty.locator('button', { hasText: 'Criar Personagem' });
      const btnCount = await btn.count();
      report('Empty state visible with "Criar Personagem" action', btnCount > 0 ? 'PASS' : 'FAIL',
        `message="${msg}" button=${btnCount}`);
      await shot(page, 'task-3-empty-state');
      await btn.click();
      await page.waitForTimeout(1200);
      const url = page.url();
      report('Empty state action navigates to /personagens/novo', url.includes('/personagens/novo') ? 'PASS' : 'FAIL',
        `URL=${url}`);
      await shot(page, 'task-3-novo-page');
    } catch (err) {
      report('Empty state action', 'FAIL', err.message.slice(0, 200));
    }
    await ctx.close();
  }

  // ── D. TASK 4: Dialogs (dice roller + quick search) ─────
  console.log('\n--- D. Dialogs (Task 4) ---');
  {
    const ctx = await browser.newContext({ viewport: DESKTOP });
    const page = await ctx.newPage();
    attachConsole(page, 'D dialogs');
    try {
      await goto(page, '/sessao');

      // dice roller: open → 1 overlay → close → 0 → reopen → 1 → close → 0
      const rollBtn = page.locator('button[aria-label="Rolar Dados"]');
      await rollBtn.click();
      await page.waitForTimeout(900);
      let dlg = await dialogCount(page);
      report('Dice dialog opens (single overlay)', dlg === 1 ? 'PASS' : 'FAIL', `dialogs=${dlg}`);
      await shot(page, 'task-4-dice-dialog');

      await page.keyboard.press('Escape');
      await page.waitForTimeout(600);
      dlg = await dialogCount(page);
      report('Dice dialog closes (0 overlays)', dlg === 0 ? 'PASS' : 'FAIL', `dialogs=${dlg}`);

      await rollBtn.click();
      await page.waitForTimeout(900);
      dlg = await dialogCount(page);
      report('Dice dialog reopens (single overlay)', dlg === 1 ? 'PASS' : 'FAIL', `dialogs=${dlg}`);
      await page.keyboard.press('Escape');
      await page.waitForTimeout(600);
      dlg = await dialogCount(page);
      report('Dice dialog closes again', dlg === 0 ? 'PASS' : 'FAIL', `dialogs=${dlg}`);

      // quick search reachability: session.component is NOT routed (session.routes → cockpit)
      const qsBtn = page.locator('button[aria-label="Pesquisa rápida"]');
      const qsBtnCount = await qsBtn.count();
      report('Quick search trigger reachable via UI', qsBtnCount > 0 ? 'PASS' : 'FAIL',
        qsBtnCount > 0 ? 'session-search-btn present' : 'NOT reachable — session.component not routed (/sessao loads cockpit); host component of SessionQuickSearchComponent is un-routed');
    } catch (err) {
      report('Dialogs (Task 4)', 'FAIL', err.message.slice(0, 200));
    }
    await ctx.close();
  }

  // ── E. Regression re-check: dice roll (baseline FAIL) ────
  console.log('\n--- E. Regression re-check: dice roll (baseline FAIL) ---');
  {
    const ctx = await browser.newContext({ viewport: DESKTOP });
    const page = await ctx.newPage();
    attachConsole(page, 'E dice roll');
    try {
      await goto(page, '/sessao');
      await page.locator('button[aria-label="Rolar Dados"]').click();
      await page.waitForTimeout(900);
      const input = page.locator('.notation-field input');
      await input.waitFor({ state: 'visible', timeout: 5000 });
      await input.fill('2d6+3');
      await page.waitForTimeout(800);
      const btn = page.locator('.roll-btn');
      const disabled = await btn.isDisabled();
      const inputValue = await input.inputValue();
      report('Dice: Rolar button enabled after typing notation', disabled === false ? 'PASS' : 'FAIL',
        `input="${inputValue}" disabled=${disabled}`);
      if (!disabled) {
        await btn.click();
        await page.waitForTimeout(1200);
        const result = await page.locator('.result-card').count();
        report('Dice: roll produces result', result > 0 ? 'PASS' : 'FAIL', `resultCards=${result}`);
        await shot(page, 'task-4-dice-roll-result');
      }
      await page.keyboard.press('Escape');
      await page.waitForTimeout(400);
    } catch (err) {
      report('Dice roll regression re-check', 'FAIL', err.message.slice(0, 200));
    }
    await ctx.close();
  }

  // ── F. MOBILE 375x812 (Tasks 6, 7, 8, 9, 10) ────────────
  console.log('\n--- F. Mobile 375x812 ---');

  // F1 — Task 6: QuickSearch width ≤ 375
  {
    const ctx = await browser.newContext({ viewport: MOBILE });
    const page = await ctx.newPage();
    attachConsole(page, 'F1 quicksearch mobile');
    try {
      await goto(page, '/sessao');
      let qsWidth = 0;
      let method = '';
      const qsBtn = page.locator('button[aria-label="Pesquisa rápida"]');
      if ((await qsBtn.count()) > 0) {
        await qsBtn.click();
        method = 'ui-button';
      } else {
        // fallback: open the actual component through the app's own DI (session.component un-routed)
        const res = await page.evaluate(async () => {
          const shell = document.querySelector('app-shell');
          const ng = window.ng;
          if (!shell || !ng || !ng.getComponent) return { ok: false, reason: 'no ng global' };
          const comp = ng.getComponent(shell);
          if (!comp || !comp.dialog) return { ok: false, reason: 'no dialog svc' };
          try {
            const mod = await import('/src/app/features/session/session-quick-search.component.ts');
            comp.dialog.open(mod.SessionQuickSearchComponent, { autoFocus: false });
            return { ok: true };
          } catch (e) {
            return { ok: false, reason: String(e) };
          }
        });
        if (res.ok) {
          method = 'di-fallback (session.component un-routed)';
        } else {
          method = `unreachable (${res.reason})`;
        }
      }
      await page.waitForTimeout(1200);
      const box = await page.locator('.qs-container').boundingBox();
      if (box) {
        qsWidth = box.width;
        report('QuickSearch container width ≤ 375px (mobile)', qsWidth <= 375 ? 'PASS' : 'FAIL',
          `width=${qsWidth}px viewport=375 method=${method}`);
        const ovf = await overflow(page);
        report('QuickSearch: no horizontal overflow', ovf.scrollWidth <= ovf.clientWidth + 1 ? 'PASS' : 'FAIL',
          `scrollWidth=${ovf.scrollWidth} clientWidth=${ovf.clientWidth}`);
      } else {
        report('QuickSearch container width ≤ 375px (mobile)', 'FAIL',
          `qs-container not rendered (method=${method})`);
      }
      await shot(page, 'task-6-quicksearch-mobile');
      await page.keyboard.press('Escape');
      await page.waitForTimeout(400);
    } catch (err) {
      report('QuickSearch mobile', 'FAIL', err.message.slice(0, 200));
    }
    await ctx.close();
  }

  // F2 — Task 7: cockpit stacks vertically
  {
    const ctx = await browser.newContext({ viewport: MOBILE });
    const page = await ctx.newPage();
    attachConsole(page, 'F2 cockpit mobile');
    try {
      await goto(page, '/sessao');
      const dir = await page.locator('.split-container').evaluate((el) => getComputedStyle(el).flexDirection);
      const left = await page.locator('.panel-left').boundingBox();
      const right = await page.locator('.panel-right').boundingBox();
      const ovf = await overflow(page);
      const stacked = dir === 'column' && left && right && Math.abs(left.width - 375) < 60 && Math.abs(right.width - 375) < 60;
      report('Cockpit panels stack vertically on mobile', stacked ? 'PASS' : 'FAIL',
        `flexDirection=${dir} leftW=${left?.width} rightW=${right?.width}`);
      report('Cockpit: no horizontal overflow (mobile)', ovf.scrollWidth <= ovf.clientWidth + 1 ? 'PASS' : 'FAIL',
        `scrollWidth=${ovf.scrollWidth} clientWidth=${ovf.clientWidth}`);
      await shot(page, 'task-7-cockpit-mobile');
    } catch (err) {
      report('Cockpit mobile', 'FAIL', err.message.slice(0, 200));
    }
    await ctx.close();
  }

  // F3 — Task 8: CharacterSheet dynamic rows wrap
  {
    const ctx = await browser.newContext({ viewport: MOBILE });
    await ctx.addInitScript((char) => {
      localStorage.setItem('meurpg_characters', JSON.stringify([char]));
    }, QA_CHAR);
    const page = await ctx.newPage();
    attachConsole(page, 'F3 sheet mobile');
    try {
      await goto(page, '/personagens/qa-f3-char-1');
      const tab = page.locator('.mat-mdc-tab', { hasText: 'Ficha' });
      if (await tab.count() > 0) {
        await tab.click();
        await page.waitForTimeout(900);
      }
      const sheet = page.locator('.character-sheet');
      await sheet.waitFor({ state: 'visible', timeout: 6000 });
      const rowWrap = await page.locator('.dynamic-row').first().evaluate((el) => getComputedStyle(el).flexWrap);
      const ovf = await overflow(page);
      report('CharacterSheet dynamic rows wrap on mobile', rowWrap === 'wrap' ? 'PASS' : 'FAIL',
        `flexWrap=${rowWrap}`);
      report('CharacterSheet: no horizontal overflow (mobile)', ovf.scrollWidth <= ovf.clientWidth + 1 ? 'PASS' : 'FAIL',
        `scrollWidth=${ovf.scrollWidth} clientWidth=${ovf.clientWidth}`);
      await shot(page, 'task-8-sheet-mobile');
    } catch (err) {
      report('CharacterSheet mobile', 'FAIL', err.message.slice(0, 200));
    }
    await ctx.close();
  }

  // F4 — Task 9: RulesReader sidebar fallback (real PDF upload)
  {
    const ctx = await browser.newContext({ viewport: MOBILE });
    const page = await ctx.newPage();
    attachConsole(page, 'F4 rules mobile');
    try {
      await goto(page, '/regras');
      const addBtn = page.locator('button', { hasText: 'Adicionar Livro' }).first();
      await addBtn.click();
      await page.waitForTimeout(600);
      const fileInput = page.locator('input[type="file"]');
      await fileInput.setInputFiles({
        name: 'livro-qa.pdf',
        mimeType: 'application/pdf',
        buffer: makePdf('Livro de Regras QA F3'),
      });
      // wait for upload + rule card
      const ruleCard = page.locator('.rule-card').first();
      await ruleCard.waitFor({ state: 'visible', timeout: 15000 });
      report('Rules: PDF upload creates rule', 'PASS', `ruleCard="${(await ruleCard.innerText()).slice(0, 40).replace(/\n/g, ' ')}"`);
      await ruleCard.click();
      await page.waitForTimeout(4000); // pdf.js render

      const layout = page.locator('.reader-layout');
      await layout.waitFor({ state: 'visible', timeout: 10000 });
      const layoutDir = await layout.evaluate((el) => getComputedStyle(el).flexDirection);
      const sidebarBox = await page.locator('.bookmarks-sidebar').boundingBox();
      const ovf = await overflow(page);
      const sidebarOk = sidebarBox && sidebarBox.width <= 375 && sidebarBox.width > 300;
      report('RulesReader sidebar stacks on mobile', layoutDir === 'column' && sidebarOk ? 'PASS' : 'FAIL',
        `layoutFlex=${layoutDir} sidebarW=${sidebarBox?.width}px`);
      report('RulesReader: no horizontal overflow (mobile)', ovf.scrollWidth <= ovf.clientWidth + 1 ? 'PASS' : 'FAIL',
        `scrollWidth=${ovf.scrollWidth} clientWidth=${ovf.clientWidth}`);
      await shot(page, 'task-9-rules-mobile');
    } catch (err) {
      report('RulesReader mobile', 'FAIL', err.message.slice(0, 200));
    }
    await ctx.close();
  }

  // F5 — Task 10: dialogs fit mobile viewport (dice + confirm)
  {
    const ctx = await browser.newContext({ viewport: MOBILE });
    const page = await ctx.newPage();
    attachConsole(page, 'F5 dialogs mobile');
    try {
      // dice-roller dialog
      await goto(page, '/sessao');
      await page.locator('button[aria-label="Rolar Dados"]').click();
      await page.waitForTimeout(900);
      const diceBox = await page.locator('.mat-mdc-dialog-container').boundingBox();
      report('Dice dialog width ≤ 375px (mobile)', diceBox && diceBox.width <= 375 ? 'PASS' : 'FAIL',
        `width=${diceBox?.width}px`);
      await shot(page, 'task-10-dice-mobile');
      await page.keyboard.press('Escape');
      await page.waitForTimeout(500);

      // confirm dialog via campaign flow
      await goto(page, '/campanha');
      const newFolder = page.locator('.new-folder-btn');
      await newFolder.click();
      await page.waitForTimeout(800);
      await page.locator('button[aria-label="Excluir pasta"]').first().click();
      await page.waitForTimeout(900);
      const confirmBox = await page.locator('.mat-mdc-dialog-container').boundingBox();
      report('Confirm dialog width ≤ 375px (mobile)', confirmBox && confirmBox.width <= 375 ? 'PASS' : 'FAIL',
        `width=${confirmBox?.width}px`);
      await shot(page, 'task-10-confirm-mobile');
      await shot(page, 'task-10-dialogs-mobile');
      await page.keyboard.press('Escape');
      await page.waitForTimeout(400);
    } catch (err) {
      report('Dialogs mobile (Task 10)', 'FAIL', err.message.slice(0, 200));
    }
    await ctx.close();
  }

  // ── G. DESKTOP 1440x900 spot-check (Tasks 6, 7, 10) ─────
  console.log('\n--- G. Desktop 1440x900 spot-check ---');
  {
    const ctx = await browser.newContext({ viewport: DESKTOP });
    const page = await ctx.newPage();
    attachConsole(page, 'G desktop');
    try {
      await goto(page, '/sessao');

      // G1 — QuickSearch ≈ 560px (DI fallback as session.component un-routed)
      const res = await page.evaluate(async () => {
        const shell = document.querySelector('app-shell');
        const ng = window.ng;
        if (!shell || !ng || !ng.getComponent) return { ok: false };
        const comp = ng.getComponent(shell);
        if (!comp || !comp.dialog) return { ok: false };
        const mod = await import('/src/app/features/session/session-quick-search.component.ts');
        comp.dialog.open(mod.SessionQuickSearchComponent, { autoFocus: false });
        return { ok: true };
      });
      await page.waitForTimeout(1200);
      const qsBox = await page.locator('.qs-container').boundingBox();
      if (qsBox) {
        report('QuickSearch desktop width ≈ 560px', qsBox.width >= 500 && qsBox.width <= 620 ? 'PASS' : 'FAIL',
          `width=${qsBox.width}px (opened via DI fallback, session.component un-routed)`);
      } else {
        report('QuickSearch desktop width ≈ 560px', res.ok ? 'FAIL' : 'WARN',
          `not rendered (diOk=${res.ok})`);
      }
      await shot(page, 'task-6-quicksearch-desktop');
      await page.keyboard.press('Escape');
      await page.waitForTimeout(500);

      // G2 — cockpit panels side-by-side
      const dir = await page.locator('.split-container').evaluate((el) => getComputedStyle(el).flexDirection);
      const left = await page.locator('.panel-left').boundingBox();
      const right = await page.locator('.panel-right').boundingBox();
      const sideBySide = dir === 'row' && left && right && right.x > left.x + left.width - 5;
      report('Cockpit panels side-by-side on desktop', sideBySide ? 'PASS' : 'FAIL',
        `flexDirection=${dir} leftX=${left?.x} leftW=${left?.width} rightX=${right?.x} rightW=${right?.width}`);
      await shot(page, 'task-7-cockpit-desktop');

      // G3 — dice dialog desktop size (config width 400px)
      await page.locator('button[aria-label="Rolar Dados"]').click();
      await page.waitForTimeout(900);
      const diceBox = await page.locator('.mat-mdc-dialog-container').boundingBox();
      report('Dice dialog desktop ≤ 400px', diceBox && diceBox.width <= 400 + 1 ? 'PASS' : 'FAIL',
        `width=${diceBox?.width}px`);
      await shot(page, 'task-10-dice-desktop');
      await page.keyboard.press('Escape');
      await page.waitForTimeout(400);
    } catch (err) {
      report('Desktop spot-check', 'FAIL', err.message.slice(0, 200));
    }
    await ctx.close();
  }

  // ── H. Edge: /mapa console error ─────────────────────────
  console.log('\n--- H. Edge: /mapa console error ---');
  {
    const ctx = await browser.newContext({ viewport: DESKTOP });
    const page = await ctx.newPage();
    attachConsole(page, 'H mapa');
    try {
      await goto(page, '/mapa');
      await shot(page, 'edge-mapa-page');
      const bodyText = (await page.locator('body').innerText()).slice(0, 80).replace(/\n/g, ' ');
      report('Mapa page renders (edge check)', bodyText.length > 0 ? 'PASS' : 'FAIL', `text="${bodyText}"`);
    } catch (err) {
      report('Mapa edge check', 'FAIL', err.message.slice(0, 200));
    }
    await ctx.close();
  }

  // ── Summary ──────────────────────────────────────────────
  console.log('\n═══════════════════════════════════════════');
  console.log('  RESULTS SUMMARY (qa-mobile)');
  console.log('═══════════════════════════════════════════');

  let passed = 0, failed = 0, warned = 0;
  for (const r of results) {
    if (r.status === 'PASS') passed++;
    else if (r.status === 'FAIL') failed++;
    else if (r.status === 'WARN') warned++;
    console.log(`  ${r.status === 'PASS' ? '✅' : r.status === 'FAIL' ? '❌' : '⚠️'}  ${r.scenario}: ${r.detail}`);
  }
  console.log(`\n  Results: ${passed} passed, ${failed} failed, ${warned} warnings`);
  console.log(`  Console/Page Errors captured: ${allConsoleErrors.length}`);

  const reportData = {
    timestamp: new Date().toISOString(),
    baseUrl: BASE_URL,
    script: 'qa-mobile.mjs',
    scenarios: results,
    consoleErrors: allConsoleErrors,
    summary: { passed, failed, warned, total: results.length },
  };
  const reportPath = resolve(EVIDENCE_DIR, 'qa-report-f3.json');
  writeFileSync(reportPath, JSON.stringify(reportData, null, 2));
  console.log(`\n  Report saved: ${reportPath}`);

  await browser.close();
  console.log('  Browser closed.\n');
  process.exit(failed > 0 ? 1 : 0);
}

run().catch((err) => {
  console.error('Fatal error:', err);
  process.exit(1);
});
