// @ts-check
// The sass of e2e/ is the version the web app builds with, so the stylesheets the layout fixtures compile are the ones the app ships.
// A version bump that reaches one lock file and not the other fails here. Run with `npm run lint:test` from e2e/.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

/**
 * @param {string | undefined} pinned the version in e2e/package.json
 * @param {string | undefined} built the version in web/package-lock.json
 * @returns {string | null} what differs, or null
 */
function sassDrift(pinned, built) {
  if (!pinned) return 'e2e/package.json does not pin sass';
  if (!built) return 'web/package-lock.json does not hold sass';
  if (pinned !== built) return `e2e/package.json pins sass ${pinned} and web/package-lock.json holds ${built}: bump them together`;
  return null;
}

test('the drift names the two versions when they differ', () => {
  assert.equal(sassDrift('1.97.3', '1.97.3'), null);
  assert.match(String(sassDrift('1.97.2', '1.97.3')), /pins sass 1\.97\.2 .* holds 1\.97\.3/);
  assert.match(String(sassDrift(undefined, '1.97.3')), /does not pin sass/);
  assert.match(String(sassDrift('1.97.3', undefined)), /does not hold sass/);
});

test('e2e pins the sass version the web app builds with', () => {
  const root = path.resolve(__dirname, '..', '..', '..');
  const e2e = JSON.parse(fs.readFileSync(path.join(root, 'e2e', 'package.json'), 'utf8'));
  const lock = JSON.parse(fs.readFileSync(path.join(root, 'web', 'package-lock.json'), 'utf8'));
  assert.equal(sassDrift(e2e.devDependencies?.sass, lock.packages?.['node_modules/sass']?.version), null);
});
