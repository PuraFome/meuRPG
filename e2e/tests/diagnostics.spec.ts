import { spawnSync } from 'node:child_process';
import path from 'node:path';

import { expect, type Browser, type Page, type TestInfo } from '@playwright/test';

import { PageDiagnostics, PROBE_DEADLINE_MS, test } from './diagnostics';
import { newSignedInContext } from './support';

// The support that explains a test stuck on a blank page (diagnostics.ts). These tests never touch the app: the pages
// are made here, with content set by hand, and made to crash, hang or fail on purpose.

// A page on an origin of its own, so the script can fetch with relative addresses; nothing leaves the browser.
async function serve(page: Page, html: string): Promise<void> {
  await page.route('http://diagnostics.test/', (route) => route.fulfill({ contentType: 'text/html; charset=utf-8', body: html }));
  await page.goto('http://diagnostics.test/');
}

async function openWatched(browser: Browser, diagnostics: PageDiagnostics) {
  const restore = diagnostics.watchBrowser(browser);
  const context = await browser.newContext();
  restore();
  return context;
}

test('uma página que responde aparece no relatório como respondendo', async ({ browser }) => {
  const diagnostics = new PageDiagnostics();
  const context = await openWatched(browser, diagnostics);
  try {
    const page = await context.newPage();
    await page.setContent('<p>pronto</p>');
    await diagnostics.probe();
    expect(diagnostics.report()).toMatch(/context 1, page 1 .*\n {2}state: answers \(readyState complete\)/);
  } finally {
    await context.close();
  }
});

test('uma página que travou diz que não responde e lista o pedido que ficou aberto', async ({ browser }) => {
  const diagnostics = new PageDiagnostics();
  const context = await openWatched(browser, diagnostics);
  try {
    const page = await context.newPage();
    await page.route('**/nunca-responde', () => {});
    await serve(page, '<script>fetch("/nunca-responde").catch(() => {});</script>');
    await expect.poll(() => diagnostics.report() ?? '').toContain('/nunca-responde');
    // The loop never ends: the evaluation below never settles, and only the probe's deadline ends the wait.
    let busyStarted!: () => void;
    const busy = new Promise<void>((resolve) => (busyStarted = resolve));
    await page.exposeFunction('busyStarted', () => busyStarted());
    void page.evaluate(() => {
      void (window as unknown as { busyStarted(): void }).busyStarted();
      for (;;) {
        // busy
      }
    }).catch(() => undefined);
    await busy;
    const started = Date.now();
    await diagnostics.probe();
    expect(Date.now() - started).toBeLessThan(PROBE_DEADLINE_MS * 4);
    const report = diagnostics.report() ?? '';
    expect(report).toContain('state: does not answer: no reply to a probe within');
    expect(report).toMatch(/requests still open \(\d+\):\n(?: {4}.*\n)* {4}GET \S+\/nunca-responde \(\d+ ms\)/);
  } finally {
    await context.close();
  }
});

test('uma página que caiu aparece como caiu', async ({ browser }) => {
  const diagnostics = new PageDiagnostics();
  const context = await openWatched(browser, diagnostics);
  try {
    const page = await context.newPage();
    const crashed = page.waitForEvent('crash');
    await page.goto('chrome://crash').catch(() => undefined);
    await crashed;
    await diagnostics.probe();
    expect(diagnostics.report()).toContain('state: crashed');
  } finally {
    await context.close();
  }
});

test('o erro da página, o erro do console e o pedido que falhou entram no relatório', async ({ browser }) => {
  const diagnostics = new PageDiagnostics();
  const context = await openWatched(browser, diagnostics);
  try {
    const page = await context.newPage();
    await page.route('**/cai', (route) => route.abort());
    await serve(page, '<script>console.error("falhou no console"); setTimeout(() => { throw new Error("falhou na página"); }); fetch("/cai").catch(() => {});</script>');
    await expect.poll(() => diagnostics.report() ?? '').toContain('request failed: GET');
    await expect.poll(() => diagnostics.report() ?? '').toContain('pageerror: falhou na página');
    expect(diagnostics.report()).toContain('console.error: falhou no console');
  } finally {
    await context.close();
  }
});

