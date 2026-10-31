// Unit tests of flake-targets.mjs: node --test .github/scripts/
import fs from 'node:fs';
import assert from 'node:assert/strict';
import test from 'node:test';

import { changedLines, mapDiff, parseSpec, playwrightTests, sample, signinsOf } from './flake-targets.mjs';

const spec = `import { test, expect } from '@playwright/test';
import { openThing } from './thing-support';

test.describe('a group', () => {
  test('first (1.5) test', async ({ page }) => {
    await openThing(page);
    await expect(page).toHaveURL('/x');
  });

  test('second test', async ({ page }) => {
    await page.goto('/y');
  });
});

for (const theme of ['claro', 'escuro']) {
  test(\`tema \${theme} passa no axe\`, { tag: '@a11y' }, async ({ page }) => {
    await page.goto('/z');
  });
}

test('one line', async () => {});
`;

const other = `import { openThing } from './thing-support';

test('another caller', async ({ page }) => {
  await openThing(page);
});

test('no caller', async () => {
  await Promise.resolve();
});
`;

const support = `import type { Page } from '@playwright/test';

/** Opens the thing. */
export async function openThing(page: Page): Promise<void> {
  await page.goto('/thing');
}

function inner(): number {
  return 1;
}

export function unused(): number {
  return 2;
}

export const limit = 2;
`;

const goTest = `package play

import "testing"

func helper() int {
	return 1
}

func TestA(t *testing.T) {
	t.Run("sub", func(t *testing.T) {
		_ = helper()
	})
}

func TestB(t *testing.T) {
	_ = 2
}
`;

const leaktest = `package leaktest

import "testing"

func seed(t *testing.T) int {
	return 1
}

func TestUsesSeed(t *testing.T) {
	_ = seed(t)
}

func TestAlsoUsesSeed(t *testing.T) {
	_ = seed(t)
}

func TestNeverUsesIt(t *testing.T) {}
`;

const files = {
  'e2e/tests/a.spec.ts': spec,
  'e2e/tests/b.spec.ts': other,
  'e2e/tests/thing-support.ts': support,
  'backend/internal/play/x_test.go': goTest,
  'backend/internal/leaktest/harness_test.go': leaktest,
};
const read = (f) => files[f];
const listFiles = (prefix) => Object.keys(files).filter((f) => f.startsWith(prefix));
const hunk = (file, from, to) => `--- a/${file}\n+++ b/${file}\n@@ -${from},${to - from + 1} +${from},${to - from + 1} @@\n`;
const map = (diff) => mapDiff({ diff, read, files: listFiles });

test('changedLines reads the new-side ranges of a -U0 diff, a deletion as the line it leaves', () => {
  const diff = '--- a/f\n+++ b/f\n@@ -3 +3 @@\n-a\n+b\n@@ -10,2 +9,0 @@\n-x\n-y\n@@ -20,0 +21,3 @@\n+1\n+2\n+3\n--- a/gone\n+++ /dev/null\n@@ -1,4 +0,0 @@\n-z\n';
  assert.deepEqual(changedLines(diff), { f: [[3, 3], [9, 9, true], [21, 23]] });
});

test('playwrightTests escapes a title and turns an interpolation into a wildcard, and skips groups and hooks', () => {
  const [plain] = playwrightTests("test('first (1.5) test', async () => {});");
  assert.deepEqual([plain.title, plain.regex], ['first (1.5) test', 'first \\(1\\.5\\) test']);
  assert.equal(playwrightTests('test(`tema ${theme} passa`, async () => {});')[0].regex, 'tema .* passa');
  assert.deepEqual(playwrightTests("test.describe('a group', () => { test.beforeEach(async () => {}); await expect(page).toHaveURL('/x'); });"), []);
});

test('a test header that Prettier wraps over several lines is found', () => {
  const wrapped = "test(\n  'um título tão comprido que o Prettier quebra a chamada em várias linhas',\n  { tag: '@MR-001' },\n  async ({ page }) => {\n    await page.goto('/');\n  },\n);\n";
  assert.deepEqual(playwrightTests(wrapped).map((t) => [t.title, t.from, t.to]), [['um título tão comprido que o Prettier quebra a chamada em várias linhas', 1, 7]]);
});

test('test.only counts as a test; test.skip, test.fixme and test.step do not', () => {
  assert.deepEqual(playwrightTests("test.only('a', async () => {});\ntest.skip('b', async () => {});\ntest.step('c', async () => {});").map((t) => t.title), ['a']);
});

