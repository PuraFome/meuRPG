import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { cwd } from 'node:process';

import { describeInviteErrorCode } from '../campaigns/invite-errors';
import { violationText } from '../content/content-violations';
import { BODY_REASON_TEXT } from './body-reasons';

/**
 * The closed list of error reasons (proto/error-reasons.json) against the
 * Portuguese sentences. The Go test in backend/internal/archtest keeps the list
 * equal to what the server can send; this spec keeps the web equal to the list:
 * a listed reason with no sentence fails, and a sentence for a reason that is
 * not listed fails. docs/architecture/api.md#error-reasons says how to add one.
 */

interface EnumField {
  field: string;
  enum: string;
  reasons: string[];
  unsent: string[];
}
interface ReasonList {
  details: { message: string; fields: EnumField[] }[];
  body: { reason: string }[];
  violations: { reason: string }[];
  redirects: { reason: string }[];
}

// The tests run from web/ (plain and under coverage); import.meta.url points elsewhere once coverage bundles the specs.
const WEB = cwd();
const APP = join(WEB, 'src', 'app');
const LIST: ReasonList = JSON.parse(
  readFileSync(join(WEB, '..', 'proto', 'error-reasons.json'), 'utf8'),
);

function sources(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const full = join(dir, e.name);
    if (e.isDirectory()) {
      return sources(full);
    }
    const code = e.name.endsWith('.ts') && !e.name.endsWith('.spec.ts');
    return code && !e.name.includes('testing') ? [readFileSync(full, 'utf8')] : [];
  });
}
const CODE = sources(APP).join('\n');

/** Every `Name.VALUE` the code names, read once: the values by the name they are reached through. */
const NAMED = new Map<string, Set<string>>();
for (const m of CODE.matchAll(/\b(\w+)\.([A-Z][A-Z0-9_]*)\b/g)) {
  const values = NAMED.get(m[1]) ?? new Set<string>();
  values.add(m[2]);
  NAMED.set(m[1], values);
}

/** The names a source gives an enum: its own, and the ones it imports it as. */
function namesOf(enumName: string): string[] {
  const aliases = [...CODE.matchAll(new RegExp(`\\b${enumName} as (\\w+)`, 'g'))].map((m) => m[1]);
  return [enumName, ...aliases];
}

/** The values the code names for an enum, whichever name it reaches the enum by. */
function handledValues(enumName: string): string[] {
  return namesOf(enumName).flatMap((n) => [...(NAMED.get(n) ?? [])]);
}

/** Whether a source names the enum value (`Enum.VALUE`, or `Enum.PREFIX_VALUE` for the enums whose generated names keep the prefix). */
function isHandled(handled: string[], value: string): boolean {
  return handled.some((h) => h === value || h.endsWith(`_${value}`));
}

describe('error reasons: a typed detail', () => {
  const byEnum = new Map<string, { message: string; r: string }[]>();
  for (const d of LIST.details) {
    for (const f of d.fields) {
      const list = byEnum.get(f.enum) ?? [];
      list.push(...f.reasons.map((r) => ({ message: d.message, r })));
      byEnum.set(f.enum, list);
    }
  }

  it('lists reasons', () => {
    const total = [...byEnum.values()].reduce((n, list) => n + list.length, 0);
    expect(total).toBeGreaterThan(200);
  });

  it.each([...byEnum.keys()].sort())(
    'has a place in the web for every reason of %s',
    (enumName) => {
      const handled = handledValues(enumName);
      const missing = (byEnum.get(enumName) ?? [])
        .filter((p) => !isHandled(handled, p.r))
        .map((p) => `${p.message}: ${enumName}.${p.r}`);
      expect(missing).toEqual([]);
    },
  );
});

describe('error reasons: the body of an HTTP route error', () => {
  const listed = [...new Set(LIST.body.map((b) => b.reason))].sort();

  it.each(listed)('has a Portuguese sentence for the listed reason %s', (reason) => {
    const text = (BODY_REASON_TEXT as Record<string, string | undefined>)[reason];
    expect(text, reason).toMatch(/^[A-ZÀ-Ý].{12,}[.!]$/);
  });

  it('has no sentence for a reason that is not listed', () => {
    expect(Object.keys(BODY_REASON_TEXT).sort()).toEqual(listed);
  });
});

describe('error reasons: a violation of the table content', () => {
  const listed = LIST.violations.map((v) => v.reason);
  /** Fields that say what the reason means in their kind of place: the entry, a name, an unknown path and the rows that have their own words. */
  const PROBES = [
    '',
    'table_race.name_pt',
    'table_x.unknown',
    'table_class.levels',
    'table_class.casting.kind',
    'table_race.traits[0].effects[0].value',
  ];
  const ctx = { aOne: 'uma magia' };
  const textOf = (field: string, reason: string) => violationText({ field, reason }, ctx);

  it.each(listed)('has a sentence of its own for the listed reason %s', (reason) => {
    const generic = 'reason-nobody-sends';
    expect(PROBES.some((field) => textOf(field, reason) !== textOf(field, generic))).toBe(true);
  });
});

describe('error reasons: the reason in the query of an error page', () => {
  const generic = describeInviteErrorCode(null);

  it('lists reasons', () => {
    expect(LIST.redirects.length).toBeGreaterThan(0);
  });

  it.each(LIST.redirects.map((r) => r.reason))(
    'has a sentence of its own for the reason %s of /invite/error',
    (reason) => {
      expect(describeInviteErrorCode(reason)).not.toBe(generic);
    },
  );
});
