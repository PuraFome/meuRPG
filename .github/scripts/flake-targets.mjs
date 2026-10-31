#!/usr/bin/env node
// Maps a diff to the tests the repeat-changed workflow runs many times (docs/ci.md, "Repeat of the changed tests").
//
//   flake-targets.mjs --base <ref>      prints the targets as JSON, from `git diff <ref>...HEAD`
//
// Two kinds of target:
//   - the tests whose body changed: a Playwright `test(...)` of e2e/tests/*.spec.ts (each spec is parsed once), a Go `func TestX` of a _test.go;
//   - for a changed helper (an exported function of an e2e/tests/*-support.ts, any function of
//     backend/internal/leaktest), a sample of up to SAMPLE tests that call it.
// A change outside any test body (a comment above a test, an import, a helper that is not one of the above) maps to nothing.
//
// Output: {playwright: [{file, title, grep, signins, why}], go: [{pkg, test, why}], notes: [string]}. `grep` is a regular expression
// for Playwright's --grep (the title with its metacharacters escaped); `why` is "changed" or "calls <helper>".
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import { createRequire } from 'node:module';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

export const SAMPLE = 5;

// The TypeScript compiler API reads the specs, so a test header that Prettier wraps over several lines is found like
// any other. It comes from e2e/lint (TypeScript 5.x, with the JavaScript API; e2e/ itself has the native 7.x):
// `npm ci --ignore-scripts` there first.
const requireFromLint = createRequire(new URL('../../e2e/lint/package.json', import.meta.url));
let tsApi;
function typescript() {
  try {
    tsApi ??= requireFromLint('typescript');
  } catch (err) {
    throw new Error(`flake-targets.mjs reads the specs with TypeScript's compiler API: run \`npm ci --ignore-scripts\` in e2e/lint first (${err.message})`);
  }
  return tsApi;
}

const SPEC = /^e2e\/tests\/[^/]+\.spec\.ts$/;
const SUPPORT = /^e2e\/tests\/[^/]+-support\.ts$/;
const GO_TEST = /^backend\/.*_test\.go$/;
const LEAKTEST = /^backend\/internal\/leaktest\/[^/]+\.go$/;

/** The new-side line ranges the diff (made with -U0) changes, per file: {file: [[from, to], ...]}. A pure deletion is [line before, line before, true]. */
export function changedLines(diff) {
  const out = {};
  let file = null;
  for (const line of diff.split('\n')) {
    const f = /^\+\+\+ b\/(.+)$/.exec(line);
    if (f) {
      file = f[1];
      out[file] ??= [];
      continue;
    }
    if (line.startsWith('+++ ')) {
      file = null; // /dev/null: the file is gone, its hunks are no test's
      continue;
    }
    const h = /^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@/.exec(line);
    if (h && file) {
      const start = Number(h[1]);
      const count = h[2] === undefined ? 1 : Number(h[2]);
      // A pure deletion names the line BEFORE it (`+9,0`): it touches only what spans that line and the next.
      out[file].push(count === 0 ? [start, start, true] : [start, start + count - 1]);
    }
  }
  return out;
}

const touches = (ranges, from, to) => ranges.some(([a, b, deletion]) => (deletion ? from <= a && to >= a + 1 : a <= to && b >= from));

const escapeRe = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

/** The members of `test.` that are not a test: groups, hooks and the calls that change one. */
const NOT_A_TEST = new Set(['describe', 'beforeAll', 'beforeEach', 'afterAll', 'afterEach', 'step', 'use', 'slow', 'setTimeout', 'info', 'skip', 'fixme', 'fail']);

/** The title of a test(...) call, from its first argument, as {title, regex}; a template literal's ${...} becomes ".*". */
function titleOfNode(ts, arg) {
  if (ts.isStringLiteral(arg) || ts.isNoSubstitutionTemplateLiteral(arg)) return { title: arg.text, regex: escapeRe(arg.text) };
  if (ts.isTemplateExpression(arg)) {
    const parts = [arg.head.text, ...arg.templateSpans.map((span) => span.literal.text)];
    return { title: arg.getText().slice(1, -1), regex: parts.map(escapeRe).join('.*') };
  }
  return null;
}

