import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import { WeaponCategory } from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  TableContentKind,
  TableEntrySchema,
  TableWeaponSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { CONTENT_NAV, KIND_NOUNS, KIND_WORDS, entrySupport, navOfKind } from './content-kinds';
import { readEntry } from './content-read';
import { catalog } from './content-testing';
import { weaponSupport } from './weapon-text';

const nameOf = (k: string): string =>
  (
    ({
      'damage-type:slashing': 'Cortante',
      'weapon-property:versatile': 'Versátil',
    }) as Record<string, string>
  )[k] ?? k;
void catalog;

const axe = create(TableEntrySchema, {
  key: 'weapon:machado-de-lenhador@mesa',
  kind: TableContentKind.WEAPON,
  namePt: 'Machado de Lenhador',
  body: {
    case: 'tableWeapon',
    value: create(TableWeaponSchema, {
      namePt: 'Machado de Lenhador',
      category: WeaponCategory.SIMPLE,
      damage: '1d6',
      damageType: 'damage-type:slashing',
      twoHandedDamage: '1d8',
      properties: ['weapon-property:versatile'],
      weightOz: 64,
      priceCp: 400,
    }),
  },
});

describe('the weapon kind of the table content', () => {
  it('has its own place in the left menu, editable, after the creatures', () => {
    const nav = navOfKind(TableContentKind.WEAPON);
    expect(nav?.slug).toBe('weapons');
    expect(nav).toMatchObject({
      plural: 'Armas',
      newLabel: 'Nova arma',
      createSegment: 'weapon',
      editable: true,
    });
    expect(CONTENT_NAV[CONTENT_NAV.length - 1].slug).toBe('weapons');
  });

  it('is named with its article', () => {
    expect(KIND_WORDS[TableContentKind.WEAPON]).toBe('Arma');
    expect(KIND_NOUNS[TableContentKind.WEAPON]).toMatchObject({
      article: 'A',
      noun: 'arma',
      saved: 'salva',
      archived: 'arquivada',
    });
  });

  it('is one line in the list', () => {
    expect(entrySupport(axe, nameOf)).toBe(
      weaponSupport(axe.body.value as ReturnType<typeof create<typeof TableWeaponSchema>>, nameOf),
    );
  });

  it('reads as rows for a player: category, damage, properties, weight, price', () => {
    const read = readEntry(axe, nameOf);
    const rows = Object.fromEntries(read.rows.map((r) => [r.label, r.value.replace(/ /g, ' ')]));
    expect(rows).toEqual({
      Categoria: 'Simples, corpo a corpo',
      Dano: '1d6 cortante',
      Propriedades: 'Versátil (1d8)',
      Peso: '2 kg (4 lb)',
      Preço: '4 PO',
    });
  });
});
