// Unit tests of flake.mjs: node --test .github/scripts/
import assert from 'node:assert/strict';
import test from 'node:test';

import { aggregate, markdown, parseGo, parsePlaywright, signatureOf, verdict } from './flake.mjs';

const pw = (tests) => ({ suites: [{ title: 'f.spec.ts', specs: [], suites: [{ title: 'd', specs: tests.map(([title, ...runs]) => ({ title, tests: runs })) }] }] });
const run = (status, message, projectName = 'chrome') => ({ projectName, status, results: [{ status: status === 'expected' ? 'passed' : 'failed', error: message ? { message } : undefined }] });

test('signatureOf takes the first line and drops what changes between runs', () => {
  const sig = signatureOf('\u001b[31mError:\u001b[39m expect(locator).toBeVisible() 3f2b8c1e-aaaa-bbbb-cccc-1234567890ab after 10000ms\n  more');
  assert.equal(sig, 'Error: expect(locator).toBeVisible() <id> after 10000ms'.replace('10000ms', '<time>'));
  assert.equal(signatureOf(''), '(no message)');
  assert.equal(signatureOf('\n\n  at 0xc000123456 x'), 'at <addr> x');
});

test('parsePlaywright counts the repetitions of the chrome project and keeps the first error line of each failure', () => {
  const report = pw([
    ['the test', run('expected'), run('unexpected', 'Error: timed out 5000ms\nstack'), run('expected'), run('unexpected', 'Error: timed out 7000ms\nstack')],
    ['sign in', run('expected', undefined, 'setup')],
    ['skipped', run('skipped')],
  ]);
  const got = parsePlaywright(report);
  assert.equal(got.passed, 2);
  assert.equal(got.failed, 2);
  assert.deepEqual(
    got.failures.map((f) => f.signature),
    ['Error: timed out <time>', 'Error: timed out <time>'],
  );
});

test('parsePlaywright counts an error of the run itself as a failure', () => {
  const got = parsePlaywright({ suites: [], errors: [{ message: 'the stack was not ready' }] });
  assert.equal(got.failed, 1);
  assert.equal(got.failures[0].test, '(the run)');
});

const ev = (o) => JSON.stringify(o);

test('parseGo counts top-level tests and names the assertion that failed', () => {
  const lines = [
    ev({ Action: 'run', Package: 'p', Test: 'TestA' }),
    ev({ Action: 'output', Package: 'p', Test: 'TestA', Output: '=== RUN   TestA\n' }),
    ev({ Action: 'pass', Package: 'p', Test: 'TestA' }),
    ev({ Action: 'output', Package: 'p', Test: 'TestB', Output: '=== RUN   TestB\n' }),
    ev({ Action: 'output', Package: 'p', Test: 'TestB/sub', Output: '    b_test.go:12: want 1, got 2\n' }),
    ev({ Action: 'fail', Package: 'p', Test: 'TestB/sub' }),
    ev({ Action: 'fail', Package: 'p', Test: 'TestB' }),
    ev({ Action: 'fail', Package: 'p' }),
    'not json',
  ].join('\n');
  const got = parseGo(lines);
  assert.equal(got.passed, 1);
  assert.equal(got.failed, 1);
  assert.deepEqual(got.failures, [{ test: 'TestB', signature: 'b_test.go:12: want 1, got 2' }]);
});

test('parseGo prefers a data race or a panic over the harness lines, and counts a package that fails with no test', () => {
  const race = [
    ev({ Action: 'output', Package: 'p', Test: 'TestR', Output: '=== RUN   TestR\n' }),
    ev({ Action: 'output', Package: 'p', Test: 'TestR', Output: 'WARNING: DATA RACE\n' }),
    ev({ Action: 'fail', Package: 'p', Test: 'TestR' }),
    ev({ Action: 'fail', Package: 'p' }),
  ].join('\n');
  assert.equal(parseGo(race).failures[0].signature, 'WARNING: DATA RACE');

  const panic = [
    ev({ Action: 'output', Package: 'q', Output: 'panic: runtime error: nil pointer dereference [recovered]\n' }),
    ev({ Action: 'fail', Package: 'q' }),
  ].join('\n');
  const got = parseGo(panic);
  assert.equal(got.failed, 1);
  assert.equal(got.failures[0].test, '(package q)');
  assert.match(got.failures[0].signature, /^panic: runtime error/);
});

const side = (name, passed, failed, sig = 'boom') => ({ side: name, runner: 1, passed, failed, missing: false, failures: Array.from({ length: failed }, () => ({ test: 'T', signature: sig })) });

test('aggregate adds the runners of a side and groups the signatures', () => {
  const sides = aggregate([side('head', 5, 1), side('head', 6, 0), side('head', 4, 2, 'other'), side('base', 15, 0)]);
  assert.equal(sides.head.passed, 15);
  assert.equal(sides.head.failed, 3);
  assert.equal(sides.head.runners, 3);
  assert.deepEqual(
    sides.head.signatures.map((s) => [s.signature, s.count]),
    [['other', 2], ['boom', 1]],
  );
});

test('the verdict says to dispatch again with runs=100 when the head fails more often', () => {
  const v = verdict(aggregate([side('head', 27, 3), side('base', 30, 0)]));
  assert.equal(v.kind, 'worse');
  assert.match(v.text, /runs=100/);
});

test('the verdict: base fails as often is "not worse"; none is "clean"; an empty side is "no-runs"', () => {
  assert.equal(verdict(aggregate([side('head', 27, 3), side('base', 27, 3)])).kind, 'not-worse');
  assert.equal(verdict(aggregate([side('head', 27, 3), side('base', 24, 6)])).kind, 'not-worse');
  assert.equal(verdict(aggregate([side('head', 30, 0), side('base', 30, 0)])).kind, 'clean');
  assert.equal(verdict(aggregate([side('head', 30, 0), side('base', 0, 0)])).kind, 'no-runs');
  assert.equal(verdict(aggregate([side('head', 30, 0)])).kind, 'incomplete');
});

test('the summary has both sides, the rates and the signatures', () => {
  const md = markdown(aggregate([side('head', 27, 3), side('base', 30, 0)]), { kind: 'go', test: 'TestA', head: 'feat/x', base: 'main', runs: 30 });
  assert.match(md, /\| head \| 30 \| 27 \| 3 \| 10\.0% \|/);
  assert.match(md, /\| base \| 30 \| 30 \| 0 \| 0\.0% \|/);
  assert.match(md, /3× `boom`/);
  assert.match(md, /runs=100/);
});

test('parseGo reads each run of a test that -count repeats on its own', () => {
  const once = (n, fails) => [
    ev({ Action: 'run', Package: 'p', Test: 'TestA' }),
    ev({ Action: 'run', Package: 'p', Test: 'TestA/sub' }),
    ...(fails ? [ev({ Action: 'output', Package: 'p', Test: 'TestA/sub', Output: `    a_test.go:6: got ${n}\n` }), ev({ Action: 'fail', Package: 'p', Test: 'TestA/sub' })] : [ev({ Action: 'pass', Package: 'p', Test: 'TestA/sub' })]),
    ev({ Action: fails ? 'fail' : 'pass', Package: 'p', Test: 'TestA' }),
  ];
  const got = parseGo([...once(1, true), ...once(2, false), ...once(3, true)].join('\n'));
  assert.equal(got.passed, 1);
  assert.deepEqual(
    got.failures.map((f) => f.signature),
    ['a_test.go:6: got 1', 'a_test.go:6: got 3'],
  );
});
