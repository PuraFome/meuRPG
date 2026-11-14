import type { EffectDraft } from './effect-draft.types';
import { effectFieldPaths } from './spell-effect-checks';

/** The paths of a feature's aura block, which the spell's block names `table_spell.effect...`, moved under the effect's own `aura` field. */
export function underAura(at: string, paths: readonly string[]): string[] {
  return paths.map((p) => `${at}.aura${p.slice('table_spell.effect'.length)}`);
}

/** The paths of the inputs one effect draws, at `at` ("table_feat.effects[0]"): the effect, its type, the fields the menu says that type
 * reads and, for an aura, the fields of its block. */
export function effectInputPaths(
  at: string,
  e: EffectDraft,
  menu: { fieldsOf(type: string): readonly { name: string }[] },
): string[] {
  return [
    at,
    `${at}.type`,
    ...menu.fieldsOf(e.type).map((f) => `${at}.${f.name}`),
    ...(e.aura ? underAura(at, effectFieldPaths(e.aura)) : []),
  ];
}
