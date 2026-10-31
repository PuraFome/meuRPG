#!/usr/bin/env node
// Splits the Playwright tests across the e2e shards by measured time (docs/ci.md, "End to end").
//
//   e2e-shards.mjs list <index>/<count> [playwright args]   the --test-list file of one shard, on stdout
//   e2e-shards.mjs check <count> [playwright args]          fails unless every test is in exactly one of the shards
//   e2e-shards.mjs timings <report.json>...                 the timing file, from Playwright JSON reports, on stdout
//
// The playwright args (a --grep, a --grep-invert) select the tests to split, as they select the tests to run.
// The timing file is e2e/timings.json: one duration in seconds per test, keyed by the line of the test list.
import { spawnSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const SEPARATOR = ' › ';

// The key of a test: the line Playwright's --test-list reads, which names the project, the file and the titles.
export function testKey(project, file, titles) {
  return [`[${project}]`, file, ...titles].join(SEPARATOR);
}

// The tests of `playwright test --list --reporter=json` or of a run's JSON report, in file order.
export function testsOf(report) {
  const found = [];
  const walk = (suite, titles) => {
    for (const spec of suite.specs ?? []) {
      for (const test of spec.tests ?? []) {
        found.push({ key: testKey(test.projectName, spec.file, [...titles, spec.title]), results: test.results ?? [] });
      }
    }
    for (const child of suite.suites ?? []) walk(child, [...titles, child.title]);
  };
  for (const file of report.suites ?? []) walk(file, []);
  return found;
}

function median(values) {
  const sorted = [...values].sort((a, b) => a - b);
  return sorted.length ? sorted[Math.floor(sorted.length / 2)] : 1;
}

// Puts every key in exactly one of `count` shards. The longest known tests go first, each to the shard with the least
// time so far; a test the timing file does not know comes after them with the median duration, to the shard with the
// least time too, so a new test runs without anyone updating the file. The setup project runs as a dependency of
// every shard, so it weighs nothing here. Returns the keys of each shard, in the order given.
export function split(keys, timings, count) {
  const isSetup = (key) => key.startsWith('[setup]');
  const known = keys.filter((key) => !isSetup(key) && key in timings);
  const unknown = keys.filter((key) => !isSetup(key) && !(key in timings));
  const guess = median(known.map((key) => timings[key]));
  const load = new Array(count).fill(0);
  const shardOf = new Map();
  const place = (key, seconds) => {
    let least = 0;
    for (let i = 1; i < count; i++) if (load[i] < load[least]) least = i;
    load[least] += seconds;
    shardOf.set(key, least);
  };
  // The sort is stable and the ties keep the order of `keys`, so every shard computes the same split.
  for (const key of [...known].sort((a, b) => timings[b] - timings[a])) place(key, timings[key]);
  for (const key of unknown) place(key, guess);
  for (const key of keys.filter(isSetup)) shardOf.set(key, 0);
  const shards = Array.from({ length: count }, () => []);
  for (const key of keys) shards[shardOf.get(key)].push(key);
  return shards;
}

// Durations in seconds of the passed tests of Playwright JSON reports; a test that several reports hold keeps its longest.
export function timingsOf(reports) {
  const seconds = {};
  for (const report of reports) {
    for (const { key, results } of testsOf(report)) {
      const passed = results.filter((result) => result.status === 'passed');
      if (passed.length === 0) continue;
      const longest = Math.max(...passed.map((result) => result.duration)) / 1000;
      seconds[key] = Math.max(seconds[key] ?? 0, Math.round(longest * 10) / 10);
    }
  }
  return Object.fromEntries(Object.entries(seconds).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)));
}

const e2eDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'e2e');