test('o relatório só vai para o teste que falhou', async ({ browser }) => {
  const diagnostics = new PageDiagnostics();
  const context = await openWatched(browser, diagnostics);
  try {
    await (await context.newPage()).setContent('<p>pronto</p>');
    const attached: string[] = [];
    const testInfo = (status: string) => ({ status, attach: (name: string) => void attached.push(`${status}:${name}`) }) as unknown as TestInfo;
    for (const status of ['passed', 'interrupted', 'skipped']) await diagnostics.attachIfFailed(testInfo(status));
    expect(attached).toEqual([]);
    await diagnostics.attachIfFailed(testInfo('failed'));
    // Once per test, whichever fixture gets there first.
    await diagnostics.attachIfFailed(testInfo('failed'));
    expect(attached).toEqual(['failed:page-diagnostics']);
  } finally {
    await context.close();
  }
});

test('o teste vê as páginas do fixture, de browser.newContext e de newSignedInContext sem pedir nada', async ({ browser, page, pageDiagnostics }) => {
  await page.setContent('<p>do fixture</p>');
  const manual = await browser.newContext();
  const signedIn = await newSignedInContext(browser, 'Mestre Teste');
  try {
    await (await manual.newPage()).setContent('<p>manual</p>');
    await (await signedIn.newPage()).setContent('<p>com sessão</p>');
    const report = pageDiagnostics.report() ?? '';
    expect(report).toContain('context 1, page 1');
    expect(report).toContain('context 2, page 1');
    expect(report).toContain('context 3, page 1');
  } finally {
    await manual.close();
    await signedIn.close();
  }
});

test('um teste que falha anexa o diagnóstico: o pedido que não voltou no fixture e a página que caiu num contexto fechado pelo teste', async () => {
  // A nested run of diagnostics-fixture, whose tests fail on purpose (the attachment is made after the test body,
  // so only a real failing test shows it). Two failures of a few seconds each, hence the longer timeout.
  test.setTimeout(90_000);
  const e2e = path.join(__dirname, '..');
  const { PLAYWRIGHT_JSON_OUTPUT_NAME: _name, ...env } = process.env;
  const run = spawnSync('npx', ['playwright', 'test', '-c', 'diagnostics-fixture/playwright.config.ts', '--reporter=json'], { cwd: e2e, env, encoding: 'utf8', maxBuffer: 1 << 26 });
  type Result = { status: string; attachments: { name: string; body?: string }[] };
  type Suite = { specs?: { title: string; tests: { results: Result[] }[] }[]; suites?: Suite[] };
  const results = new Map<string, Result>();
  const walk = (suite: Suite) => {
    for (const spec of suite.specs ?? []) results.set(spec.title, spec.tests[0].results[0]);
    suite.suites?.forEach(walk);
  };
  walk({ suites: (JSON.parse(run.stdout) as { suites: Suite[] }).suites });
  const attachment = (title: string) => {
    const result = results.get(title);
    expect(result?.status, `${title} must fail`).toMatch(/failed|timedOut/);
    const found = result?.attachments.find((a) => a.name === 'page-diagnostics');
    expect(found, `${title} must attach page-diagnostics`).toBeDefined();
    return Buffer.from(found?.body ?? '', 'base64').toString('utf8');
  };
  const waiting = attachment('a click whose answer never arrives');
  expect(waiting).toContain('state: answers');
  expect(waiting).toMatch(/requests still open \(1\):\n {4}GET http:\/\/diagnostics.test\/never/);
  expect(attachment('a page that crashed in a context the test closes itself')).toContain('state: crashed');
});
