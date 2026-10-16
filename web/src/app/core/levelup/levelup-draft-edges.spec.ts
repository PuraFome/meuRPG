import { create } from '@bufbuild/protobuf';

import {
  ChoiceGroupSchema,
  ChoiceKind,
  ChoiceOptionSchema,
  ChoiceSchema,
  LevelUpFeatureChoiceSchema,
  LevelUpHitPointsMethod,
  LevelUpHitPointsRule,
  LevelUpProficiencyKind,
  LevelUpSubclassSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { Ability } from '../../../gen/meurpg/rules/v1/rules_pb';
import { LevelUpDraft } from './levelup-draft';
import {
  SKILLS,
  SPELLS,
  WIZARD_KEYS,
  featOptions,
  fighterOptions,
  wizardOptions,
} from './levelup-testing';

const catalog = { spells: SPELLS, skills: SKILLS };

const style = create(ChoiceSchema, {
  key: 'style',
  labelPt: 'Estilo',
  kind: ChoiceKind.OPTIONS,
  picks: 1,
  missing: 1,
  options: [create(ChoiceOptionSchema, { key: 'duel', storedKey: 'stored:duel', namePt: 'Duelo' })],
});
const terrain = create(ChoiceSchema, {
  key: 'terrain',
  labelPt: 'Terreno',
  kind: ChoiceKind.OPTIONS,
  picks: 1,
  missing: 1,
  options: [
    create(ChoiceOptionSchema, { key: 'forest', storedKey: 'stored:forest', namePt: 'Floresta' }),
  ],
});
const lore = create(LevelUpSubclassSchema, {
  key: 'sub:lore',
  namePt: 'Colégio do Conhecimento',
  skillChoices: 1,
  featureChoices: [
    create(LevelUpFeatureChoiceSchema, {
      feature: { key: 'feature:x', namePt: 'Estilo' },
      choose: 1,
      options: [{ key: 'o:a', namePt: 'A' }],
    }),
  ],
});

/** A wizard that has every kind of pick there is: feats, a subclass, open choices and an instrument. */
function rich(): LevelUpDraft {
  return new LevelUpDraft(
    wizardOptions({
      preparedMaxAfter: 3,
      expertiseChoices: 1,
      feats: featOptions(),
      subclassDue: true,
      subclasses: [lore],
      lateChoices: [create(ChoiceGroupSchema, { choices: [style] })],
      newChoices: [create(ChoiceGroupSchema, { choices: [terrain] })],
      proficiencyChoices: [
        {
          kind: LevelUpProficiencyKind.INSTRUMENT,
          count: 1,
          from: [
            { key: 'proficiency:lute', namePt: 'Alaúde', alreadyHave: false },
            { key: 'proficiency:drum', namePt: 'Tambor', alreadyHave: true },
          ],
        },
      ],
    }),
    WIZARD_KEYS,
    { ...catalog, names: new Map([['extra:key', 'Extra']]) },
  );
}

describe('LevelUpDraft: a screen that still calls the draft that was replaced', () => {
  it('sends every setter on to the newest draft and leaves the old one as it was', () => {
    const old = rich();
    const fresh = rich();
    fresh.adopt(old);
    old.handOver(fresh);

    old.setAbilityMode('two');
    expect(fresh.abilityMode()).toBe('two');
    old.toggleAbility('int');
    expect(fresh.abilityKeys()).toEqual(['int']);
    old.selectLate(style, ['duel']);
    expect(fresh.lateDrafts().get('style')?.optionKeys).toEqual(['duel']);
    old.editLateText(style, 1, 'orcs');
    expect(fresh.lateDrafts().get('style')?.texts).toEqual(['orcs', '']);
    old.selectNew(terrain, ['forest']);
    expect(fresh.newDrafts().get('terrain')?.optionKeys).toEqual(['forest']);
    old.editNewText(terrain, 1, 'elfos');
    expect(fresh.newDrafts().get('terrain')?.texts[0]).toBe('elfos');
    old.setAsiMode('feat');
    expect(fresh.asiMode()).toBe('feat');
    old.setFeat('feat:atleta@mesa');
    expect(fresh.featKey()).toBe('feat:atleta@mesa');
    old.toggleFeatAbility(Ability.DEXTERITY);
    expect(fresh.featAbilities()).toEqual([Ability.DEXTERITY]);
    old.setHpCard('roll');
    expect(fresh.hpCard()).toBe('roll');
    old.setSubclass('sub:lore');
    expect(fresh.subclassKey()).toBe('sub:lore');
    old.toggleFeature('o:a', ['o:a'], 1);
    expect([...fresh.features()]).toEqual(['o:a']);
    old.toggleCantrip('spell:prestidigitation');
    expect([...fresh.cantrips()]).toEqual(['spell:prestidigitation']);
    old.toggleSpell('spell:misty-step');
    expect([...fresh.spells()]).toEqual(['spell:misty-step']);
    old.togglePrepared('spell:misty-step');
    expect([...fresh.prepared()]).toEqual(['spell:misty-step']);
    old.toggleSkill('skill:stealth');
    expect([...fresh.skills()]).toEqual(['skill:stealth']);
    old.toggleExpertise('skill:stealth');
    expect([...fresh.expertise()]).toEqual(['skill:stealth']);
    old.toggleInstrument('proficiency:lute');
    expect(fresh.instrument()).toBe('proficiency:lute');

    expect(old.dirty()).toBe(false);
    expect(old.subclassKey()).toBe('');
    expect(old.abilityKeys()).toEqual([]);
  });
});

describe('LevelUpDraft: what the screen reads', () => {
  it('names every key the screen may show, from the catalog, the lists, the subclasses and the features', () => {
    const d = rich();
    d.setSubclass('sub:lore');
    const names = d.names();
    expect(names.get('extra:key')).toBe('Extra');
    expect(names.get('spell:fire-bolt')).toBe('Raio de Fogo');
    expect(names.get('skill:arcana')).toBe('Arcanismo');
    expect(names.get('sub:lore')).toBe('Colégio do Conhecimento');
    expect(names.get('feature:x')).toBe('Estilo');
    expect(names.get('o:a')).toBe('A');
  });

  it('names a feature choice without a feature as an empty key', () => {
    const d = new LevelUpDraft(
      fighterOptions({
        subclassDue: true,
        subclasses: [
          create(LevelUpSubclassSchema, {
            key: 'sub:x',
            namePt: 'X',
            featureChoices: [
              create(LevelUpFeatureChoiceSchema, {
                choose: 1,
                options: [{ key: 'o:z', namePt: 'Z' }],
              }),
            ],
          }),
        ],
      }),
      WIZARD_KEYS,
      catalog,
    );
    d.setSubclass('sub:x');
    expect(d.names().get('')).toBe('');
    expect(d.names().get('o:z')).toBe('Z');
    expect(d.missing().find((m) => m.id === 'feature-0')?.text).toBe(
      'Falta escolher 1 opção de característica.',
    );
  });

  it('names the class whose list the new spells come from, else the class itself', () => {
    const own = new LevelUpDraft(wizardOptions(), WIZARD_KEYS, {
      ...catalog,
      classes: [{ key: 'class:wizard', namePt: 'Mago' }],
    });
    expect(own.listName()).toBe('Mago');
    const none = new LevelUpDraft(wizardOptions(), WIZARD_KEYS, catalog);
    expect(none.listName()).toBe('Mago');
    const other = new LevelUpDraft(
      wizardOptions({ spellListClassKey: 'class:cleric' }),
      WIZARD_KEYS,
      {
        ...catalog,
        classes: [{ key: 'class:cleric', namePt: 'Clérigo' }],
      },
    );
    expect(other.listName()).toBe('Clérigo');
  });

  it('starts a table that makes everybody roll on the die, and the rest on the average', () => {
    const roll = new LevelUpDraft(
      fighterOptions({ hitPointsRule: LevelUpHitPointsRule.ROLL_ONLY }),
      WIZARD_KEYS,
      catalog,
    );
    expect(roll.hpCard()).toBe('roll');
    expect(roll.dirty()).toBe(false);
    expect(roll.missing().map((m) => m.id)).toEqual(['hp']);
    expect(new LevelUpDraft(fighterOptions(), WIZARD_KEYS, catalog).hpCard()).toBe('average');
  });

  it('sends the die as rolled in the app or typed from a physical one', () => {
    const d = new LevelUpDraft(fighterOptions(), WIZARD_KEYS, catalog);
    d.setHpCard('roll');
    d.rolled.set({ kind: 'app', value: 7 });
    expect(d.choices().hitPoints).toEqual({
      method: LevelUpHitPointsMethod.ROLLED_IN_APP,
      value: 7,
    });
    d.rolled.set({ kind: 'physical', value: 5 });
    expect(d.choices().hitPoints).toEqual({
      method: LevelUpHitPointsMethod.ROLLED_PHYSICAL,
      value: 5,
    });
    expect(d.averageChoices().hitPoints).toEqual({ method: LevelUpHitPointsMethod.AVERAGE });
  });

  it('turns off the spells from outside the class list once the cap is reached', () => {
    const d = new LevelUpDraft(wizardOptions({ spells: 3, anyClassSpells: 1 }), WIZARD_KEYS, {
      spells: SPELLS.map((s) =>
        s.key === 'spell:cure-wounds' || s.key === 'spell:thunderwave'
          ? { ...s, classKeys: ['class:cleric'] }
          : s,
      ) as never,
      skills: SKILLS,
    });
    expect(d.outsideMax()).toBe(1);
    const outside = d.spellItems().filter((i) => i.outside);
    expect(outside.length).toBeGreaterThan(1);
    expect(outside.every((i) => !i.disabled)).toBe(true);
    d.toggleSpell(outside[0].key);
    expect(d.outsidePicked()).toBe(1);
    const after = d.spellItems().filter((i) => i.outside);
    expect(after.find((i) => i.key === outside[0].key)?.disabled).toBeUndefined();
    expect(after.find((i) => i.key === outside[1].key)?.disabled).toBe(
      'Limite de 1 de outra classe',
    );
    // A turned off spell cannot be picked.
    d.toggleSpell(outside[1].key);
    expect(d.spells().has(outside[1].key)).toBe(false);
  });

  it('asks for the feat abilities, names the feat and sends the increase', () => {
    const d = rich();
    d.setAsiMode('feat');
    d.setFeat('feat:atleta@mesa');
    expect(d.featSummary()).toEqual({ name: 'Atleta', increase: '' });
    d.toggleFeatAbility(Ability.STRENGTH);
    expect(d.featSummary()).toEqual({ name: 'Atleta', increase: '+1 Força' });
    expect(d.pickedKeys()).toEqual(['str']);
    d.setFeat('feat:grappler');
    expect(d.featSummary()).toEqual({ name: 'Agarrador', increase: '' });
    expect(d.pickedKeys()).toEqual([]);
    // A feat that is not on the list picks nothing.
    d.toggleFeatAbility(Ability.STRENGTH);
    expect(d.featAbilities()).toEqual([]);
    d.setFeat('feat:atleta@mesa');
    // An ability outside the feat's list is refused.
    d.toggleFeatAbility(Ability.WISDOM);
    expect(d.featAbilities()).toEqual([]);
  });

  it('picks two abilities for a feat that asks two and drops one that is picked again', () => {
    const feat = featOptions()[1];
    feat.increase = {
      ...feat.increase!,
      count: 2,
      from: [Ability.STRENGTH, Ability.DEXTERITY, Ability.CONSTITUTION],
    } as never;
    const d = new LevelUpDraft(wizardOptions({ feats: [feat] }), WIZARD_KEYS, catalog);
    d.setAsiMode('feat');
    d.setFeat(feat.key);
    d.toggleFeatAbility(Ability.STRENGTH);
    d.toggleFeatAbility(Ability.DEXTERITY);
    d.toggleFeatAbility(Ability.CONSTITUTION);
    expect(d.featAbilities()).toEqual([Ability.STRENGTH, Ability.DEXTERITY]);
    d.toggleFeatAbility(Ability.STRENGTH);
    expect(d.featAbilities()).toEqual([Ability.DEXTERITY]);
    expect(d.missingIn('abilities').map((m) => m.text)).toEqual(['Falta escolher 1 habilidade.']);
    expect(d.choices().abilityIncrease).toEqual({});
  });

  it('refuses a feat mode the level has no feats for, and keeps the same mode', () => {
    const d = rich();
    d.setAsiMode('increase');
    expect(d.asiMode()).toBe('increase');
    const bare = new LevelUpDraft(wizardOptions(), WIZARD_KEYS, catalog);
    bare.setAsiMode('feat');
    expect(bare.asiMode()).toBe('increase');
  });

  it('clears the ability picks when the mode of the increase changes, and keeps them when it does not', () => {
    const d = rich();
    d.toggleAbility('int');
    d.setAbilityMode('one');
    expect(d.abilityKeys()).toEqual(['int']);
    d.toggleAbility('int');
    expect(d.abilityKeys()).toEqual([]);
    d.toggleAbility('wis');
    d.setAbilityMode('two');
    expect(d.abilityKeys()).toEqual([]);
  });

  it('does nothing when the subclass picked again is the one already picked', () => {
    const d = rich();
    d.setSubclass('sub:lore');
    d.toggleFeature('o:a', ['o:a'], 1);
    d.setSubclass('sub:lore');
    expect([...d.features()]).toEqual(['o:a']);
  });

  it('picks one option of a group in place of another when the feature takes one', () => {
    const d = rich();
    const group = ['o:a', 'o:b'];
    d.toggleFeature('o:a', group, 1);
    d.toggleFeature('o:b', group, 1);
    expect([...d.features()]).toEqual(['o:b']);
    d.toggleFeature('o:b', group, 1);
    expect([...d.features()]).toEqual([]);
  });
});

describe('LevelUpDraft: what a re-read of the sheet keeps', () => {
  it('keeps the instrument and the subclass when these options still offer them', () => {
    const old = rich();
    old.toggleInstrument('proficiency:lute');
    old.setSubclass('sub:lore');
    const fresh = rich();
    fresh.adopt(old);
    expect(fresh.instrument()).toBe('proficiency:lute');
    expect(fresh.subclassKey()).toBe('sub:lore');
    const bare = new LevelUpDraft(wizardOptions(), WIZARD_KEYS, catalog);
    bare.adopt(old);
    expect(bare.instrument()).toBe('');
    expect(bare.subclassKey()).toBe('');
  });

  it('keeps the card of the player when the table lets him choose', () => {
    const old = new LevelUpDraft(fighterOptions(), WIZARD_KEYS, catalog);
    old.setHpCard('roll');
    const fresh = new LevelUpDraft(fighterOptions(), WIZARD_KEYS, catalog);
    fresh.adopt(old);
    expect(fresh.hpCard()).toBe('roll');
    expect(fresh.rolled()).toBeNull();
  });

  it('keeps an in-app roll when the server kept it for a class that was not named', () => {
    const old = new LevelUpDraft(
      fighterOptions({ keptHitPointRoll: 6, keptHitPointRollClassKey: '' }),
      WIZARD_KEYS,
      catalog,
    );
    old.setHpCard('roll');
    old.rolled.set({ kind: 'app', value: 6 });
    const fresh = new LevelUpDraft(
      fighterOptions({ keptHitPointRoll: 6, keptHitPointRollClassKey: '' }),
      WIZARD_KEYS,
      catalog,
    );
    fresh.adopt(old);
    expect(fresh.rolled()).toEqual({ kind: 'app', value: 6 });
    const other = new LevelUpDraft(
      fighterOptions({ keptHitPointRoll: 6, keptHitPointRollClassKey: 'class:rogue' }),
      WIZARD_KEYS,
      catalog,
    );
    other.adopt(old);
    expect(other.rolled()).toBeNull();
  });

  it('keeps a typed roll that is a whole number inside the die, and drops one outside it', () => {
    const old = new LevelUpDraft(fighterOptions(), WIZARD_KEYS, catalog);
    old.setHpCard('roll');
    for (const [value, kept] of [
      [10, true],
      [11, false],
      [0, false],
      [2.5, false],
    ] as const) {
      old.rolled.set({ kind: 'physical', value });
      const fresh = new LevelUpDraft(fighterOptions(), WIZARD_KEYS, catalog);
      fresh.adopt(old);
      expect(fresh.rolled() !== null).toBe(kept);
    }
  });

  it('keeps the ability picks of a level with an increase, and none of a level without', () => {
    const old = rich();
    old.toggleAbility('int');
    old.setAbilityMode('one');
    const fresh = rich();
    fresh.adopt(old);
    expect(fresh.abilityKeys()).toEqual(['int']);
    const plain = new LevelUpDraft(fighterOptions(), WIZARD_KEYS, catalog);
    plain.adopt(old);
    expect(plain.abilityKeys()).toEqual([]);
  });
});

describe('LevelUpDraft: everything that can be missing, in step order', () => {
  it('lists every kind of missing pick with its step and its words', () => {
    const d = rich();
    expect(d.missing().map((m) => [m.step, m.id, m.text])).toEqual([
      ['abilities', 'abilities', 'Falta escolher 1 habilidade.'],
      ['picks', 'late', 'Falta escolher 1 escolha que ficou para trás.'],
      ['picks', 'new', 'Falta escolher 1 escolha do nível.'],
      ['picks', 'subclass', 'Falta escolher a subclasse.'],
      ['picks', 'instrument', 'Falta escolher o instrumento.'],
      ['picks', 'expertise', 'Falta escolher 1 especialização.'],
      ['spells', 'cantrips', 'Falta escolher 1 truque.'],
      ['spells', 'spells', 'Faltam escolher 2 magias para o livro.'],
    ]);
    d.setSubclass('sub:lore');
    expect(d.missing().map((m) => [m.id, m.text])).toContainEqual([
      'feature-0',
      'Falta escolher 1 opção de Estilo.',
    ]);
    expect(d.missing().map((m) => m.id)).toContain('skills');
  });

  it('asks for the die to be rolled while the roll card is open with no result', () => {
    const d = rich();
    d.setHpCard('roll');
    expect(d.missingIn('hp').map((m) => m.text)).toEqual(['Falta rolar o dado de vida.']);
    d.rolled.set({ kind: 'app', value: 4 });
    expect(d.missingIn('hp')).toEqual([]);
  });

  it('asks for the abilities of a feat, the count of them that are still free', () => {
    const d = rich();
    d.setAsiMode('feat');
    expect(d.missingIn('abilities').map((m) => m.text)).toEqual(['Falta escolher o talento.']);
    d.setFeat('feat:atleta@mesa');
    expect(d.missingIn('abilities').map((m) => m.text)).toEqual(['Falta escolher 1 habilidade.']);
  });

  it('says the prepared spells that are missing, and the spellbook words for the known ones', () => {
    const d = new LevelUpDraft(wizardOptions({ preparedMaxAfter: 3 }), WIZARD_KEYS, catalog);
    d.toggleSpell('spell:misty-step');
    d.toggleSpell('spell:mirror-image');
    expect(d.missingIn('spells').map((m) => [m.id, m.text])).toContainEqual([
      'prepared',
      'Falta preparar 1 magia.',
    ]);
  });
});
