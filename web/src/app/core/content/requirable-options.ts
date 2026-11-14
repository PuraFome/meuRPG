import { type TableEntry, TableContentKind } from '../../../gen/meurpg/rules/v1/table_content_pb';
import type { SelectOption } from '../../shared/form-fields/select-field';
import type { EffectMenuVm } from './effect-draft';
import { OFFERERS } from './option-draft';

/**
 * The options a prerequisite may name (#580): an option's (S1) and a feat's. The SRD's options of every set, and the table's own
 * (`@mesa`) with "(da mesa)" after the name; a Pact Boon is one of the set "pact_boon".
 */

const byName = (a: SelectOption, b: SelectOption): number =>
  a.label.localeCompare(b.label, 'pt-BR');

/** The SRD's options of these sets, with the set's prefix taken off the name. */
export function srdOptionsOf(
  menu: EffectMenuVm,
  sets: readonly string[],
): { key: string; namePt: string }[] {
  return menu.optionSets
    .filter((s) => sets.includes(s.key))
    .flatMap((s) => s.options.filter((o) => !o.key.endsWith('@mesa')))
    .map((o) => ({ key: o.key, namePt: o.namePt.replace(/^[^:]+: /, '') }));
}

function tableOptionsOf(entries: readonly TableEntry[]): readonly TableEntry[] {
  return entries.filter((e) => e.kind === TableContentKind.OPTION && !e.archived);
}

/** What an entry may require, by name: every SRD option and the table's, but `own` (an option never requires itself). */
export function requirableOptions(
  menu: EffectMenuVm,
  entries: readonly TableEntry[],
  own = '',
): SelectOption[] {
  const srd = srdOptionsOf(menu, Object.values(OFFERERS).flat());
  const table = tableOptionsOf(entries).filter((o) => o.key !== own);
  return [
    ...srd.map((o) => ({ value: o.key, label: o.namePt })),
    ...table.map((o) => ({ value: o.key, label: `${o.namePt} (da mesa)` })),
  ].sort(byName);
}

/** "Nenhuma", then the Pact Boons: the SRD's and the table's. */
export function pactChoices(menu: EffectMenuVm, entries: readonly TableEntry[]): SelectOption[] {
  return [
    { value: '', label: 'Nenhuma' },
    ...srdOptionsOf(menu, OFFERERS['pact_boon']).map((o) => ({ value: o.key, label: o.namePt })),
    ...tableOptionsOf(entries)
      .filter((o) => o.body.case === 'tableOption' && o.body.value.set === 'pact_boon')
      .map((o) => ({ value: o.key, label: `${o.namePt} (da mesa)` })),
  ];
}