/** A `test.describe(...)`, `.serial`, `.parallel` and so on: the group whose callback scopes the hooks inside it. */
const isDescribe = (ts, callee) => {
  let node = callee;
  while (ts.isPropertyAccessExpression(node)) {
    if (ts.isIdentifier(node.expression) && node.expression.text === 'test') return node.name.text === 'describe';
    node = node.expression;
  }
  return false;
};

/**
 * Reads a spec once: its tests, {title, regex, from, to, body, hooks, hookLines}. A `test.beforeEach/beforeAll` hook runs
 * for the tests inside its own `test.describe` (nested ones too), or for every test of the file when it is outside any:
 * `hooks` is the text of the hooks that reach a test and `hookLines` their [from, to] lines.
 */
export function parseSpec(text) {
  const ts = typescript();
  const source = ts.createSourceFile('spec.ts', text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
  const lines = (node) => [source.getLineAndCharacterOfPosition(node.getStart(source)).line + 1, source.getLineAndCharacterOfPosition(node.getEnd()).line + 1];
  const tests = [];
  const hooks = [];
  const visit = (node, groups) => {
    let inner = groups;
    if (ts.isCallExpression(node)) {
      const callee = node.expression;
      const member = ts.isPropertyAccessExpression(callee) && ts.isIdentifier(callee.expression) && callee.expression.text === 'test' ? callee.name.text : null;
      const [from, to] = lines(node);
      const body = text.slice(node.getStart(source), node.getEnd());
      if (member === 'beforeEach' || member === 'beforeAll') hooks.push({ from, to, body, groups });
      else if ((ts.isIdentifier(callee) && callee.text === 'test') || (member !== null && !NOT_A_TEST.has(member))) {
        const title = node.arguments.length > 0 ? titleOfNode(ts, node.arguments[0]) : null;
        if (title) tests.push({ ...title, from, to, body, groups });
      } else if (isDescribe(ts, callee)) {
        inner = [...groups, node.getStart(source)];
      }
    }
    ts.forEachChild(node, (child) => visit(child, inner));
  };
  visit(source, []);
  return {
    tests: tests.map(({ groups, ...t }) => {
      const reaching = hooks.filter((h) => h.groups.every((g) => groups.includes(g)));
      return { ...t, hooks: reaching.map((h) => h.body).join('\n'), hookLines: reaching.map((h) => [h.from, h.to]) };
    }),
  };
}

/** The tests of a spec, as parseSpec gives them. */
export function playwrightTests(text) {
  return parseSpec(text).tests;
}

/**
 * How many sign-ins one run of a test may make, for the login rate limit. A textual count misses a closure called
 * several times (login.spec.ts "cada pessoa tem uma conta": one `signIn(` in `accountOf`, called 3 times), a loop, a
 * hook and a helper, so a test that signs in at all counts as at least 3: `body` is the test and the hooks that reach it,
 * `signers` the names of the functions that sign in (signIn itself, and the helpers that call it), and a test that
 * opens /auth/login itself counts too.
 */
export function signinsOf(body, signers = ['signIn']) {
  const literal = (body.match(/\bsignIn\(/g) ?? []).length;
  const heavy = literal > 0 || body.includes('/auth/login') || [...signers].some((name) => new RegExp(`\\b${escapeRe(name)}\\(`).test(body));
  return heavy ? Math.max(literal, 3) : 0;
}

/** The top-level statements of a file whose lines start in column 0: {name, from, to, body}. A leading comment belongs to the statement. */
export function topLevel(text, nameOf) {
  const lines = text.split('\n');
  const starts = [];
  for (let i = 0; i < lines.length; i++) {
    const name = nameOf(lines[i]);
    if (name) starts.push({ name, i });
  }
  const isStarter = (l) => l !== '' && !/^[\s})\]]/.test(l) && !/^(\/\/|\/\*|\*)/.test(l);
  return starts.map((s) => {
    let end = lines.length - 1;
    for (let j = s.i + 1; j < lines.length; j++) {
      if (isStarter(lines[j])) {
        end = j - 1;
        break;
      }
    }
    while (end > s.i && (lines[end].trim() === '' || /^(\/\/|\/\*|\s*\*)/.test(lines[end]))) end--;
    return { name: s.name, from: s.i + 1, to: end + 1, body: lines.slice(s.i, end + 1).join('\n') };
  });
}

