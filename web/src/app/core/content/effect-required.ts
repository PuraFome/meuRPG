import type { EffectDraft, EffectMenuVm } from './effect-draft';
import { rangeFeet } from './range-text';

/** Whether a required field the switch of `missingRequired` does not name (a closed list, a text) is still empty. */
function choiceEmpty(d: EffectDraft, name: string): boolean {
  const v = d[name.replace(/_(\w)/g, (_, c: string) => c.toUpperCase()) as keyof EffectDraft];
  return typeof v === 'string' && v.trim() === '';
}

/** A required field of the type that still has nothing: the editor says so before the server does. */
export function missingRequired(d: EffectDraft, menu: EffectMenuVm): string[] {
  const out: string[] = [];
  for (const f of menu.fieldsOf(d.type)) {
    if (!f.required) {
      continue;
    }
    const empty = ((): boolean => {
      switch (f.name) {
        case 'target':
          return d.target === '';
        case 'mode':
          return d.mode === '';
        case 'value':
          return d.value.trim() === '';
        case 'proficiency':
          return d.proficiency === '';
        case 'roll':
          return d.roll === '';
        case 'targets':
          return d.targets.length === 0;
        case 'sense':
          return d.sense === '';
        case 'range_ft':
          return rangeFeet(d.rangeM) <= 0;
        case 'resource':
          return d.resource.trim() === '';
        case 'max':
          return d.max.trim() === '';
        case 'recharge':
          return d.recharge === '';
        case 'choice':
          return d.choice === '';
        case 'count':
          return d.count <= 0;
        default:
          return choiceEmpty(d, f.name);
      }
    })();
    if (empty) {
      out.push(f.name);
    }
  }
  return out;
}
