import type { TableEffect } from '../../../gen/meurpg/rules/v1/table_content_pb';
import type { EffectDraft, EffectInit } from './effect-draft.types';
import { signed } from '../creatures/creature-format';

/** The menu's types the editor does not offer yet: the server lists them, but the editor has no block that writes their fields.
 * A stored effect of one of them is kept exactly as it was stored (`keptEffect`), shown read-only, and written back untouched.
 * The branch that adds the `power_attack` block removes its entry. */
export const TYPES_NOT_OFFERED: readonly string[] = ['power_attack'];

/** The stored effect a draft carries untouched, or `undefined` for a type the editor offers: the draft's other fields are then
 * the editor's own and the effect is written from them. */
export function keptOf(e: TableEffect): TableEffect | undefined {
  return TYPES_NOT_OFFERED.includes(e.type) ? e : undefined;
}

/** What a kept effect sends: the stored effect with only the fields that hold something, as a request carries any effect. */
export function keptToInit(kept: TableEffect): EffectInit {
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(kept)) {
    const empty =
      key.startsWith('$') ||
      value === '' ||
      value === 0 ||
      value === false ||
      value === undefined ||
      (Array.isArray(value) && value.length === 0);
    if (!empty) {
      out[key] = Array.isArray(value) ? [...(value as unknown[])] : value;
    }
  }
  return out as EffectInit;
}

/** The numbers of a kept effect, as the lines the editor shows in place of its fields: [label, text]. A closed-list key is
 * shown by `nameOf` (the Portuguese name the menu gives it, the key when it does not know it). */
export function keptLines(
  d: EffectDraft,
  nameOf: (list: string, key: string) => string,
): readonly (readonly [string, string])[] {
  const k = d.kept;
  if (!k) {
    return [];
  }
  const lines: [string, string][] = [
    ['Penalidade no ataque', signed(-k.attackPenalty)],
    ['Bônus no dano', signed(k.damageBonus)],
    ['Arma', WEAPON_PT[k.weapon === '' ? 'any' : k.weapon] ?? k.weapon],
  ];
  if (k.weaponProperties.length > 0) {
    lines.push([
      'Propriedades da arma',
      k.weaponProperties.map((p) => nameOf('weapon_properties', p)).join(', '),
    ]);
  }
  lines.push(['Só com arma proficiente', k.onlyProficient ? 'sim' : 'não']);
  if (k.resource !== '') {
    lines.push(['Recurso gasto', k.resource]);
  }
  return lines;
}

const WEAPON_PT: Readonly<Record<string, string>> = {
  any: 'qualquer',
  melee: 'corpo a corpo',
  ranged: 'à distância',
};

/** The line that says why nothing of a kept effect is editable. */
export const KEPT_NOTE =
  'O editor ainda não muda este efeito. Ele é salvo como está; só dá para removê-lo.';

/** What the picker draws for a kept effect: its lines and the note, or `null` for an effect the editor writes. */
export function keptView(d: EffectDraft, menu: { nameOf(list: string, key: string): string }) {
  return d.kept ? { rows: keptLines(d, (l, k) => menu.nameOf(l, k)), note: KEPT_NOTE } : null;
}