test('parseSpec gives a test the beforeEach and beforeAll hooks that reach it', () => {
  const [inside] = parseSpec("test.describe('g', () => {\n  test.beforeEach(async ({ page }) => {\n    await signIn(page);\n  });\n  test('t', async () => {});\n});").tests;
  assert.match(inside.hooks, /signIn/);
  assert.deepEqual(inside.hookLines, [[2, 4]]);
});

test('playwrightTests finds each test, one-line ones too, and not the describe', () => {
  assert.deepEqual(
    playwrightTests(spec).map((t) => [t.title, t.from, t.to]),
    [
      ['first (1.5) test', 5, 8],
      ['second test', 10, 12],
      ['tema ${theme} passa no axe', 16, 18],
      ['one line', 21, 21],
    ],
  );
});

test('a changed test body maps to that test alone', () => {
  const got = map(hunk('e2e/tests/a.spec.ts', 11, 11));
  assert.deepEqual(got.playwright, [{ file: 'e2e/tests/a.spec.ts', title: 'second test', grep: 'second test', signins: 0, why: 'changed' }]);
  assert.deepEqual(got.go, []);
});

test('a changed title line counts as a change of the test', () => {
  const got = map(hunk('e2e/tests/a.spec.ts', 5, 5));
  assert.deepEqual(got.playwright.map((t) => t.title), ['first (1.5) test']);
});

test('a changed Go test body maps to its function; a helper of another package maps to nothing', () => {
  const got = map(hunk('backend/internal/play/x_test.go', 11, 11));
  assert.deepEqual(got.go, [{ pkg: './internal/play', test: 'TestA', why: 'changed' }]);
  assert.deepEqual(map(hunk('backend/internal/play/x_test.go', 6, 6)), { playwright: [], go: [], notes: [] });
});

test('a changed exported helper of a *-support.ts maps to the tests that call it, and a helper nobody calls says so', () => {
  const got = map(hunk('e2e/tests/thing-support.ts', 5, 6));
  assert.deepEqual(
    got.playwright.map((t) => [t.file, t.title, t.why]),
    [
      ['e2e/tests/a.spec.ts', 'first (1.5) test', 'calls openThing'],
      ['e2e/tests/b.spec.ts', 'another caller', 'calls openThing'],
    ],
  );
  const none = map(hunk('e2e/tests/thing-support.ts', 12, 12));
  assert.deepEqual(none.playwright, []); // `unused` is exported and nobody calls it
  assert.match(none.notes[0], /no test calls it/);
  assert.deepEqual(map(hunk('e2e/tests/thing-support.ts', 9, 9)), { playwright: [], go: [], notes: [] }); // not exported
  assert.deepEqual(map(hunk('e2e/tests/thing-support.ts', 16, 16)), { playwright: [], go: [], notes: [] }); // a constant, not a function
});

test('a changed leaktest helper maps to the tests that call it', () => {
  const got = map(hunk('backend/internal/leaktest/harness_test.go', 6, 6));
  assert.deepEqual(
    got.go.map((t) => [t.pkg, t.test, t.why]),
    [
      ['./internal/leaktest', 'TestAlsoUsesSeed', 'calls seed'],
      ['./internal/leaktest', 'TestUsesSeed', 'calls seed'],
    ],
  );
});

test('a deletion inside a test maps to it; removing a whole test touches the tests around it no more', () => {
  const inside = map('--- a/e2e/tests/a.spec.ts\n+++ b/e2e/tests/a.spec.ts\n@@ -7,1 +6,0 @@\n-x\n');
  assert.deepEqual(inside.playwright.map((t) => t.title), ['first (1.5) test']);
  const between = map('--- a/e2e/tests/a.spec.ts\n+++ b/e2e/tests/a.spec.ts\n@@ -9,2 +8,0 @@\n-x\n-y\n');
  assert.deepEqual(between.playwright, []); // the blank line between two tests
  assert.deepEqual(map('--- a/e2e/tests/a.spec.ts\n+++ /dev/null\n@@ -1,4 +0,0 @@\n-x\n').playwright, []);
});

test('a change outside any test maps to nothing', () => {
  for (const [file, line] of [['e2e/tests/a.spec.ts', 1], ['e2e/tests/a.spec.ts', 4], ['e2e/tests/a.spec.ts', 14], ['backend/internal/play/x_test.go', 3], ['docs/ci.md', 1]]) {
    assert.deepEqual(map(hunk(file, line, line)), { playwright: [], go: [], notes: [] }, `${file}:${line}`);
  }
});

