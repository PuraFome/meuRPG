import { create } from '@bufbuild/protobuf';

import { SpellTableOptionSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  damageTypeName,
  tableOptionPicks,
  tableOptionRows,
  toggledTable,
} from './table-cast-options';

const swap = create(SpellTableOptionSchema, {
  key: 'feature:tinted-flame@mesa',
  namePt: 'Chama tingida',
  summaryPt: 'O dano da magia vira de outro tipo.',
  change: 'damage_type_swap',
  resource: 'tintas',
  usesLeft: 2,
  allowed: true,
  damageTypeKeys: ['damage-type:cold', 'damage-type:fire'],
});
const always = create(SpellTableOptionSchema, {
  key: 'feature:warm-hands@mesa',
  namePt: 'Mãos mornas',
  summaryPt: '+1 de dano de fogo.',
  change: 'bonus_damage',
  alwaysOn: true,
  allowed: true,
});
const spent = create(SpellTableOptionSchema, {
  key: 'feature:spare@mesa',
  namePt: 'Reserva',
  change: 'bonus_damage',
  resource: 'reserva',
  usesLeft: 0,
  allowed: false,
  disabledReasonPt: 'Sem usos: volta num descanso longo.',
});
const all = [always, swap, spent];

describe('table cast options', () => {
  it('names a damage type, and keeps the key of one it does not know', () => {
    expect(damageTypeName('damage-type:fire')).toBe('Fogo');
    expect(damageTypeName('damage-type:cold')).toBe('Frio');
    expect(damageTypeName('damage-type:mystery')).toBe('damage-type:mystery');
  });

  it('lists an always-on option as told, a swap with its types and cost, and a refused one with the reason', () => {
    const rows = tableOptionRows(all, [], {});
    expect(rows[0]).toMatchObject({ alwaysOn: true, checked: true, blocked: '', cost: '' });
    expect(rows[1]).toMatchObject({
      alwaysOn: false,
      checked: false,
      cost: 'Gasta 1 uso · restam 2',
      type: 'damage-type:cold',
    });
    expect(rows[1].types.map((t) => t.name)).toEqual(['Frio', 'Fogo']);
    expect(rows[2]).toMatchObject({ blocked: 'Sem usos: volta num descanso longo.' });
    expect(rows[2].cost).toBe('Gasta 1 uso · restam 0');
  });

  it('sends only the ticked options that may be ticked, a swap with its picked type', () => {
    expect(tableOptionPicks(all, [], {})).toEqual([]);
    expect(tableOptionPicks(all, [swap.key], {})).toEqual([
      { key: swap.key, damageTypeKey: 'damage-type:cold' },
    ]);
    expect(
      tableOptionPicks(all, [swap.key, spent.key, always.key], { [swap.key]: 'damage-type:fire' }),
    ).toEqual([{ key: swap.key, damageTypeKey: 'damage-type:fire' }]);
    // A type that is not one of the option's falls back to its first.
    expect(tableOptionPicks(all, [swap.key], { [swap.key]: 'damage-type:acid' })).toEqual([
      { key: swap.key, damageTypeKey: 'damage-type:cold' },
    ]);
  });

  it('toggles a key', () => {
    expect(toggledTable([], 'a')).toEqual(['a']);
    expect(toggledTable(['a', 'b'], 'a')).toEqual(['b']);
  });
});
