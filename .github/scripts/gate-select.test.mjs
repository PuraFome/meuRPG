// Tests of gate-select.sh in a throwaway git repository: node --test .github/scripts/
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const script = path.join(path.dirname(fileURLToPath(import.meta.url)), 'gate-select.sh');

// A repository whose origin/main has the `base` files and whose head adds `files` and moves `moves` ([from, to]).
// The repository is removed when the call returns.
function select(env, files, ...args) {
  return selectIn(env, { files }, ...args);
}

function selectIn(env, { base = [], files = [], moves = [] }, ...args) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gate-'));
  try {
    const git = (...a) => spawnSync('git', a, { cwd: dir, encoding: 'utf8' });
    const write = (f) => {
      fs.mkdirSync(path.dirname(path.join(dir, f)), { recursive: true });
      fs.writeFileSync(path.join(dir, f), 'y');
    };
    git('init', '-q', '-b', 'main');
    git('config', 'user.email', 'a@b.c');
    git('config', 'user.name', 'n');
    fs.writeFileSync(path.join(dir, 'base.txt'), 'x');
    base.forEach(write);
    git('add', '.');
    git('commit', '-qm', 'base');
    git('update-ref', 'refs/remotes/origin/main', 'HEAD');
    files.forEach(write);
    for (const [from, to] of moves) {
      fs.mkdirSync(path.dirname(path.join(dir, to)), { recursive: true });
      git('mv', from, to);
    }
    git('add', '.');
    git('commit', '-qm', 'head', '--allow-empty');
    const out = path.join(dir, 'out');
    const r = spawnSync('bash', [script, ...args], { cwd: dir, env: { ...process.env, GITHUB_OUTPUT: out, BASE_REF: 'main', ...env }, encoding: 'utf8' });
    return { code: r.status, out: fs.existsSync(out) ? fs.readFileSync(out, 'utf8') : '', err: r.stderr };
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

const pr = {};

test('a pull request is selected only by a matching file', () => {
  assert.match(select(pr, ['backend/a.go'], 'run', '^backend/').out, /run=true/);
  assert.match(select(pr, ['docs/a.txt'], 'run', '^backend/').out, /run=false/);
});

test('--ignore-md drops markdown files from the match', () => {
  assert.match(select(pr, ['web/readme.md'], 'run', '^web/', '--ignore-md').out, /run=false/);
  assert.match(select(pr, ['web/readme.md', 'web/a.ts'], 'run', '^web/', '--ignore-md').out, /run=true/);
});

test('a non-ASCII path is seen', () => {
  assert.match(select(pr, ['web/ação.ts'], 'run', '^web/').out, /run=true/);
});

test('a file moved out of the selected tree selects it: the old path counts too', () => {
  const got = selectIn(pr, { base: ['web/x.ts'], moves: [['web/x.ts', 'docs/x.ts']] }, 'run', '^web/');
  assert.match(got.out, /run=true/, got.err);
});

test('it needs the base branch', () => {
  assert.notEqual(select({ ...pr, BASE_REF: '' }, ['web/a.ts'], 'run', '^web/').code, 0);
});
