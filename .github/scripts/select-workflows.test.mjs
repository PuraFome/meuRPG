// The workflows that choose what runs state the train rule in one expression (DIFF), copied into each of them because a
// local composite action needs a checkout first: node --test .github/scripts/
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

// The rule (docs/ci.md, "What runs when"): only a pull request that is not a train, from a branch of this repository named
// refactor/train-*, needs the diff.
const DIFF = "${{ github.event_name == 'pull_request' && !(startsWith(github.head_ref, 'refactor/train-') && github.event.pull_request.head.repo.full_name == github.repository) }}";
const SELECTING = ['backend', 'codeql', 'e2e', 'osv-scan', 'web', 'workflows-lint'];

const workflows = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'workflows');

test('each selecting workflow has the DIFF expression, exactly, and no other workflow has one', () => {
  for (const name of fs.readdirSync(workflows).filter((f) => f.endsWith('.yml'))) {
    const found = [...fs.readFileSync(path.join(workflows, name), 'utf8').matchAll(/^ {6}DIFF: (.+)$/gm)].map((m) => m[1]);
    const workflow = name.replace(/\.yml$/, '');
    assert.deepEqual(found, SELECTING.includes(workflow) ? [DIFF] : [], name);
  }
});
