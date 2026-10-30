// Tests of e2e-no-retries.sh: node --test .github/scripts/
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const script = path.join(path.dirname(fileURLToPath(import.meta.url)), 'e2e-no-retries.sh');

function check(config, spec = "test('x', async () => {});\n") {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'retries-'));
  fs.mkdirSync(path.join(dir, 'e2e', 'tests'), { recursive: true });
  fs.writeFileSync(path.join(dir, 'e2e', 'playwright.config.ts'), config);
  fs.writeFileSync(path.join(dir, 'e2e', 'tests', 'a.spec.ts'), spec);
  return spawnSync('bash', [script, path.join(dir, 'e2e', 'playwright.config.ts')], { cwd: dir, encoding: 'utf8' }).status;
}

test('retries: 0 passes', () => assert.equal(check('export default {\n  retries: 0,\n};\n'), 0));
test('retries on CI fails', () => assert.equal(check('export default {\n  retries: process.env.CI ? 2 : 0,\n};\n'), 1));
test('no retries line fails, so the guard is explicit', () => assert.equal(check('export default {};\n'), 1));
test('a spec that sets retries fails', () => assert.equal(check('export default {\n  retries: 0,\n};\n', 'test.describe.configure({ retries: 2 });\n'), 1));
test('retries after a "/" on the same line still counts', () =>
  assert.equal(check("export default {\n  retries: 0,\n  use: { baseURL: 'http://x', retries: 2 },\n};\n"), 1));
test('a retries line that is a comment is ignored', () =>
  assert.equal(check('export default {\n  // retries: 3 would hide flakes\n  retries: 0,\n};\n'), 0));
