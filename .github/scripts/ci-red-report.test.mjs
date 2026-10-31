// Tests of ci-red-report.sh with a fake `gh` that logs its calls: node --test .github/scripts/
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const script = path.join(path.dirname(fileURLToPath(import.meta.url)), 'ci-red-report.sh');

// Runs the script with a `gh` whose `issue list` prints `listed` (the number the jq filter would give), and returns the calls.
function report({ listed = '', args = [], env = {} } = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ci-red-'));
  try {
    const log = path.join(dir, 'calls');
    fs.writeFileSync(path.join(dir, 'gh'), `#!/usr/bin/env bash\nprintf '%s\\n' "$*" >>"${log}"\nif [ "$1 $2" = "issue list" ]; then if [ -n "$ISSUES" ]; then for a in "$@"; do case "$prev" in --jq) prog=$a;; esac; prev=$a; done; printf '%s' "$ISSUES" | jq -r "$prog"; else printf '%s' "${listed}"; fi; fi\nif [ "$1 $2" = "issue create" ] || [ "$1 $2" = "issue comment" ]; then for a in "$@"; do case "$prev" in --body-file) cat "$a" >>"${log}.body";; esac; prev=$a; done; fi\n`, { mode: 0o755 });
    const r = spawnSync('bash', [script, ...args], {
      cwd: dir,
      env: { ...process.env, PATH: `${dir}:${process.env.PATH}`, WORKFLOW: 'e2e', SHA: 'abc123', RUN_URL: 'https://example.test/run/1', ...env },
      encoding: 'utf8',
    });
    const read = (f) => (fs.existsSync(f) ? fs.readFileSync(f, 'utf8') : '');
    return { code: r.status, err: r.stderr, calls: read(log).trim().split('\n').filter(Boolean), body: read(`${log}.body`) };
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

test('without an open issue of its title it creates one, titled after the workflow', () => {
  const r = report();
  assert.equal(r.code, 0, r.err);
  assert.ok(r.calls.some((c) => c.startsWith('label create ci-red')), r.calls.join('\n'));
  assert.ok(r.calls.some((c) => c.startsWith('issue create') && c.includes('--title CI vermelho: e2e na main')), r.calls.join('\n'));
  assert.match(r.body, /O workflow e2e falhou na main\.\n\nCommit: abc123\nExecução: https:\/\/example\.test\/run\/1/);
});

test('with an open issue of its title it comments on that one', () => {
  const r = report({ listed: '42' });
  assert.ok(r.calls.some((c) => c.startsWith('issue comment 42')), r.calls.join('\n'));
  assert.ok(!r.calls.some((c) => c.startsWith('issue create')));
});

test('the lookup filters by the exact title, so another ci-red issue is not taken', () => {
  const r = report({ env: { TITLE: 'CI vermelho: caça noturna a testes instáveis' } });
  const list = r.calls.find((c) => c.startsWith('issue list'));
  assert.match(list, /select\(\.title == env\.TITLE\)/);
  assert.match(list, /\[0\]\.number \/\/ empty/);
});

test('a body file is used as it is', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ci-red-body-'));
  const file = path.join(dir, 'body.md');
  fs.writeFileSync(file, 'the nightly body');
  try {
    const r = report({ args: [file], env: { TITLE: 'CI vermelho: nightly' } });
    assert.equal(r.body, 'the nightly body');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('it needs the workflow, the commit and the run', () => {
  for (const name of ['WORKFLOW', 'SHA', 'RUN_URL']) assert.notEqual(report({ env: { [name]: '' } }).code, 0, name);
});

test('a title with a quote or a backslash still finds its issue', () => {
  const title = 'CI vermelho: "e2e" \\ na main';
  const issues = JSON.stringify([
    { number: 7, title: 'CI vermelho: outro' },
    { number: 9, title },
  ]);
  const r = report({ env: { TITLE: title, ISSUES: issues } });
  assert.equal(r.code, 0, r.err);
  assert.ok(r.calls.some((c) => c.startsWith('issue comment 9')), r.calls.join('\n'));
});
