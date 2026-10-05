import { create } from '@bufbuild/protobuf';

import {
  CharacterSpellSchema,
  DerivedClassSchema,
  DerivedSheetSchema,
  FeatureSchema,
  SpellSchema,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { creatureAccess } from './summon-access';
import { beastOptions, summonSpell, undeadCount } from './summon-spells';

function sheet(opts: { wizard?: boolean; spells?: { key: string; prepared: boolean; ritual?: boolean }[]; features?: string[] }) {
  return create(DerivedSheetSchema, {
    classes: opts.wizard ? [create(DerivedClassSchema, { classKey: 'class:wizard', level: 4 })] : [],
    spells: (opts.spells ?? []).map((s) => create(CharacterSpellSchema, { prepared: s.prepared, spell: create(SpellSchema, { key: s.key, ritual: s.ritual ?? false }) })),
    features: (opts.features ?? []).map((key) => create(FeatureSchema, { key })),
  });
}

describe('the summoning spells the sheet offers (MR-037)', () => {
  it('knows the three spells and their casting times', () => {
    expect(summonSpell('spell:find-familiar')?.time).toBe('1 hora');
    expect(summonSpell('spell:animate-dead')?.time).toBe('1 minuto');
    expect(summonSpell('spell:conjure-animals')?.time).toBe('1 ação');
    expect(summonSpell('spell:find-familiar')?.forms).toHaveLength(15);
    expect(summonSpell('spell:fire-bolt')).toBeUndefined();
  });

  it('Animar os Mortos raises 1 undead with a 3rd circle slot and 2 more for each circle above', () => {
    expect([3, 4, 5, 9].map(undeadCount)).toEqual([1, 3, 5, 13]);
  });

  it("Conjurar Animais' counts follow the slot: x2 at the 5th, x3 at the 7th, x4 at the 9th", () => {
    expect(beastOptions(3).map((o) => o.count)).toEqual([1, 2, 4, 8]);
    expect(beastOptions(5).map((o) => o.count)).toEqual([2, 4, 8, 16]);
    expect(beastOptions(7).map((o) => o.count)).toEqual([3, 6, 12, 24]);
    expect(beastOptions(9).map((o) => o.count)).toEqual([4, 8, 16, 32]);
    expect(beastOptions(3).map((o) => o.maxCr)).toEqual(['2', '1', '1/2', '1/4']);
  });

  it('a wizard with Encontrar Familiar in the book casts it as a ritual, even unprepared, and with a slot only when prepared', () => {
    const ritualOnly = creatureAccess(sheet({ wizard: true, spells: [{ key: 'spell:find-familiar', prepared: false, ritual: true }] }));
    expect(ritualOnly.casts).toEqual([expect.objectContaining({ key: 'spell:find-familiar', ritual: true, slot: false })]);
    const both = creatureAccess(sheet({ wizard: true, spells: [{ key: 'spell:find-familiar', prepared: true, ritual: true }] }));
    expect(both.casts[0]).toMatchObject({ ritual: true, slot: true, chain: false });
  });

  it('another caster casts the ritual only when prepared; an unprepared non-ritual spell is not offered', () => {
    const none = creatureAccess(sheet({ spells: [{ key: 'spell:find-familiar', prepared: false, ritual: true }, { key: 'spell:animate-dead', prepared: false }] }));
    expect(none.casts).toEqual([]);
    const ready = creatureAccess(sheet({ spells: [{ key: 'spell:animate-dead', prepared: true }, { key: 'spell:conjure-animals', prepared: true }] }));
    expect(ready.casts.map((c) => [c.key, c.ritual, c.slot])).toEqual([
      ['spell:animate-dead', false, true],
      ['spell:conjure-animals', false, true],
    ]);
  });

  it("the Pact of the Chain gives a warlock the familiar as a ritual with no spell, and the chain's forms", () => {
    const access = creatureAccess(sheet({ features: ['feature:pact-of-the-chain'] }));
    expect(access.casts).toEqual([expect.objectContaining({ key: 'spell:find-familiar', ritual: true, slot: false, chain: true })]);
  });

  it('a druid with Wild Shape has the panel even without a creature', () => {
    expect(creatureAccess(sheet({ features: ['feature:wild-shape-cr-1-4-or-below-no-flying-or-swim-speed'] })).wildShape).toBe(true);
    expect(creatureAccess(sheet({})).wildShape).toBe(false);
  });
});
