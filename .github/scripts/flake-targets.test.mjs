// Unit tests of flake-targets.mjs: node --test .github/scripts/
import assert from 'node:assert/strict';
import test from 'node:test';

import { changedLines, mapDiff, playwrightTests, sample, titleOf } from './flake-targets.mjs';

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

export const other = 2;
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
  const diff = '--- a/f\n+++ b/f\n@@ -3 +3 @@\n-a\n+b\n@@ -10,2 +10,0 @@\n-x\n-y\n@@ -20,0 +21,3 @@\n+1\n+2\n+3\n';
  assert.deepEqual(changedLines(diff), { f: [[3, 3], [10, 11], [21, 23]] });
});

test('titleOf escapes a title and turns an interpolation into a wildcard', () => {
  assert.deepEqual(titleOf("  test('first (1.5) test', async () => {"), { title: 'first (1.5) test', regex: 'first \\(1\\.5\\) test' });
  assert.equal(titleOf('  test(`tema ${theme} passa`, async () => {').regex, 'tema .* passa');
  assert.equal(titleOf('  test.describe(\'a group\', () => {'), null);
  assert.equal(titleOf("  await expect(page).toHaveURL('/x');"), null);
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
  assert.deepEqual(got.playwright, [{ file: 'e2e/tests/a.spec.ts', title: 'second test', grep: 'second test', why: 'changed' }]);
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
  assert.deepEqual(none.playwright, []); // `other` is exported and nobody calls it
  assert.match(none.notes[0], /no test calls it/);
  assert.deepEqual(map(hunk('e2e/tests/thing-support.ts', 9, 9)), { playwright: [], go: [], notes: [] }); // not exported
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
