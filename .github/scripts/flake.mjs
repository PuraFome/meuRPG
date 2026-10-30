#!/usr/bin/env node
// The tools of the flaky-test process (docs/ci.md, "Flaky tests"), shared by the flake-check workflow, the
// nightly hunt and the repeat of the changed tests.
//
//   flake.mjs parse <playwright|go> <file> [--side S] [--runner N]
//       reads what one runner's test run wrote (Playwright's JSON report, or `go test -json`) and prints one
//       result: {kind, side, runner, passed, failed, missing, failures: [{test, signature}]}
//   flake.mjs aggregate <dir> [--kind K] [--test T] [--head H] [--base B] [--runs N]
//       reads every result.json under <dir>, prints the verdict as markdown (for the job summary) and, with
//       --json <file>, writes the counts and the signatures as JSON.
//
// A signature is the first error line of a failure, with what changes from run to run (ids, addresses, times)
// taken out, so the same flake counts as one signature however often it comes.
import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

const ANSI = /\u001b\[[0-9;]*[A-Za-z]/g;

/** The first error line of a failure, normalized. */
export function signatureOf(text) {
  const lines = String(text ?? '')
    .replace(ANSI, '')
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean);
  const first = lines.find((l) => !/^(Error:)?\s*$/.test(l)) ?? '(no message)';
  return first
    .replace(/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi, '<id>')
    .replace(/0x[0-9a-f]+/gi, '<addr>')
    .replace(/\b\d+(\.\d+)?(ms|s|m)\b/g, '<time>')
    .replace(/\s+/g, ' ')
    .slice(0, 300);
}

/** Playwright's JSON report: one entry per repetition of the `chrome` project (the sign-in setup is not a test). */
export function parsePlaywright(report) {
  const out = { passed: 0, failed: 0, failures: [] };
  const visit = (suite) => {
    for (const spec of suite.specs ?? []) {
      for (const t of spec.tests ?? []) {
        if (t.projectName && t.projectName !== 'chrome') continue;
        if (t.status === 'skipped') continue;
        if (t.status === 'expected') {
          out.passed++;
          continue;
        }
        out.failed++;
        const results = t.results ?? [];
        const last = results[results.length - 1] ?? {};
        const message = last.error?.message ?? last.errors?.[0]?.message ?? (last.status ? `the test ${last.status}` : '');
        out.failures.push({ test: spec.title, signature: signatureOf(message) });
      }
    }
    for (const s of suite.suites ?? []) visit(s);
  };
  for (const s of report.suites ?? []) visit(s);
  // Errors of the run itself (a global setup that fails, a broken file) are failures with no test.
  for (const e of report.errors ?? []) {
    out.failed++;
    out.failures.push({ test: '(the run)', signature: signatureOf(e.message) });
  }
  return out;
}

const GO_SIGNATURE = /^(panic:|fatal error:|WARNING: DATA RACE|.+_test\.go:\d+:|.+\.go:\d+:\d+: )/;

/** `go test -json` events (one JSON per line): every top-level test is a run, a package that fails with no test is one too. */
export function parseGo(text) {
  const out = { passed: 0, failed: 0, failures: [] };
  const lines = new Map(); // "pkg\ttest" -> its output
  const packageOutput = new Map();
  const failedPackages = new Set();
  const failedTests = new Set();
  for (const raw of text.split('\n')) {
    if (!raw.startsWith('{')) continue;
    let ev;
    try {
      ev = JSON.parse(raw);
    } catch {
      continue;
    }
    const key = `${ev.Package}\t${ev.Test ?? ''}`;
    if (ev.Action === 'run' && ev.Test) {
      // -count repeats a test: each run starts with an empty output, its subtests' too.
      lines.delete(key);
      if (!ev.Test.includes('/')) for (const k of [...lines.keys()]) if (k.startsWith(`${key}/`)) lines.delete(k);
      continue;
    }
    if (ev.Action === 'output') {
      const into = ev.Test ? lines : packageOutput;
      const k = ev.Test ? key : ev.Package;
      into.set(k, (into.get(k) ?? '') + ev.Output);
      // A test's subtests print on their own key: their output counts for the top-level test.
      continue;
    }
    if (ev.Action !== 'pass' && ev.Action !== 'fail') continue;
    if (!ev.Test) {
      if (ev.Action === 'fail') failedPackages.add(ev.Package);
      continue;
    }
    if (ev.Test.includes('/')) {
      if (ev.Action === 'fail') failedTests.add(`${ev.Package}\t${ev.Test.split('/')[0]}`);
      continue;
    }
    if (ev.Action === 'pass') {
      out.passed++;
    } else {
      out.failed++;
      failedTests.add(key);
      out.failures.push({ test: ev.Test, signature: signatureOf(firstErrorLine(lines.get(key) ?? '', subOutput(lines, ev.Package, ev.Test))) });
    }
  }
  // A package that failed with no failed test (a panic, a build error, a timeout) is a failure of its own.
  for (const pkg of failedPackages) {
    if ([...failedTests].some((k) => k.startsWith(`${pkg}\t`))) continue;
    out.failed++;
    out.failures.push({ test: `(package ${pkg})`, signature: signatureOf(firstErrorLine(packageOutput.get(pkg) ?? '', '')) });
  }
  return out;
}

function subOutput(lines, pkg, test) {
  let text = '';
  for (const [k, v] of lines) if (k.startsWith(`${pkg}\t${test}/`)) text += v;
  return text;
}