const TS_FUNCTION = /^(?:export\s+)?(?:async\s+)?function\s+(\w+)/;
const TS_ARROW = /^(?:export\s+)?const\s+(\w+)\s*(?::[^=]+)?=\s*(?:async\s*)?(?:\(|function\b|\w+\s*=>)/;
/** An exported function of a support file: the helpers whose change maps to their callers. */
export const exportedOf = (line) => (/^export\s/.test(line) ? (TS_FUNCTION.exec(line) ?? TS_ARROW.exec(line))?.[1] : undefined);
/** Any top-level function of a TypeScript file, exported or not: a spec's own helper can call a support one. */
export const tsFuncOf = (line) => (TS_FUNCTION.exec(line) ?? TS_ARROW.exec(line))?.[1];
/** A Go function or method (with a receiver, with type parameters). */
export const goFuncOf = (line) => /^func\s+(?:\([^)]*\)\s*)?(\w+)(?:\[[^\]]*\])?\(/.exec(line)?.[1];

const callsAny = (body, names) => [...names].some((n) => new RegExp(`\\b${escapeRe(n)}\\b`).test(body));

/** The helper and the functions that call it, up to three levels: a test that calls a helper that calls it calls it too. */
function withCallers(name, functionsOf) {
  const names = new Set([name]);
  for (let level = 0; level < 3; level++) {
    const before = names.size;
    for (const f of functionsOf()) if (!names.has(f.name) && callsAny(f.body, names)) names.add(f.name);
    if (names.size === before) break;
  }
  return names;
}

/** Up to n items spread evenly over the (sorted) list, the same ones every time. */
export function sample(items, n = SAMPLE) {
  if (items.length <= n) return items;
  return Array.from({ length: n }, (_, k) => items[Math.floor((k * items.length) / n)]);
}

/**
 * The targets of a diff. `diff` is `git diff -U0` output; `read(file)` gives a file's current text (undefined if it is gone);
 * `files(prefix)` lists the repository's files under a prefix, for the callers of a helper.
 */
export function mapDiff({ diff, read: readFile, files }) {
  // Each file is read once; its parse is cached below.
  const texts = new Map();
  const read = (file) => (texts.has(file) ? texts.get(file) : texts.set(file, readFile(file)).get(file));
  const changed = changedLines(diff);
  const playwright = new Map();
  const go = new Map();
  const notes = [];
  const addPlaywright = (file, t, why) => {
    const key = `${file}\t${t.title}`;
    if (!playwright.has(key) || why === 'changed') playwright.set(key, { file, title: t.title, grep: t.regex, signins: signinsOf(`${t.body}\n${t.hooks}`, signersOf()), why });
  };
  const addGo = (pkg, test, why) => {
    const key = `${pkg}\t${test}`;
    if (!go.has(key) || why === 'changed') go.set(key, { pkg, test, why });
  };
  // Each file is parsed once per kind of parse, however many tests, helpers and levels ask for it.
  const cache = new Map();
  const once = (kind, file, parse) => {
    const key = `${kind}\t${file}`;
    if (!cache.has(key)) cache.set(key, parse());
    return cache.get(key);
  };
  const testsOf = (file) => once('spec', file, () => parseSpec(read(file))).tests;
  const functionsOf = (kind, file, nameOf) => once(kind, file, () => topLevel(read(file), nameOf));
  // The functions that sign in: signIn and, up to three levels, the helpers of e2e/tests that call it.
  let signers;
  const signersOf = () => (signers ??= withCallers('signIn', () => files('e2e/tests/').filter((f) => f.endsWith('.ts')).sort().flatMap((f) => (read(f) === undefined ? [] : functionsOf('ts', f, tsFuncOf)))));
  const goPkg = (file) => `./${file.replace(/^backend\//, '').replace(/\/[^/]+$/, '')}`;

  const helpers = []; // {kind, name, file}
  for (const [file, ranges] of Object.entries(changed)) {
    const text = read(file);
    if (text === undefined) continue;
    if (SPEC.test(file)) {
      // A change in a hook changes every test the hook reaches.
      for (const t of testsOf(file)) if (touches(ranges, t.from, t.to) || t.hookLines.some(([from, to]) => touches(ranges, from, to))) addPlaywright(file, t, 'changed');
    } else if (SUPPORT.test(file)) {
      for (const f of functionsOf('exported', file, exportedOf)) if (touches(ranges, f.from, f.to)) helpers.push({ kind: 'playwright', name: f.name, file });
    } else if (GO_TEST.test(file) || LEAKTEST.test(file)) {
      const leak = LEAKTEST.test(file);
      for (const f of functionsOf('go', file, goFuncOf)) {
        if (!touches(ranges, f.from, f.to)) continue;
        if (/^Test[A-Z_0-9]/.test(f.name) || f.name === 'Test') addGo(goPkg(file), f.name, 'changed');
        else if (leak) helpers.push({ kind: 'go', name: f.name, file });
      }
    }
  }

  for (const h of helpers) {
    const callers = [];
    if (h.kind === 'playwright') {
      const tsFiles = files('e2e/tests/').filter((f) => f.endsWith('.ts')).sort();
      const names = withCallers(h.name, () => tsFiles.flatMap((f) => (read(f) === undefined ? [] : functionsOf('ts', f, tsFuncOf))));
      for (const file of tsFiles.filter((f) => SPEC.test(f))) {
        const text = read(file);
        if (text === undefined) continue;
        for (const t of testsOf(file)) if (callsAny(`${t.body}\n${t.hooks}`, names)) callers.push({ file, t });
      }
      const picked = sample(callers);
      for (const c of picked) addPlaywright(c.file, c.t, `calls ${h.name}`);
      if (callers.length === 0) notes.push(`${h.file}: the helper ${h.name} changed and no test calls it`);
      else if (callers.length > picked.length) notes.push(`${h.file}: the helper ${h.name} has ${callers.length} callers; ${picked.length} run`);
    } else {
      const goFiles = files('backend/internal/leaktest/').filter((f) => GO_TEST.test(f)).sort();
      const isTest = (n) => /^Test[A-Z_0-9]/.test(n);
      const names = withCallers(h.name, () => goFiles.flatMap((f) => (read(f) === undefined ? [] : functionsOf('go', f, goFuncOf).filter((fn) => !isTest(fn.name)))));
      for (const file of goFiles) {
        const text = read(file);
        if (text === undefined) continue;
        for (const f of functionsOf('go', file, goFuncOf)) if (isTest(f.name) && callsAny(f.body, names)) callers.push({ file, name: f.name });
      }
      const picked = sample(callers);
      for (const c of picked) addGo(goPkg(c.file), c.name, `calls ${h.name}`);
      if (callers.length === 0) notes.push(`${h.file}: the helper ${h.name} changed and no test calls it`);
      else if (callers.length > picked.length) notes.push(`${h.file}: the helper ${h.name} has ${callers.length} callers; ${picked.length} run`);
    }
  }

  const byKey = (a, b) => (a.file ?? a.pkg).localeCompare(b.file ?? b.pkg) || (a.title ?? a.test).localeCompare(b.title ?? b.test);
  return { playwright: [...playwright.values()].sort(byKey), go: [...go.values()].sort(byKey), notes };
}

function listFiles(prefix) {
  return execFileSync('git', ['ls-files', prefix], { encoding: 'utf8' }).split('\n').filter(Boolean);
}

function main(argv) {
  const i = argv.indexOf('--base');
  if (i < 0 || !argv[i + 1]) {
    process.stderr.write('usage: flake-targets.mjs --base <ref>\n');
    return 2;
  }
  const diff = execFileSync('git', ['-c', 'core.quotePath=false', 'diff', '-U0', '--no-color', '--no-ext-diff', '--src-prefix=a/', '--dst-prefix=b/', `${argv[i + 1]}...HEAD`, '--', 'e2e/tests', 'backend/internal', 'backend/migrations'], {
    encoding: 'utf8',
    maxBuffer: 256 * 1024 * 1024,
  });
  const read = (f) => (fs.existsSync(f) ? fs.readFileSync(f, 'utf8') : undefined);
  process.stdout.write(JSON.stringify(mapDiff({ diff, read, files: listFiles }), null, 2) + '\n');
  return 0;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) process.exit(main(process.argv.slice(2)));
