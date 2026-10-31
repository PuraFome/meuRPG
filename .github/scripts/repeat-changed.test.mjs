// Tests of repeat-changed.sh with a fake `npx` and `go` on the PATH: node --test .github/scripts/
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const script = path.join(path.dirname(fileURLToPath(import.meta.url)), 'repeat-changed.sh');

// The fake tool looks at its arguments: a regex with FAIL fails, one with OOM exits 137 at once (a kill that is not the time limit), one with SLOW outlasts the budget, the rest pass.
function run(kind, targets, env = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'repeat-'));
  const shim = '#!/usr/bin/env bash\nargs="$*"\necho "$args" >> "$(dirname "$0")/calls.log"\ncase "$args" in *FAIL*) exit 1 ;; *OOM*) exit 137 ;; *SLOW*) sleep 30 ;; esac\nexit 0\n';
  for (const name of ['npx', 'go']) {
    fs.writeFileSync(path.join(dir, name), shim, { mode: 0o755 });
  }
  const file = path.join(dir, 'targets.json');
  fs.writeFileSync(file, JSON.stringify(targets));
  const summary = path.join(dir, 'summary.md');
  const r = spawnSync('bash', [script, kind, file], {
    env: { ...process.env, PATH: `${dir}:${process.env.PATH}`, GITHUB_STEP_SUMMARY: summary, MIN_SECONDS: '1', ...env },
    encoding: 'utf8',
  });
  const calls = fs.existsSync(path.join(dir, 'calls.log')) ? fs.readFileSync(path.join(dir, 'calls.log'), 'utf8') : '';
  return { calls, code: r.status, summary: fs.existsSync(summary) ? fs.readFileSync(summary, 'utf8') : '', out: r.stdout + r.stderr };
}

const pw = (...titles) => ({ playwright: titles.map((t) => ({ file: 'e2e/tests/a.spec.ts', title: t, grep: t, why: 'changed' })), go: [], notes: [] });

test('every target runs in both colour schemes and the job passes when none fails', () => {
  const r = run('playwright', pw('one', 'two'), { BUDGET_SECONDS: '60' });
  assert.equal(r.code, 0, r.out);
  assert.match(r.summary, /Passed: 4/);
  assert.match(r.summary, /Failed: 0/);
});

test('a failing target fails the job after the others ran', () => {
  const r = run('playwright', pw('one', 'FAIL two', 'three'), { BUDGET_SECONDS: '60' });
  assert.equal(r.code, 1, r.out);
  assert.match(r.summary, /Failed: 2/); // both schemes
  assert.match(r.summary, /Passed: 4/);
  assert.match(r.summary, /FAILED: FAIL two \(light\)/);
});

test('running out of time passes, and the summary says what was cut off and what did not run', () => {
  const r = run('playwright', pw('SLOW one'), { BUDGET_SECONDS: '2' });
  assert.equal(r.code, 0, r.out);
  assert.match(r.summary, /cut off: SLOW one \(light\)/);
  assert.match(r.summary, /Not run, the budget was spent: [1-9]/);
});

test('Go targets run once per package, with the tests joined in the regex', () => {
  const targets = {
    playwright: [],
    go: [
      { pkg: './internal/a', test: 'TestOne', why: 'changed' },
      { pkg: './internal/a', test: 'TestTwo', why: 'changed' },
      { pkg: './internal/b', test: 'TestFAIL', why: 'changed' },
    ],
    notes: [],
  };
  const r = run('go', targets, { BUDGET_SECONDS: '60' });
  assert.equal(r.code, 1, r.out);
  assert.match(r.summary, /Passed: 1/);
  assert.match(r.summary, /FAILED: \.\/internal\/b: TestFAIL/);
});

test('the regex of a title reaches Playwright with its backslashes, and the spec file scopes it', () => {
  const targets = { playwright: [{ file: 'e2e/tests/a.spec.ts', title: 'first (1.5) test', grep: 'first \\(1\\.5\\) test', why: 'changed' }], go: [], notes: [] };
  const r = run('playwright', targets, { BUDGET_SECONDS: '60' });
  assert.equal(r.code, 0, r.out);
  assert.ok(r.calls.includes('playwright test tests/a.spec.ts --grep first \\(1\\.5\\) test --repeat-each=10'), r.calls);
});

// /auth/login: 40 requests at once per address; a sign-in is 2 and the setup of each run makes 3.
test('a test that signs in a lot repeats in batches, the stack reset before each but the first', () => {
  const targets = { playwright: [{ file: 'e2e/tests/a.spec.ts', title: 'three', grep: 'three', signins: 3, why: 'changed' }], go: [], notes: [] };
  const r = run('playwright', targets, { BUDGET_SECONDS: '60', RESET_STACK_CMD: 'echo reset >> "$(dirname "$(command -v npx)")/calls.log"' });
  assert.equal(r.code, 0, r.out);
  const lines = r.calls.trim().split('\n');
  // 6 + 6 x 10 = 66 > 40: batches of (40 - 6) / 6 = 5, so two per colour scheme: 4 runs, 3 resets (none before the first).
  assert.equal(lines.filter((l) => l.startsWith('playwright test')).length, 4);
  assert.ok(lines.every((l) => !l.startsWith('playwright test') || l.includes('--repeat-each=5')), r.calls);
  assert.equal(lines.filter((l) => l === 'reset').length, 3);
});

test('a test that signs in once repeats in one run: 6 + 2 x 10 fits the limit', () => {
  const targets = { playwright: [{ file: 'e2e/tests/a.spec.ts', title: 'one', grep: 'one', signins: 1, why: 'changed' }], go: [], notes: [] };
  const r = run('playwright', targets, { BUDGET_SECONDS: '60' });
  assert.equal(r.code, 0, r.out);
  assert.equal(r.calls.trim().split('\n').filter((l) => l.includes('--repeat-each=10')).length, 2);
});

test('the stack is reset only when the sign-ins would not fit: six plain chunks need none', () => {
  const r = run('playwright', pw('one', 'two', 'three'), { BUDGET_SECONDS: '60', RESET_STACK_CMD: 'echo reset >> "$(dirname "$(command -v npx)")/calls.log"' });
  assert.equal(r.code, 0, r.out);
  assert.equal(r.calls.split('\n').filter((l) => l === 'reset').length, 0);
});

test('an exit 137 before the time limit (an out-of-memory kill) is a failure, not "cut off"', () => {
  const r = run('playwright', pw('OOM one'), { BUDGET_SECONDS: '60' });
  assert.notEqual(r.code, 0, r.out);
  assert.match(r.summary, /Failed: 2/);
  assert.match(r.summary, /Cut off by the time budget, no failure before it: 0/);
});
