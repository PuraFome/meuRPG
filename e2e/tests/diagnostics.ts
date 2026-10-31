import { test as base, type Browser, type BrowserContext, type ConsoleMessage, type Page, type Request, type TestInfo } from '@playwright/test';

/**
 * The `test` every spec imports (the lint rejects `test` from '@playwright/test'). It adds an automatic fixture that
 * watches every page of every context the test opens (the `page` and `context` fixtures, `browser.newContext()`,
 * `newSignedInContext` and the other helpers that call it) and, when the test fails, attaches `page-diagnostics`: what
 * each page reported (a crash, `pageerror`, console errors, failed requests), whether it still answers, and which
 * requests the browser still has open. A click that waits 20 s on a blank page then says whether the page was dead,
 * busy or waiting for an answer that never came (docs/testing.md, "A page that hangs").
 */

/** How long a page gets to answer the probe; the probe never keeps a test or its teardown waiting longer. */
export const PROBE_DEADLINE_MS = 1_500;

// A page that fails in a loop must not fill the report.
const MAX_LINES = 40;
const MAX_TEXT = 400;

type OpenRequest = { description: string; since: number };

type Probe = { answers: string; openRequests: string[] };

type Watched = {
  label: string;
  page: Page;
  crashed: boolean;
  errors: string[];
  failedRequests: string[];
  open: Map<Request, OpenRequest>;
  probe?: Probe;
};

function clip(text: string): string {
  return text.length > MAX_TEXT ? `${text.slice(0, MAX_TEXT)}…` : text;
}

function firstLine(error: unknown): string {
  const text = error instanceof Error ? error.message : String(error);
  return clip(text.split('\n')[0] ?? '');
}

/** What the probe of one page found, one block of the report. */
function describe(watched: Watched, now: number): string[] {
  const { page, label, probe } = watched;
  const lines = [`${label}  ${page.url() || '(no url yet)'}`];
  const state = watched.crashed ? 'crashed' : page.isClosed() && !probe ? 'closed before it was probed' : (probe?.answers ?? 'not probed');
  lines.push(`  state: ${state}`);
  for (const error of watched.errors) lines.push(`  ${error}`);
  for (const failed of watched.failedRequests) lines.push(`  ${failed}`);
  const open = probe?.openRequests ?? [...watched.open.values()].map((request) => `${request.description} (${now - request.since} ms)`);
  if (open.length > 0) {
    lines.push(`  requests still open (${open.length}):`);
    for (const request of open.slice(0, MAX_LINES)) lines.push(`    ${request}`);
  }
  return lines;
}

export class PageDiagnostics {
  private readonly pages: Watched[] = [];
  private readonly contexts = new WeakSet<BrowserContext>();
  private contextCount = 0;

  /** Watches the pages the context has and the ones it opens later, and probes its pages before it closes. */
  watchContext(context: BrowserContext): void {
    if (this.contexts.has(context)) return;
    this.contexts.add(context);
    const name = `context ${++this.contextCount}`;
    let pageCount = 0;
    const watch = (page: Page) => this.watchPage(page, `${name}, page ${++pageCount}`);
    context.pages().forEach(watch);
    context.on('page', watch);
    // The test body usually closes its contexts itself, in a `finally`, before anything knows that it failed:
    // the evidence is taken while the pages are still there.
    const close = context.close.bind(context);
    context.close = async (options) => {
      await this.probe(this.pages.filter((watched) => watched.page.context() === context));
      return close(options);
    };
  }

