import assert from 'node:assert/strict';
import { test } from 'node:test';

import { split, testKey, testsOf, timingsOf } from './e2e-shards.mjs';

// Invented tests: a file, a title and how long it takes.
const key = (file, title) => testKey('chrome', file, [title]);
const setup = testKey('setup', 'auth.setup.ts', ['sign in']);

function suite(...specs) {
  return { suites: specs.map(([file, title, status, duration]) => ({ title: file, specs: [{ file, title, tests: [{ projectName: 'chrome', results: [{ status, duration }] }] }] })) };
}

test('every test lands in exactly one shard, the setup project included', () => {
  const keys = [setup, ...Array.from({ length: 20 }, (_, i) => key(`f${i}.spec.ts`, `test ${i}`))];
  const timings = Object.fromEntries(keys.slice(1).map((k, i) => [k, i + 1]));
  for (const count of [1, 3, 6]) {
    const shards = split(keys, timings, count);
    assert.equal(shards.length, count);
    assert.deepEqual(shards.flat().sort(), [...keys].sort());
  }
});

test('the shards are balanced by time, not by count', () => {
  // One test as long as ten short ones: a split by count would put five tests in each shard.
  const long = key('slow.spec.ts', 'slow');
  const short = Array.from({ length: 10 }, (_, i) => key('fast.spec.ts', `fast ${i}`));
  const timings = { [long]: 10, ...Object.fromEntries(short.map((k) => [k, 1])) };
  const [first, second] = split([long, ...short], timings, 2);
  const total = (shard) => shard.reduce((sum, k) => sum + timings[k], 0);
  assert.equal(total(first), 10);
  assert.equal(total(second), 10);
  assert.deepEqual(first, [long]);
});

test('a test the timing file does not know still runs, in the shard with the least time', () => {
  const heavy = key('a.spec.ts', 'heavy');
  const light = key('a.spec.ts', 'light');
  const fresh = key('b.spec.ts', 'new');
  const [first, second] = split([heavy, light, fresh], { [heavy]: 30, [light]: 10 }, 2);
  assert.deepEqual(first, [heavy]);
  assert.deepEqual(second, [light, fresh]);
});

test('unknown tests spread over the shards instead of piling up in one', () => {
  const keys = Array.from({ length: 6 }, (_, i) => key('new.spec.ts', `new ${i}`));
  const shards = split(keys, {}, 3);
  assert.deepEqual(shards.map((shard) => shard.length), [2, 2, 2]);
});

test('a subset is split on its own: only its tests are listed and they are balanced', () => {
  const all = Array.from({ length: 12 }, (_, i) => key(`f${i}.spec.ts`, `test ${i}`));
  const timings = Object.fromEntries(all.map((k, i) => [k, i % 2 === 0 ? 8 : 1]));
  const slow = all.filter((_, i) => i % 2 === 0);
  const shards = split(slow, timings, 3);
  assert.deepEqual(shards.flat().sort(), [...slow].sort());
  assert.deepEqual(shards.map((shard) => shard.length), [2, 2, 2]);
});

test('the split does not depend on who computes it', () => {
  const keys = Array.from({ length: 30 }, (_, i) => key(`f${i % 7}.spec.ts`, `test ${i}`));
  const timings = Object.fromEntries(keys.filter((_, i) => i % 3).map((k, i) => [k, (i * 7) % 5]));
  assert.deepEqual(split(keys, timings, 4), split([...keys], { ...timings }, 4));
});

test('a timing file with stale entries and no tests at all still splits', () => {
  assert.deepEqual(split([], { [key('gone.spec.ts', 'gone')]: 5 }, 2), [[], []]);
});

test('testsOf names a test by project, file and the titles of its describes', () => {
  const report = {
    suites: [
      {
        title: 'a.spec.ts',
        specs: [{ file: 'a.spec.ts', title: 'alone', tests: [{ projectName: 'chrome' }] }],
        suites: [{ title: 'group', specs: [{ file: 'a.spec.ts', title: 'inside', tests: [{ projectName: 'chrome' }] }] }],
      },
    ],
  };
  assert.deepEqual(testsOf(report).map((t) => t.key), ['[chrome] › a.spec.ts › alone', '[chrome] › a.spec.ts › group › inside']);
});

test('timingsOf keeps the longest passed run in seconds and drops the tests that did not pass', () => {
  const timings = timingsOf([suite(['a.spec.ts', 'one', 'passed', 1234], ['b.spec.ts', 'two', 'failed', 9000]), suite(['a.spec.ts', 'one', 'passed', 2051])]);
  assert.deepEqual(timings, { '[chrome] › a.spec.ts › one': 2.1 });
});
