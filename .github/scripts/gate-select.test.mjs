// Tests of gate-select.sh in a throwaway git repository: node --test .github/scripts/
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const script = path.join(path.dirname(fileURLToPath(import.meta.url)), 'gate-select.sh');

// A repository whose origin/main has one commit and whose head adds `files`.
function select(env, files, ...args) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gate-'));
  const git = (...a) => spawnSync('git', a, { cwd: dir, encoding: 'utf8' });
  git('init', '-q', '-b', 'main');
  git('config', 'user.email', 'a@b.c');
  git('config', 'user.name', 'n');
  fs.writeFileSync(path.join(dir, 'base.txt'), 'x');
  git('add', '.');
  git('commit', '-qm', 'base');
  git('update-ref', 'refs/remotes/origin/main', 'HEAD');
  for (const f of files) {
    fs.mkdirSync(path.dirname(path.join(dir, f)), { recursive: true });
    fs.writeFileSync(path.join(dir, f), 'y');
  }
  git('add', '.');
  git('commit', '-qm', 'head', '--allow-empty');
  const out = path.join(dir, 'out');
  const r = spawnSync('bash', [script, ...args], { cwd: dir, env: { ...process.env, GITHUB_OUTPUT: out, BASE_REF: 'main', ...env }, encoding: 'utf8' });
  return { code: r.status, out: fs.existsSync(out) ? fs.readFileSync(out, 'utf8') : '', err: r.stderr };
}

const pr = { EVENT: 'pull_request', HEAD_REF: 'feat/x' };

test('a push, a schedule or a dispatch is always selected', () => {
  for (const EVENT of ['push', 'schedule', 'workflow_dispatch']) {
    assert.match(select({ EVENT, HEAD_REF: '' }, ['docs/a.md'], 'run', '^backend/').out, /run=true\ntrain=false/);
  }
});

test('a pull request is selected only by a matching file', () => {
  assert.match(select(pr, ['backend/a.go'], 'run', '^backend/').out, /run=true\ntrain=false/);
  assert.match(select(pr, ['docs/a.txt'], 'run', '^backend/').out, /run=false\ntrain=false/);
});

test('--ignore-md drops markdown files from the match', () => {
  assert.match(select(pr, ['web/readme.md'], 'run', '^web/', '--ignore-md').out, /run=false/);
  assert.match(select(pr, ['web/readme.md', 'web/a.ts'], 'run', '^web/', '--ignore-md').out, /run=true/);
});

test('a train is selected whatever it changes, docs-only included', () => {
  const r = select({ EVENT: 'pull_request', HEAD_REF: 'refactor/train-69' }, ['docs/a.md'], 'run', '^backend/');
  assert.match(r.out, /run=true\ntrain=true/);
});

test('a branch that only looks like a train is not one', () => {
  assert.match(select({ EVENT: 'pull_request', HEAD_REF: 'feat/refactor/train-1' }, ['docs/a.md'], 'run', '^backend/').out, /run=false\ntrain=false/);
});
