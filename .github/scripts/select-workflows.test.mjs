// The workflows that choose what runs state the train rule in an expression (DIFF); it must be the same in all of them
// and agree with gate-select.sh: node --test .github/scripts/
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const root = path.join(path.dirname(fileURLToPath(import.meta.url)), '..');
const expressions = fs
  .readdirSync(path.join(root, 'workflows'))
  .filter((f) => f.endsWith('.yml'))
  .flatMap((f) => [...fs.readFileSync(path.join(root, 'workflows', f), 'utf8').matchAll(/^ {6}DIFF: (.+)$/gm)].map((m) => ({ f, expression: m[1] })));

test('every selecting workflow has the same DIFF expression', () => {
  assert.ok(expressions.length >= 6, JSON.stringify(expressions.map((e) => e.f)));
  assert.equal(new Set(expressions.map((e) => e.expression)).size, 1, JSON.stringify(expressions));
});

test('the DIFF expression and gate-select.sh state the same train rule', () => {
  const [{ expression }] = expressions;
  assert.match(expression, /github\.event_name == 'pull_request'/);
  assert.match(expression, /startsWith\(github\.head_ref, 'refactor\/train-'\)/);
  assert.match(expression, /github\.event\.pull_request\.head\.repo\.full_name == github\.repository/);
  const script = fs.readFileSync(path.join(root, 'scripts', 'gate-select.sh'), 'utf8');
  assert.match(script, /HEAD_REF:-\}" == refactor\/train-\*/);
  assert.match(script, /HEAD_REPO:-\}" == "\$BASE_REPO"/);
});