test('a helper with many callers is sampled, spread and the same every time', () => {
  const items = Array.from({ length: 23 }, (_, i) => i);
  assert.equal(sample(items).length, 5);
  assert.deepEqual(sample(items), sample(items));
  assert.deepEqual(sample([1, 2, 3]), [1, 2, 3]);
  const many = Object.fromEntries(Array.from({ length: 8 }, (_, i) => [`e2e/tests/c${i}.spec.ts`, `test('t${i}', async ({ page }) => {\n  await openThing(page);\n});\n`]));
  const got = mapDiff({
    diff: hunk('e2e/tests/thing-support.ts', 5, 6),
    read: (f) => many[f] ?? files[f],
    files: (p) => [...Object.keys(many), ...Object.keys(files)].filter((f) => f.startsWith(p)),
  });
  assert.equal(got.playwright.length, 5);
  assert.match(got.notes.join('\n'), /has \d+ callers; 5 run/);
});

test('a helper is followed through the helpers that call it, and a Go method counts as a helper', () => {
  const more = {
    ...files,
    'e2e/tests/c.spec.ts': "async function twoPeople(page) {\n  await openThing(page);\n}\n\ntest('through a spec helper', async ({ page }) => {\n  await twoPeople(page);\n});\n",
    'backend/internal/leaktest/table_test.go': 'package leaktest\n\nfunc (rt *table) attack(t *testing.T) {\n\t_ = 1\n}\n\nfunc (rt *table) both(t *testing.T) {\n\trt.attack(t)\n}\n\nfunc TestViaMethods(t *testing.T) {\n\trt := &table{}\n\trt.both(t)\n}\n',
  };
  const m = (diff) => mapDiff({ diff, read: (f) => more[f], files: (p) => Object.keys(more).filter((f) => f.startsWith(p)) });
  const pw = m(hunk('e2e/tests/thing-support.ts', 5, 6));
  assert.ok(pw.playwright.some((t) => t.title === 'through a spec helper'), JSON.stringify(pw.playwright));
  const go = m(hunk('backend/internal/leaktest/table_test.go', 4, 4));
  assert.deepEqual(go.go.map((t) => t.test), ['TestViaMethods']);
});

test('signinsOf counts a closure that signs in, or /auth/login, as a heavy test', async () => {
  const { signinsOf } = await import('./flake-targets.mjs');
  const real = fs.readFileSync('e2e/tests/login.spec.ts', 'utf8');
  const test3 = playwrightTests(real).find((t) => t.title.startsWith('cada pessoa tem uma conta'));
  assert.equal(signinsOf(test3.body), 3);
  assert.equal(signinsOf("await page.goto('/auth/login?return_to=x');"), 3);
  assert.equal(signinsOf('await page.goto("/");'), 0);
});

test('signinsOf counts a sign-in through a helper, and a hook of the file reaches its tests', () => {
  assert.equal(signinsOf('await asMaster(page);', ['signIn', 'asMaster']), 3);
  assert.equal(signinsOf('await asMaster(page);', ['signIn']), 0);
});

test('a test whose file signs in through a helper in beforeEach, or whose helper signs in, is heavy', () => {
  const files = {
    'e2e/tests/x.spec.ts': "test.beforeEach(async ({ page }) => {\n  await asMaster(page);\n});\n\ntest('uses the hook', async ({ page }) => {\n  await page.goto('/');\n});\n",
    'e2e/tests/x-support.ts': "export async function asMaster(page: Page): Promise<void> {\n  await signIn(page);\n}\n\nexport async function signIn(page: Page): Promise<void> {\n  await page.goto('/auth/login');\n}\n",
  };
  const got = mapDiff({ diff: hunk('e2e/tests/x.spec.ts', 5, 6), read: (f) => files[f], files: (prefix) => Object.keys(files).filter((f) => f.startsWith(prefix)) });
  assert.equal(got.playwright[0].signins, 3);
});

test('a hook counts only for the tests inside its own describe', () => {
  const files = {
    'e2e/tests/x.spec.ts': [
      "test.describe('with a hook', () => {",
      '  test.beforeEach(async ({ page }) => {',
      '    await signIn(page);',
      '  });',
      "  test.describe('nested', () => {",
      "    test('inherits the hook', async () => {});",
      '  });',
      '});',
      "test.describe('without one', () => {",
      "  test('outside the hook', async () => {});",
      '});',
      "test('at the top', async () => {});",
      '',
    ].join('\n'),
    'e2e/tests/x-support.ts': "export async function signIn(page: Page): Promise<void> {\n  await page.goto('/auth/login');\n}\n",
  };
  const diff = hunk('e2e/tests/x.spec.ts', 1, 12);
  const got = mapDiff({ diff, read: (f) => files[f], files: (prefix) => Object.keys(files).filter((f) => f.startsWith(prefix)) });
  assert.deepEqual(
    got.playwright.map((t) => [t.title, t.signins]),
    [
      ['at the top', 0],
      ['inherits the hook', 3],
      ['outside the hook', 0],
    ],
  );
});

