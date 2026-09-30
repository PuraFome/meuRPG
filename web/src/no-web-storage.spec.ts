import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join, relative, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

/**
 * Vinicius's hard rule: nothing in this app may read or write Web Storage
 * (localStorage, sessionStorage, IndexedDB), set a cookie from JavaScript,
 * or use a Worker to hold a token or personal data (docs/privacidade.md,
 * docs/arquitetura.md#frontend-web). The only place a session lives is the
 * httpOnly `__Host-meurpg_session` cookie, which this app's own code never
 * reads or writes — `AuthService` only ever calls `IdentityService`.
 *
 * This is a cheap, blunt whole-text scan of every .ts/.html file under
 * `src/`, excluding `src/gen/` (generated code we do not author) and
 * `*.spec.ts` (a spec may legitimately assert the *absence* of a storage
 * call, e.g. `vi.spyOn(Storage.prototype, 'setItem')`, without that
 * assertion itself being a use of the forbidden API). It fails loudly the
 * moment someone reaches for one of these APIs, instead of relying on a
 * reviewer noticing by hand.
 */

const SRC_ROOT = dirname(fileURLToPath(import.meta.url));

function listFiles(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === 'gen' || entry.name === 'node_modules') {
      continue;
    }
    const full = join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...listFiles(full));
    } else if (entry.isFile()) {
      out.push(full);
    }
  }
  return out;
}

const FORBIDDEN: Array<{ pattern: RegExp; label: string }> = [
  { pattern: /\blocalStorage\b/, label: 'localStorage' },
  { pattern: /\bsessionStorage\b/, label: 'sessionStorage' },
  { pattern: /\bindexedDB\b/i, label: 'IndexedDB' },
  { pattern: /document\.cookie\s*=/, label: 'setting document.cookie from JavaScript' },
  { pattern: /\bnew\s+(Shared)?Worker\s*\(/, label: 'a Worker' },
];

const files = listFiles(SRC_ROOT)
  .map((path) => ({ path, rel: relative(SRC_ROOT, path).split(sep).join('/') }))
  .filter(({ rel }) => (rel.endsWith('.ts') || rel.endsWith('.html')) && !rel.endsWith('.spec.ts'));

describe("no Web Storage (Vinicius's hard rule — see docs/privacidade.md)", () => {
  it('scanned at least a handful of source files (the scan is not a no-op)', () => {
    expect(files.length).toBeGreaterThan(10);
  });

  for (const { path, rel } of files) {
    it(`${rel} does not use Web Storage, a JS-set cookie, or a Worker`, () => {
      const text = readFileSync(path, 'utf8');
      for (const { pattern, label } of FORBIDDEN) {
        expect(text, `${rel} appears to use ${label}`).not.toMatch(pattern);
      }
    });
  }
});