  watchPage(page: Page, label: string): void {
    const watched: Watched = { label, page, crashed: false, errors: [], failedRequests: [], open: new Map() };
    this.pages.push(watched);
    const note = (list: string[], line: string) => {
      if (list.length < MAX_LINES) list.push(clip(line));
    };
    page.on('crash', () => {
      watched.crashed = true;
    });
    page.on('pageerror', (error) => note(watched.errors, `pageerror: ${firstLine(error)}`));
    page.on('console', (message: ConsoleMessage) => {
      if (message.type() === 'error') note(watched.errors, `console.error: ${message.text()} (${message.location().url}:${message.location().lineNumber})`);
    });
    page.on('request', (request) => watched.open.set(request, { description: `${request.method()} ${request.url()}`, since: Date.now() }));
    page.on('requestfinished', (request) => watched.open.delete(request));
    page.on('requestfailed', (request) => {
      watched.open.delete(request);
      note(watched.failedRequests, `request failed: ${request.method()} ${request.url()}: ${request.failure()?.errorText ?? 'unknown'}`);
    });
  }

  /** Watches what the browser opens from now on, until the returned function puts it back. */
  watchBrowser(browser: Browser): () => void {
    const newContext = browser.newContext.bind(browser);
    const newPage = browser.newPage.bind(browser);
    browser.newContext = async (options) => {
      const context = await newContext(options);
      this.watchContext(context);
      return context;
    };
    browser.newPage = async (options) => {
      const page = await newPage(options);
      this.watchContext(page.context());
      return page;
    };
    return () => {
      browser.newContext = newContext;
      browser.newPage = newPage;
    };
  }

  /**
   * Asks every open page that was not probed yet whether it answers. Each probe has its own deadline and they run
   * together, so this never takes longer than the deadline; a probe that never returns is abandoned.
   */
  async probe(watched: Watched[] = this.pages): Promise<void> {
    await Promise.all(
      watched.map(async (entry) => {
        if (entry.probe || entry.page.isClosed()) return;
        // A crashed page has nothing to answer with.
        if (entry.crashed) {
          entry.probe = { answers: 'crashed', openRequests: [...entry.open.values()].map((request) => request.description) };
          return;
        }
        const started = Date.now();
        entry.probe = { answers: 'probe running', openRequests: [] };
        let timer: NodeJS.Timeout | undefined;
        const deadline = new Promise<string>((resolve) => {
          timer = setTimeout(() => resolve(`does not answer: no reply to a probe within ${PROBE_DEADLINE_MS} ms`), PROBE_DEADLINE_MS);
        });
        // The evaluation of a page that is stuck never settles: it is left behind, and its late rejection is read here.
        const answer = entry.page.evaluate(() => document.readyState).then(
          (readyState) => `answers (readyState ${readyState}) in ${Date.now() - started} ms`,
          (error: unknown) => `does not answer: ${firstLine(error)}`,
        );
        const answers = await Promise.race([answer, deadline]);
        clearTimeout(timer);
        const now = Date.now();
        entry.probe = { answers, openRequests: [...entry.open.values()].map((request) => `${request.description} (${now - request.since} ms)`) };
      }),
    );
  }

  /** The text of the attachment, or undefined when the test opened no page. */
  report(): string | undefined {
    if (this.pages.length === 0) return undefined;
    const now = Date.now();
    const blocks = this.pages.map((watched) => describe(watched, now).join('\n'));
    return ['Pages the test opened, as the browser saw them when it failed.', '', ...blocks].join('\n') + '\n';
  }

  /** Probes what is still open and attaches the report when the test failed. */
  async attachIfFailed(testInfo: TestInfo): Promise<void> {
    if (testInfo.status !== 'failed' && testInfo.status !== 'timedOut') return;
    await this.probe();
    const body = this.report();
    if (body) await testInfo.attach('page-diagnostics', { body, contentType: 'text/plain' });
  }
}

export const test = base.extend<{ pageDiagnostics: PageDiagnostics }>({
  pageDiagnostics: [
    async ({ browser }, use, testInfo) => {
      const diagnostics = new PageDiagnostics();
      const restore = diagnostics.watchBrowser(browser);
      try {
        await use(diagnostics);
      } finally {
        restore();
        await diagnostics.attachIfFailed(testInfo);
      }
    },
    { auto: true },
  ],
});