test('a hook outside every describe reaches all the tests of the file', () => {
  const [a, b] = playwrightTests("test.beforeEach(async () => {\n  await signIn();\n});\ntest.describe('g', () => {\n  test('in', async () => {});\n});\ntest('out', async () => {});");
  assert.match(a.hooks, /signIn/);
  assert.match(b.hooks, /signIn/);
});

test('mapDiff reads and parses each spec once, however many tests and callers ask for it', () => {
  const files = {
    'e2e/tests/x.spec.ts': "test('a', async () => {\n  await openThing();\n});\n\ntest('b', async () => {\n  await openThing();\n});\n",
    'e2e/tests/x-support.ts': 'export async function openThing(): Promise<void> {}\n',
  };
  const reads = [];
  const read = (f) => {
    reads.push(f);
    return files[f];
  };
  const got = mapDiff({ diff: hunk('e2e/tests/x.spec.ts', 1, 7) + hunk('e2e/tests/x-support.ts', 1, 1), read, files: (prefix) => Object.keys(files).filter((f) => f.startsWith(prefix)) });
  assert.deepEqual(got.playwright.map((t) => t.title), ['a', 'b']);
  assert.equal(reads.filter((f) => f === 'e2e/tests/x.spec.ts').length, 1);
});

test('a test that reaches a changed helper only through a beforeEach is a caller', () => {
  const files = {
    'e2e/tests/x.spec.ts': "test.beforeEach(async ({ page }) => {\n  await openThing(page);\n});\n\ntest('through the hook', async ({ page }) => {\n  await page.goto('/');\n});\n",
    'e2e/tests/x-support.ts': 'export async function openThing(page: Page): Promise<void> {}\n',
  };
  const got = mapDiff({ diff: hunk('e2e/tests/x-support.ts', 1, 1), read: (f) => files[f], files: (prefix) => Object.keys(files).filter((f) => f.startsWith(prefix)) });
  assert.deepEqual(got.playwright.map((t) => [t.title, t.why]), [['through the hook', 'calls openThing']]);
});

test('a change inside a hook maps to the tests the hook reaches, and only those', () => {
  const files = {
    'e2e/tests/x.spec.ts': [
      "test.describe('with a hook', () => {",
      '  test.beforeEach(async ({ page }) => {',
      "    await page.goto('/a');",
      '  });',
      "  test('first', async () => {});",
      "  test('second', async () => {});",
      '});',
      "test.describe('without one', () => {",
      "  test('third', async () => {});",
      '});',
      '',
    ].join('\n'),
  };
  const got = mapDiff({ diff: hunk('e2e/tests/x.spec.ts', 3, 3), read: (f) => files[f], files: () => [] });
  assert.deepEqual(got.playwright.map((t) => [t.title, t.why]), [['first', 'reached by a changed hook'], ['second', 'reached by a changed hook']]);
});

test('a changed hook that reaches many tests maps to a sample of them, with a note, and a test changed itself still runs', () => {
  const tests = Array.from({ length: 12 }, (_, i) => `test('t${String(i).padStart(2, '0')}', async () => {});`);
  const files = { 'e2e/tests/x.spec.ts': ['test.beforeEach(async ({ page }) => {', "  await page.goto('/a');", '});', ...tests, ''].join('\n') };
  const read = (f) => files[f];
  const got = mapDiff({ diff: hunk('e2e/tests/x.spec.ts', 2, 2), read, files: () => [] });
  assert.equal(got.playwright.length, 5);
  assert.ok(got.playwright.every((t) => t.why === 'reached by a changed hook'));
  assert.equal(got.notes.length, 1);
  assert.match(got.notes[0], /hook at lines 1 to 3 changed and reaches 12 tests; 5 run/);
  // A test changed in the same diff is a "changed" target whether or not the sample picks it.
  const last = hunk('e2e/tests/x.spec.ts', 2, 2) + hunk('e2e/tests/x.spec.ts', 15, 15);
  const both = mapDiff({ diff: last, read, files: () => [] });
  assert.equal(both.playwright.find((t) => t.title === 't11').why, 'changed');
});
