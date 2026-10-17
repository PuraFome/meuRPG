import { create } from '@bufbuild/protobuf';

import { TableSpellSchema } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { draftToSpell, spellToDraft } from './spell-draft';
import { UNAPPLIED_DAMAGE_PT, unappliedDamage } from './spell-warnings';

const base = {
  namePt: 'Raio de Nanquim',
  level: 1,
  schoolKey: 'school:evocation',
  castingTime: { unit: 1, amount: 1 },
  range: { kind: 3, distanceFt: 60 },
  duration: { kind: 1 },
  target: { kind: 1 },
  damage: [{ damageTypeKey: 'damage-type:fire', dice: '2d6', perSlotLevel: '1d6' }],
};
const draftOf = (init: object) => spellToDraft(create(TableSpellSchema, { ...base, ...init }));

describe('unappliedDamage', () => {
  it('warns for damage with no attack, no saving throw and no area', () => {
    expect(unappliedDamage(draftOf({}))).toBe(true);
    expect(unappliedDamage(draftOf({ target: { kind: 2, count: 2 } }))).toBe(true);
  });

  it('is quiet when the spell has an attack, a saving throw or an area', () => {
    expect(unappliedDamage(draftOf({ attack: 'ranged' }))).toBe(false);
    expect(unappliedDamage(draftOf({ save: { ability: 2, onSuccess: 1 } }))).toBe(false);
    expect(unappliedDamage(draftOf({ target: { kind: 3, shape: 5, sizeFt: 20 } }))).toBe(false);
  });

  it('is quiet for a spell on the caster alone and for a spell with no damage', () => {
    expect(unappliedDamage(draftOf({ target: { kind: 4 }, range: { kind: 2 } }))).toBe(false);
    expect(unappliedDamage(draftOf({ damage: [] }))).toBe(false);
  });

  it('says what is wrong and how to fix it, in Portuguese', () => {
    expect(UNAPPLIED_DAMAGE_PT).toMatch(/^O app não sabe aplicar este dano/);
    expect(UNAPPLIED_DAMAGE_PT).toContain('ataque');
    expect(UNAPPLIED_DAMAGE_PT).toContain('teste de resistência');
    expect(UNAPPLIED_DAMAGE_PT).toContain('área');
  });

  it('only warns: the spell still builds to be saved', () => {
    expect(draftToSpell(draftOf({})).namePt).toBe('Raio de Nanquim');
  });
});
