// Tests of repeat-changed.sh with a fake `npx` and `go` on the PATH: node --test .github/scripts/
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const script = path.join(path.dirname(fileURLToPath(import.meta.url)), 'repeat-changed.sh');

// The fake tool looks at its arguments: a regex with FAIL fails, one with SLOW outlasts the budget, the rest pass.
function run(kind, targets, env = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'repeat-'));
  const shim = '#!/usr/bin/env bash\nargs="$*"\ncase "$args" in *FAIL*) exit 1 ;; *SLOW*) sleep 30 ;; esac\nexit 0\n';
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
  return { code: r.status, summary: fs.existsSync(summary) ? fs.readFileSync(summary, 'utf8') : '', out: r.stdout + r.stderr };
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
