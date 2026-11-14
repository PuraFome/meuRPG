import { create } from '@bufbuild/protobuf';

import { TableEffectSchema } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { rollEffectItem } from './content-read-rolls';
import {
  draftToEffect,
  effectToDraft,
  emptyRow,
  featureAuraDraft,
  rowToModifiers,
} from './spell-effect-draft';
import { catalog } from './content-testing';

const cat = catalog();
const nameOf = (k: string) => cat.nameOf(k);

function auraOf(radiusM: string, mode: 'fixed' | 'ability') {
  const d = {
    ...featureAuraDraft(),
    aura: { radiusM, affects: 'allies' as const },
    rows: [
      {
        ...emptyRow('roll'),
        rollMode: mode,
        value: '2',
        rolls: ['save' as const],
        valueAbility: 'cha',
        valueMin: '1',
      },
    ],
  };
  return draftToEffect(d)!;
}

describe('a feature aura (#579)', () => {
  it('reads as one line with the radius, who it reaches and the bonus', () => {
    const e = create(TableEffectSchema, { type: 'aura', aura: auraOf('3', 'ability') });
    expect(rollEffectItem(e, nameOf)).toEqual({
      title: 'Aura',
      text: 'Aura de 3 m: aliados, Modificador de CAR (mín. +1) em resistências, em todo o combate.',
    });
  });

  it('reads a fixed bonus the same way', () => {
    const e = create(TableEffectSchema, { type: 'aura', aura: auraOf('9', 'fixed') });
    expect(rollEffectItem(e, nameOf)?.text).toBe(
      'Aura de 9 m: aliados, +2 em resistências, em todo o combate.',
    );
  });

  it('says nothing while the aura has no bonus yet', () => {
    const d = featureAuraDraft();
    expect(
      rollEffectItem(create(TableEffectSchema, { type: 'aura', aura: draftToEffect(d) }), nameOf),
    ).toBeNull();
  });

  it('sends the ability and the least, and reads them back as a roll row', () => {
    const [m] = rowToModifiers({ ...emptyRow('roll'), rollMode: 'ability', valueMin: '2' });
    expect(m.valueAbility).toBe('cha');
    expect(m.valueMin).toBe(2);
    expect(m.value).toBe(0);
    const back = effectToDraft(auraOf('3', 'ability')).rows[0];
    expect(back).toMatchObject({ rollMode: 'ability', valueAbility: 'cha', valueMin: '1' });
  });
});