/** The line that says what failed: a panic, a race, an assertion with its file and line; else the first line that is not the harness's. */
function firstErrorLine(...texts) {
  const all = texts.join('').split('\n').map((l) => l.trim()).filter(Boolean);
  return (
    all.find((l) => GO_SIGNATURE.test(l)) ??
    all.find((l) => !/^(=== |--- |FAIL|PASS|ok\s)/.test(l)) ??
    all[0] ??
    ''
  );
}

/** Adds the runners' results: counts per side and the signatures, most frequent first. */
export function aggregate(results) {
  const sides = {};
  for (const r of results) {
    const s = (sides[r.side] ??= { passed: 0, failed: 0, runners: 0, missing: 0, signatures: {} });
    s.runners++;
    s.passed += r.passed;
    s.failed += r.failed;
    if (r.missing) s.missing++;
    for (const f of r.failures) {
      const sig = (s.signatures[f.signature] ??= { count: 0, tests: new Set() });
      sig.count++;
      sig.tests.add(f.test);
    }
  }
  for (const s of Object.values(sides)) {
    s.signatures = Object.entries(s.signatures)
      .map(([signature, v]) => ({ signature, count: v.count, tests: [...v.tests].sort() }))
      .sort((a, b) => b.count - a.count || a.signature.localeCompare(b.signature));
  }
  return sides;
}

const rate = (s) => (s && s.passed + s.failed > 0 ? s.failed / (s.passed + s.failed) : null);
const percent = (r) => (r === null ? 'n/a' : `${(r * 100).toFixed(1)}%`);

/** The verdict: what the head did against the base, and what to do next. */
export function verdict(sides) {
  const head = sides.head;
  const base = sides.base;
  if (!head || !base) return { kind: 'incomplete', text: 'A side has no result: read the runners of the run.' };
  const hr = head.passed + head.failed;
  const br = base.passed + base.failed;
  if (hr === 0 || br === 0) {
    const none = hr === 0 ? 'head' : 'base';
    return { kind: 'no-runs', text: `The ${none} side ran no test: the test does not exist on that ref, or the title or regex matches nothing there.` };
  }
  const headRate = head.failed / hr;
  const baseRate = base.failed / br;
  if (headRate > baseRate) {
    return { kind: 'worse', text: `The head fails more often than the base (${percent(headRate)} against ${percent(baseRate)}). Dispatch again with \`runs=100\` to tell the train's doing from chance.` };
  }
  if (head.failed === 0 && base.failed === 0) {
    return { kind: 'clean', text: `No failure on either side in ${hr} + ${br} runs: it is not reproduced. Read its artifacts and add diagnostics; do not guess a fix.` };
  }
  return { kind: 'not-worse', text: `The head does not fail more often than the base (${percent(headRate)} against ${percent(baseRate)}). If the base fails, the flake is there without the train.` };
}

export function markdown(sides, meta = {}) {
  const v = verdict(sides);
  const out = [`## flake-check: ${meta.test ?? '(test)'}`, ''];
  if (meta.kind) out.push(`Kind: ${meta.kind}. Head: \`${meta.head ?? '?'}\`. Base: \`${meta.base ?? '?'}\`. Runs per side asked: ${meta.runs ?? '?'}.`, '');
  out.push('| Side | Runs | Passed | Failed | Failure rate | Runners |', '|---|---|---|---|---|---|');
  for (const name of ['head', 'base']) {
    const s = sides[name];
    if (!s) {
      out.push(`| ${name} | no result | | | | |`);
      continue;
    }
    out.push(`| ${name} | ${s.passed + s.failed} | ${s.passed} | ${s.failed} | ${percent(rate(s))} | ${s.runners}${s.missing ? ` (${s.missing} found no test)` : ''} |`);
  }
  out.push('', `**${v.text}**`, '');
  for (const name of ['head', 'base']) {
    const s = sides[name];
    if (!s || s.signatures.length === 0) continue;
    out.push(`### Failures on ${name}`, '');
    for (const g of s.signatures) out.push(`- ${g.count}× \`${g.signature.replace(/`/g, "'")}\` (${g.tests.join(', ')})`);
    out.push('');
  }
  return out.join('\n');
}

function flag(args, name) {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : undefined;
}

function findResults(dir) {
  const found = [];
  const walk = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) walk(p);
      else if (e.name === 'result.json') found.push(JSON.parse(fs.readFileSync(p, 'utf8')));
    }
  };
  walk(dir);
  return found;
}

function main(argv) {
  const [cmd, a, b] = argv;
  if (cmd === 'parse') {
    const text = fs.existsSync(b) ? fs.readFileSync(b, 'utf8') : '';
    const missing = text.trim() === '';
    const parsed = a === 'go' ? parseGo(text) : missing ? { passed: 0, failed: 0, failures: [] } : parsePlaywright(JSON.parse(text));
    const none = parsed.passed + parsed.failed === 0;
    process.stdout.write(JSON.stringify({ kind: a, side: flag(argv, 'side') ?? '', runner: Number(flag(argv, 'runner') ?? 0), ...parsed, missing: none }) + '\n');
    return 0;
  }
  if (cmd === 'aggregate') {
    const sides = aggregate(findResults(a));
    const meta = { kind: flag(argv, 'kind'), test: flag(argv, 'test'), head: flag(argv, 'head'), base: flag(argv, 'base'), runs: flag(argv, 'runs') };
    process.stdout.write(markdown(sides, meta) + '\n');
    const json = flag(argv, 'json');
    if (json) fs.writeFileSync(json, JSON.stringify({ ...meta, verdict: verdict(sides), sides }, null, 2) + '\n');
    return 0;
  }
  process.stderr.write('usage: flake.mjs parse <playwright|go> <file> | aggregate <dir>\n');
  return 2;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) process.exit(main(process.argv.slice(2)));
