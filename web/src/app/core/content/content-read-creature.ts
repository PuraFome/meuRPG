import type { TableCreature } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { type EntryRead, type NameOf, type ReadRow } from './content-read-parts';
import { modSub, modTitle } from './creature-damage-mods';
import { numberValue, signedBonus } from './creature-text';

/** One number of a creature as the table reads it: the fixed value (or the base's), then "com dono, <formula>". */
function numberRow(label: string, fixed: number, formula: string, ifNone: string): ReadRow | null {
  const value = numberValue(fixed > 0 ? String(fixed) : ifNone, formula);
  return value === '' ? null : { label, value };
}

/** A table creature as the table reads it: the base and the numbers it changes, then the traits and the attacks it adds. The full
 * stat block read (what the base gives) is the bestiary's. */
export function readCreature(c: TableCreature, nameOf: NameOf): EntryRead {
  const bonus = signedBonus(c.damageBonus);
  const rows = [
    { label: 'Base', value: nameOf(c.baseKey) },
    numberRow('Pontos de vida', c.hitPoints, c.hitPointsFormula, 'os da base'),
    numberRow('Classe de armadura', c.armorClass, c.armorClassFormula, 'a da base'),
    numberRow('Bônus no dano', 0, c.damageBonusFormula, bonus),
    numberRow('Bônus de ataque', 0, c.attackBonusFormula, ''),
  ].filter((r): r is ReadRow => r !== null);
  const sections = [
    {
      title: 'Resistências, imunidades e vulnerabilidades',
      items: c.damageMods.map((m) => {
        const d = { id: '', kind: m.kind, types: [...m.damageTypes], qualifier: m.qualifier };
        return { title: modTitle(d, nameOf), text: `${modSub(d)}.` };
      }),
    },
    {
      title: 'Traços',
      items: c.traits.map((t) => ({ title: t.namePt, text: t.textPt })),
    },
    {
      title: 'Ataques',
      items: c.attacks.map((a) => ({
        title: a.namePt,
        text: `${a.diceCount}d${a.diceSides}${a.toHit === '' && a.damageBonus === '' ? ', acerto e dano da criatura' : ''}.`,
      })),
    },
  ].filter((s) => s.items.length > 0);
  return { rows, sections, text: [] };
}