function listTests(args, extra = []) {
  // With a report file named in the environment the json reporter writes there and prints nothing.
  const { PLAYWRIGHT_JSON_OUTPUT_NAME: _unused, ...env } = process.env;
  const run = spawnSync('npx', ['playwright', 'test', '--list', '--reporter=json', ...args, ...extra], {
    cwd: e2eDir,
    env,
    encoding: 'utf8',
    maxBuffer: 1 << 28,
  });
  if (run.status !== 0) throw new Error(`playwright --list failed (${run.status}):\n${run.stderr}`);
  return testsOf(JSON.parse(run.stdout)).map((test) => test.key);
}

function readTimings() {
  return JSON.parse(readFileSync(join(e2eDir, 'timings.json'), 'utf8'));
}

function shardsOf(args, count) {
  const keys = listTests(args);
  const timings = readTimings();
  const missing = keys.filter((key) => !key.startsWith('[setup]') && !(key in timings)).length;
  const stale = Object.keys(timings).filter((key) => !keys.includes(key)).length;
  return { keys, missing, stale, shards: split(keys, timings, count) };
}

function parseShard(text) {
  const match = /^(\d+)\/(\d+)$/.exec(text ?? '');
  if (!match || Number(match[1]) < 1 || Number(match[1]) > Number(match[2])) throw new Error(`the shard must look like 1/3, got ${text}`);
  return { index: Number(match[1]), count: Number(match[2]) };
}

function main(argv) {
  const [command, ...rest] = argv;
  if (command === 'list') {
    const { index, count } = parseShard(rest[0]);
    const { keys, missing, stale, shards } = shardsOf(rest.slice(1), count);
    if (missing > 0) console.error(`::notice::${missing} of ${keys.length} tests are not in e2e/timings.json; they ran in the shard with the least time. Refresh the file (docs/ci.md).`);
    if (stale > 0) console.error(`${stale} entries of e2e/timings.json name no test any more.`);
    process.stdout.write(shards[index - 1].join('\n') + '\n');
  } else if (command === 'check') {
    const count = Number(rest[0]);
    const args = rest.slice(1);
    const { keys, shards } = shardsOf(args, count);
    // Playwright, not this script, decides what a list selects, so each list is read back through `--test-list`.
    // Without --no-deps every shard would list the setup project, which runs ahead of all of them.
    const seen = new Map();
    shards.forEach((shard, i) => {
      const file = join(process.env.RUNNER_TEMP ?? '/tmp', `e2e-shard-${process.pid}-${i + 1}.txt`);
      writeFileSync(file, shard.join('\n') + '\n');
      for (const key of listTests(args, ['--no-deps', '--test-list', file])) {
        if (!key.startsWith('[setup]')) seen.set(key, [...(seen.get(key) ?? []), i + 1]);
      }
      // The setup project is never listed back: it runs ahead of every shard whatever the list says.
      for (const key of shard) if (key.startsWith('[setup]')) seen.set(key, [...(seen.get(key) ?? []), i + 1]);
    });
    const problems = keys.filter((key) => (seen.get(key) ?? []).length !== 1).map((key) => `${key}: in shards [${(seen.get(key) ?? []).join(', ')}]`);
    const unknown = [...seen.keys()].filter((key) => !keys.includes(key));
    if (problems.length > 0 || unknown.length > 0) {
      console.error(`::error::The ${count} lists do not hold every test exactly once.`);
      for (const line of [...problems, ...unknown.map((key) => `${key}: not a listed test`)]) console.error(line);
      process.exit(1);
    }
    console.log(`${keys.length} tests, each in exactly one of ${count} lists.`);
  } else if (command === 'timings') {
    process.stdout.write(JSON.stringify(timingsOf(rest.map((file) => JSON.parse(readFileSync(file, 'utf8')))), null, 1) + '\n');
  } else {
    console.error('usage: e2e-shards.mjs list <index>/<count> [args] | check <count> [args] | timings <report.json>...');
    process.exit(2);
  }
}

if (process.argv[1] === fileURLToPath(import.meta.url)) main(process.argv.slice(2));
