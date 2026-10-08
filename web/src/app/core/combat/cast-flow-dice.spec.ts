import { create } from '@bufbuild/protobuf';
import {
  ActionEconomy,
  SpellDamageChoice,
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

describe('the cast sheet subtitle of a spell with several damage types', () => {
  const flameStrike = (damageChoice: SpellDamageChoice) =>
    create(SpellDetailsSchema, {
      spell: { key: 'spell:flame-strike', level: 5 },
      damageChoice,
      damage: [
        create(SpellDamageSchema, {
          damageTypeKey: 'damage-type:fire',
          damageTypePt: 'fogo',
          bySlotLevel: { 5: '4d6', 6: '5d6' },
        }),
        create(SpellDamageSchema, {
          damageTypeKey: 'damage-type:radiant',
          damageTypePt: 'radiante',
          bySlotLevel: { 5: '4d6', 6: '5d6' },
        }),
      ],
    });
  const subtitle = (d: ReturnType<typeof flameStrike>, slot: number, pick: string) =>
    castSubtitle(ActionEconomy.ACTION, 'save', d, slot, 0, '', pick);

  it('gives the higher slot’s dice to the picked type and leaves the other at the spell’s own circle', () => {
    const d = flameStrike(SpellDamageChoice.SCALE);
    expect(subtitle(d, 6, 'damage-type:fire')).toContain('5d6 de fogo e 4d6 de radiante');
    expect(subtitle(d, 6, 'damage-type:radiant')).toContain('4d6 de fogo e 5d6 de radiante');
    expect(subtitle(d, 6, '')).toContain('5d6 de fogo e 4d6 de radiante');
    expect(subtitle(d, 5, 'damage-type:radiant')).toContain('4d6 de fogo e 4d6 de radiante');
  });

  it('says only the picked type when the spell deals just that one', () => {
    const d = flameStrike(SpellDamageChoice.ALTERNATIVE);
    expect(subtitle(d, 5, 'damage-type:radiant')).toContain('4d6 de radiante');
    expect(subtitle(d, 5, 'damage-type:radiant')).not.toContain('fogo');
  });

  it('says every type of a spell that deals them all', () => {
    const d = flameStrike(SpellDamageChoice.UNSPECIFIED);
    expect(subtitle(d, 6, '')).toContain('5d6 de fogo e 5d6 de radiante');
  });
});
