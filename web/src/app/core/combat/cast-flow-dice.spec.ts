import { create } from '@bufbuild/protobuf';
import {
  ActionEconomy,
  SpellDamageSchema,
  SpellDetailsSchema,
  SpellRangeKind,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { castSubtitle, damageDice } from './cast-flow';
import { spellSummary } from './spell-summary';

// Sacred Flame (save cantrip): 1d8 at 1st, 2d8 at 5th, 3d8 at 11th and 4d8 at 17th
// character level. The details carry the whole table; the server sends the caster's
// row with the attack (`Attack.spellDice`).
function sacredFlame() {
  return create(SpellDetailsSchema, {
    spell: { key: 'spell:sacred-flame', level: 0 },
    range: { kind: SpellRangeKind.RANGED, distanceFt: 60 },
    damage: [
      create(SpellDamageSchema, {
        damageTypePt: 'radiante',
        bySlotLevel: {},
        byCharacterLevel: { 1: '1d8', 5: '2d8', 11: '3d8', 17: '4d8' },
      }),
    ],
  });
}

describe('a cantrip’s dice', () => {
  it('shows the caster’s dice in the action list', () => {
    expect(spellSummary(sacredFlame(), '3d8')).toContain('3d8 de radiante');
  });

  it('never pins the first row of the table in the action list', () => {
    expect(spellSummary(sacredFlame())).not.toContain('1d8');
  });

  it('shows the caster’s dice under the name on the cast sheet', () => {
    const sub = castSubtitle(ActionEconomy.ACTION, 'save', sacredFlame(), 0, 0, '3d8');
    expect(sub).toContain('3d8 de radiante');
    expect(sub).not.toContain('1d8');
  });

  it('shows no dice on the cast sheet until the caster’s are known', () => {
    expect(castSubtitle(ActionEconomy.ACTION, 'save', sacredFlame(), 0, 0)).not.toContain('d8');
  });

  it('takes the caster’s dice for a cantrip, not the first row of the table', () => {
    expect(damageDice(sacredFlame(), 0, '3d8')).toBe('3d8');
    expect(damageDice(sacredFlame(), 0)).toBe('');
  });

  it('keeps the dice of the slot for a leveled spell', () => {
    const fireball = create(SpellDetailsSchema, {
      spell: { key: 'spell:fireball', level: 3 },
      damage: [
        create(SpellDamageSchema, { damageTypePt: 'fogo', bySlotLevel: { 3: '8d6', 4: '9d6' } }),
      ],
    });
    expect(damageDice(fireball, 4)).toBe('9d6');
    expect(damageDice(fireball, 3)).toBe('8d6');
  });
});
