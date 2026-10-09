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
// Two are prepared today and the new maximum is 3: one more to prepare.
const wizard = () => new LevelUpDraft(wizardOptions({ preparedMaxAfter: 3 }), WIZARD_KEYS, catalog);

describe('LevelUpDraft: what is missing', () => {
  it('asks for the ability, the cantrip, the two spells and the two to prepare, in step order', () => {
    const d = wizard();
    expect(d.missing().map((m) => [m.step, m.id, m.text])).toEqual([
      ['abilities', 'abilities', 'Falta escolher 1 habilidade.'],
      ['spells', 'cantrips', 'Falta escolher 1 truque.'],
      ['spells', 'spells', 'Faltam escolher 2 magias para o livro.'],
    ]);
    // One more prepared spell would need a spell in the book beyond the two that are prepared.
    d.toggleSpell('spell:misty-step');
    d.toggleSpell('spell:mirror-image');
    expect(d.missingIn('spells').map((m) => m.id)).toEqual(['cantrips', 'prepared']);
  });

  it('is complete after every choice, and the choices go in the server shape', () => {
    const d = wizard();
    d.toggleAbility('int');
    d.toggleCantrip('spell:prestidigitation');
    d.toggleSpell('spell:misty-step');
    d.toggleSpell('spell:mirror-image');
    d.togglePrepared('spell:misty-step');
    expect(d.missing()).toEqual([]);
    expect(d.choices()).toMatchObject({
      classKey: 'class:wizard',
      abilityIncrease: { intelligence: 2 },
      cantripKeys: ['spell:prestidigitation'],
      knownSpellKeys: ['spell:misty-step', 'spell:mirror-image'],
      preparedSpellKeys: ['spell:misty-step'],
      hitPoints: { method: LevelUpHitPointsMethod.AVERAGE },
    });
  });

  it('asks what the server asks, even from a list that has fewer rows, and says so instead of letting it through', () => {
    const few = new LevelUpDraft(
      wizardOptions({ cantrips: 5, preparedMaxAfter: 3 }),
      WIZARD_KEYS,
      catalog,
    );
    expect(few.cantripsAsked()).toBe(5);
    few.toggleAbility('int');
    for (const item of few.cantripItems()) few.toggleCantrip(item.key);
    const text = few.missing().find((m) => m.id === 'cantrips')?.text;
    expect(text).toContain('A lista só traz 2 truques e o nível pede 5');
  });
});

describe('LevelUpDraft: ability increase', () => {
  it('is +2 in one, or +1 in two (a third replaces the oldest)', () => {
    const d = wizard();
    d.toggleAbility('int');
    d.toggleAbility('wis');
    expect(d.abilityKeys()).toEqual(['wis']);
    d.setAbilityMode('two');
    expect(d.abilityKeys()).toEqual([]);
    d.toggleAbility('int');
    d.toggleAbility('wis');
    d.toggleAbility('cha');
    expect(d.abilityKeys()).toEqual(['wis', 'cha']);
    expect(d.choices()).toMatchObject({ abilityIncrease: { wisdom: 1, charisma: 1 } });
  });

  it('sends nothing for an incomplete increase', () => {
    const d = wizard();
    d.setAbilityMode('two');
    d.toggleAbility('int');
    expect(d.abilityIncrease()).toEqual({});
  });
});

describe('LevelUpDraft: hit points', () => {
  it('is the average until a die is chosen, and the die is missing until it is rolled', () => {
    const d = wizard();
    d.toggleAbility('int');
    expect(d.missingIn('hp')).toEqual([]);
    d.setHpCard('roll');
    expect(d.missingIn('hp').map((m) => m.text)).toEqual(['Falta rolar o dado de vida.']);
    // Previewed with the average until the die is rolled.
    expect(d.choices()).toMatchObject({ hitPoints: { method: LevelUpHitPointsMethod.AVERAGE } });
    d.rolled.set({ kind: 'app', value: 5 });
    expect(d.missingIn('hp')).toEqual([]);
    expect(d.choices()).toMatchObject({
      hitPoints: { method: LevelUpHitPointsMethod.ROLLED_IN_APP, value: 5 },
    });
    d.rolled.set({ kind: 'physical', value: 3 });
    expect(d.choices()).toMatchObject({
      hitPoints: { method: LevelUpHitPointsMethod.ROLLED_PHYSICAL, value: 3 },
    });
    // The average card still gives its own choices, whichever card is open.
    expect(d.averageChoices()).toMatchObject({
      hitPoints: { method: LevelUpHitPointsMethod.AVERAGE },
    });
  });
});

describe('LevelUpDraft: lists', () => {
  it('replaces the pick when there is one place, and refuses a full list', () => {
    const d = wizard();
    d.toggleCantrip('spell:prestidigitation');
    d.toggleCantrip('spell:shocking-grasp');
    expect([...d.cantrips()]).toEqual(['spell:shocking-grasp']);
    d.toggleSpell('spell:misty-step');
    d.toggleSpell('spell:mirror-image');
    d.toggleSpell('spell:invisibility');
    expect([...d.spells()]).toEqual(['spell:misty-step', 'spell:mirror-image']);
  });

  it('drops a prepared spell whose book copy was unpicked', () => {
    const d = wizard();
    d.toggleSpell('spell:misty-step');
    d.togglePrepared('spell:misty-step');
    d.toggleSpell('spell:misty-step');
    expect([...d.prepared()]).toEqual([]);
  });

  it('follows the new maximum of prepared spells that the preview gives', () => {
    const d = wizard();
    d.toggleSpell('spell:misty-step');
    d.toggleSpell('spell:mirror-image');
    expect(d.preparedAsked()).toBe(1);
    d.preparedMaxAfter.set(4);
    expect(d.preparedAsked()).toBe(2);
  });

  it('is dirty once anything beyond the defaults was chosen', () => {
    const d = wizard();
    expect(d.dirty()).toBe(false);
    d.setHpCard('roll');
    expect(d.dirty()).toBe(true);
  });
});

describe('LevelUpDraft: subclass, feature options, skills and expertise', () => {
  const options = () =>
    fighterOptions({
      subclassDue: true,
      subclasses: [
        create(LevelUpSubclassSchema, {
          key: 'sub:lore',
          namePt: 'Colégio do Conhecimento',
          skillChoices: 1,
          featureChoices: [
            create(LevelUpFeatureChoiceSchema, {
              feature: { key: 'feature:x', namePt: 'Estilo' },
              choose: 2,
              options: [
                { key: 'o:a', namePt: 'A' },
                { key: 'o:b', namePt: 'B' },
                { key: 'o:c', namePt: 'C' },
              ],
            }),
          ],
        }),
        create(LevelUpSubclassSchema, { key: 'sub:champion', namePt: 'Campeão' }),
      ],
      expertiseChoices: 1,
    });

  it('needs the subclass first, then what it adds', () => {
    const d = new LevelUpDraft(options(), WIZARD_KEYS, catalog);
    expect(d.missing().map((m) => m.id)).toEqual(['subclass', 'expertise']);
    d.setSubclass('sub:lore');
    expect(d.missing().map((m) => m.id)).toEqual(['feature-0', 'skills', 'expertise']);
  });

  it('forgets what the previous subclass offered', () => {
    const d = new LevelUpDraft(options(), WIZARD_KEYS, catalog);
    d.setSubclass('sub:lore');
    d.toggleFeature('o:a', ['o:a', 'o:b', 'o:c'], 2);
    d.toggleSkill('skill:stealth');
    d.setSubclass('sub:champion');
    expect(d.features().size + d.skills().size).toBe(0);
  });

  it('takes `choose` options of a feature, no more', () => {
    const d = new LevelUpDraft(options(), WIZARD_KEYS, catalog);
    d.setSubclass('sub:lore');
    const group = ['o:a', 'o:b', 'o:c'];
    d.toggleFeature('o:a', group, 2);
    d.toggleFeature('o:b', group, 2);
    d.toggleFeature('o:c', group, 2);
    expect([...d.features()]).toEqual(['o:a', 'o:b']);
    expect(d.choices()).toMatchObject({
      featureChoiceKeys: ['o:a', 'o:b'],
      subclassKey: 'sub:lore',
    });
  });

  it('offers expertise in a skill just picked, and drops it when the skill is unpicked', () => {
    const d = new LevelUpDraft(options(), WIZARD_KEYS, catalog);
    d.setSubclass('sub:lore');
    d.toggleSkill('skill:stealth');
    d.toggleExpertise('skill:stealth');
    expect([...d.expertise()]).toEqual(['skill:stealth']);
    d.toggleSkill('skill:stealth');
    expect([...d.expertise()]).toEqual([]);
  });
});

describe('LevelUpDraft: picks that follow the counts', () => {
  /** A wizard draft that holds the two new book spells and has the preview's maximum raised to 4. */
  const raised = () => {
    const d = wizard();
    d.toggleSpell('spell:misty-step');
    d.toggleSpell('spell:mirror-image');
    d.preparedMaxAfter.set(4);
    d.togglePrepared('spell:misty-step');
    d.togglePrepared('spell:mirror-image');
    return d;
  };

  const fighter = (skillChoices: number) =>
    fighterOptions({
      subclassDue: true,
      expertiseChoices: 1,
      subclasses: [
        create(LevelUpSubclassSchema, { key: 'sub:lore', namePt: 'Conhecimento', skillChoices }),
        create(LevelUpSubclassSchema, { key: 'sub:champion', namePt: 'Campeão' }),
      ],
    });

  it('drops the prepared picks beyond the maximum when the preview lowers it', () => {
    const d = raised();
    expect(d.choices().preparedSpellKeys).toHaveLength(2);
    // The ability increase goes back to one that does not raise the maximum.
    d.preparedMaxAfter.set(3);
    expect(d.preparedAsked()).toBe(1);
    expect(d.choices().preparedSpellKeys).toHaveLength(1);
  });

  it('keeps the prepared picks of the raised maximum when the subclass changes', () => {
    const d = new LevelUpDraft(
      wizardOptions({
        preparedMaxAfter: 3,
        subclassDue: true,
        subclasses: [create(LevelUpSubclassSchema, { key: 'sub:x', namePt: 'X' })],
      }),
      WIZARD_KEYS,
      catalog,
    );
    d.toggleSpell('spell:misty-step');
    d.toggleSpell('spell:mirror-image');
    d.preparedMaxAfter.set(4);
    d.togglePrepared('spell:misty-step');
    d.togglePrepared('spell:mirror-image');
    d.setSubclass('sub:x');
    expect(d.choices().preparedSpellKeys).toHaveLength(2);
  });

  it('sends no expertise for a skill that a subclass change trimmed away', () => {
    const d = new LevelUpDraft(fighter(2), WIZARD_KEYS, catalog);
    d.setSubclass('sub:lore');
    d.toggleSkill('skill:stealth');
    d.toggleSkill('skill:perception');
    d.toggleExpertise('skill:stealth');
    d.setSubclass('sub:champion');
    expect([...d.skills()]).toEqual([]);
    expect(d.choices().expertiseSkillKeys).toEqual([]);
  });

  it('keeps expertise on a skill the sheet already trains when the subclass changes', () => {
    const d = new LevelUpDraft(fighter(1), WIZARD_KEYS, catalog);
    d.setSubclass('sub:lore');
    d.toggleExpertise(WIZARD_KEYS.skills[0]);
    d.setSubclass('sub:champion');
    expect(d.choices().expertiseSkillKeys).toEqual([WIZARD_KEYS.skills[0]]);
  });
});

describe('LevelUpDraft: adopt after the sheet is read again', () => {
  const lore = (skillChoices: number) =>
    fighterOptions({
      subclassDue: true,
      expertiseChoices: 1,
      subclasses: [
        create(LevelUpSubclassSchema, { key: 'sub:lore', namePt: 'Conhecimento', skillChoices }),
      ],
    });

  it('sends no expertise for a skill that the new counts trimmed away', () => {
    const old = new LevelUpDraft(lore(2), WIZARD_KEYS, catalog);
    old.setSubclass('sub:lore');
    old.toggleSkill('skill:stealth');
    old.toggleSkill('skill:perception');
    old.toggleExpertise('skill:perception');
    // The level now asks for one skill only.
    const fresh = new LevelUpDraft(lore(1), WIZARD_KEYS, catalog);
    fresh.adopt(old);
    expect([...fresh.skills()]).toEqual(['skill:stealth']);
    expect(fresh.choices().expertiseSkillKeys).toEqual([]);
  });

  it('keeps the prepared picks the old draft had under its raised maximum', () => {
    const old = new LevelUpDraft(wizardOptions({ preparedMaxAfter: 3 }), WIZARD_KEYS, catalog);
    old.toggleSpell('spell:misty-step');
    old.toggleSpell('spell:mirror-image');
    old.preparedMaxAfter.set(4);
    old.togglePrepared('spell:misty-step');
    old.togglePrepared('spell:mirror-image');
    const fresh = wizard();
    fresh.adopt(old);
    expect(fresh.choices().preparedSpellKeys).toHaveLength(2);
  });

  it('goes back to the average, with no roll, when the table now allows only the average', () => {
    const old = wizard();
    old.setHpCard('roll');
    old.rolled.set({ kind: 'app', value: 5 });
    const fresh = new LevelUpDraft(
      wizardOptions({ preparedMaxAfter: 3, hitPointsRule: LevelUpHitPointsRule.AVERAGE_ONLY }),
      WIZARD_KEYS,
      catalog,
    );
    fresh.adopt(old);
    expect(fresh.hpCard()).toBe('average');
    expect(fresh.rolled()).toBeNull();
    expect(fresh.choices().hitPoints?.method).toBe(LevelUpHitPointsMethod.AVERAGE);
  });

  it('keeps an in-app roll only when it is the one the server kept for this class and level', () => {
    const old = wizard();
    old.setHpCard('roll');
    old.rolled.set({ kind: 'app', value: 5 });
    const same = new LevelUpDraft(
      wizardOptions({ preparedMaxAfter: 3, keptHitPointRoll: 5 }),
      WIZARD_KEYS,
      catalog,
    );
    same.adopt(old);
    expect(same.rolled()).toEqual({ kind: 'app', value: 5 });
    const nextLevel = new LevelUpDraft(
      wizardOptions({ fromLevel: 4, toLevel: 5, keptHitPointRoll: 0 }),
      WIZARD_KEYS,
      catalog,
    );
    nextLevel.adopt(old);
    expect(nextLevel.hpCard()).toBe('roll');
    expect(nextLevel.rolled()).toBeNull();
  });

  it('keeps a typed roll only for the same class and level and a die it fits', () => {
    const old = wizard();
    old.setHpCard('roll');
    old.rolled.set({ kind: 'physical', value: 6 });
    const same = wizard();
    same.adopt(old);
    expect(same.rolled()).toEqual({ kind: 'physical', value: 6 });
    const smallerDie = new LevelUpDraft(
      wizardOptions({ preparedMaxAfter: 3, hitDie: 4 }),
      WIZARD_KEYS,
      catalog,
    );
    smallerDie.adopt(old);
    expect(smallerDie.rolled()).toBeNull();
    const nextLevel = new LevelUpDraft(
      wizardOptions({ fromLevel: 4, toLevel: 5 }),
      WIZARD_KEYS,
      catalog,
    );
    nextLevel.adopt(old);
    expect(nextLevel.rolled()).toBeNull();
  });

  describe('LevelUpDraft: a feat in place of the increase (MR-025)', () => {
    const withFeats = () =>
      new LevelUpDraft(
        wizardOptions({ preparedMaxAfter: 3, feats: featOptions() }),
        WIZARD_KEYS,
        catalog,
      );

    it('offers the feat only when the server lists feats at the level', () => {
      expect(wizard().hasFeats()).toBe(false);
      wizard().setAsiMode('feat');
      expect(wizard().taking()).toBe(false);
      const d = withFeats();
      expect(d.hasFeats()).toBe(true);
      d.setAsiMode('feat');
      expect(d.taking()).toBe(true);
    });

    it("asks for the feat, then for the abilities it raises, and sends feat_key with the feat's increase", () => {
      const d = withFeats();
      d.setAsiMode('feat');
      expect(d.missingIn('abilities').map((m) => m.text)).toEqual(['Falta escolher o talento.']);
      d.setFeat('feat:atleta@mesa');
      expect(d.missingIn('abilities').map((m) => m.text)).toEqual(['Falta escolher 1 habilidade.']);
      d.toggleFeatAbility(Ability.DEXTERITY);
      expect(d.missingIn('abilities')).toEqual([]);
      expect(d.pickedKeys()).toEqual(['dex']);
      expect(d.choices()).toMatchObject({
        featKey: 'feat:atleta@mesa',
        abilityIncrease: { dexterity: 1 },
      });
    });

    it('a feat with no increase sends none, and picking another ability replaces it when only one is asked', () => {
      const d = withFeats();
      d.setAsiMode('feat');
      d.setFeat('feat:grappler');
      expect(d.missingIn('abilities')).toEqual([]);
      expect(d.choices().abilityIncrease).toEqual({});
      d.setFeat('feat:atleta@mesa');
      d.toggleFeatAbility(Ability.STRENGTH);
      d.toggleFeatAbility(Ability.DEXTERITY);
      expect(d.featAbilities()).toEqual([Ability.DEXTERITY]);
      d.toggleFeatAbility(Ability.CONSTITUTION);
      expect(d.featAbilities()).toEqual([Ability.DEXTERITY]);
    });

    it('does not take a feat the character does not qualify for', () => {
      const d = withFeats();
      d.setAsiMode('feat');
      d.setFeat('feat:mestre@mesa');
      expect(d.featKey()).toBe('');
    });

    it('going back to the increase sends the increase and no feat', () => {
      const d = withFeats();
      d.setAsiMode('feat');
      d.setFeat('feat:atleta@mesa');
      d.toggleFeatAbility(Ability.STRENGTH);
      d.setAsiMode('increase');
      d.toggleAbility('int');
      expect(d.choices()).toMatchObject({ featKey: '', abilityIncrease: { intelligence: 2 } });
    });

    it('keeps the feat and its abilities after the sheet is read again, if the server still offers it', () => {
      const before = withFeats();
      before.setAsiMode('feat');
      before.setFeat('feat:atleta@mesa');
      before.toggleFeatAbility(Ability.STRENGTH);
      const after = withFeats();
      after.adopt(before);
      expect(after.taking()).toBe(true);
      expect(after.featKey()).toBe('feat:atleta@mesa');
      expect(after.featAbilities()).toEqual([Ability.STRENGTH]);
      const gone = new LevelUpDraft(wizardOptions({ preparedMaxAfter: 3 }), WIZARD_KEYS, catalog);
      gone.adopt(before);
      expect(gone.taking()).toBe(false);
      expect(gone.featKey()).toBe('');
    });

    it('counts a picked feat as a choice made (leaving asks first)', () => {
      const d = withFeats();
      d.setAsiMode('feat');
      d.setFeat('feat:grappler');
      expect(d.dirty()).toBe(true);
    });
  });

  describe('LevelUpDraft: feats and the abilities at 20 (MR-025)', () => {
    const draftWith = (feat: ReturnType<typeof featOptions>[number]) => {
      const d = new LevelUpDraft(
        wizardOptions({ preparedMaxAfter: 3, feats: [feat] }),
        WIZARD_KEYS,
        catalog,
      );
      d.setAsiMode('feat');
      d.setFeat(feat.key);
      return d;
    };
    const feat = (over: { count: number; from: Ability[]; capped?: Ability[] }) => {
      const f = featOptions()[1];
      f.increase = { ...f.increase!, count: over.count, from: over.from } as never;
      f.cappedAbilities = over.capped ?? [];
      return f;
    };

    it('sends no increase for a feat that raises every ability it lists: the server applies it', () => {
      const d = draftWith(
        feat({ count: 2, from: [Ability.STRENGTH, Ability.DEXTERITY], capped: [Ability.STRENGTH] }),
      );
      expect(d.featFixed()).toBe(true);
      expect(d.missingIn('abilities')).toEqual([]);
      expect(d.choices().abilityIncrease).toEqual({});
      expect(d.choices().featKey).toBe('feat:atleta@mesa');
      // The Resumo still names what rises: the abilities not at the cap.
      expect(d.featSummary()).toEqual({ name: 'Atleta', increase: '+1 Destreza' });
    });

    it('does not let the player pick an ability the server says would pass 20', () => {
      const d = draftWith(
        feat({ count: 1, from: [Ability.STRENGTH, Ability.DEXTERITY], capped: [Ability.STRENGTH] }),
      );
      d.toggleFeatAbility(Ability.STRENGTH);
      expect(d.featAbilities()).toEqual([]);
      d.toggleFeatAbility(Ability.DEXTERITY);
      expect(d.choices().abilityIncrease).toEqual({ dexterity: 1 });
      expect(d.featSummary()?.increase).toBe('+1 Destreza');
    });

    it('asks only for the free abilities when fewer than the count are below 20', () => {
      const d = draftWith(
        feat({
          count: 2,
          from: [Ability.STRENGTH, Ability.DEXTERITY, Ability.CONSTITUTION],
          capped: [Ability.STRENGTH],
        }),
      );
      expect(d.featAsked()).toBe(2);
      const few = draftWith(
        feat({
          count: 2,
          from: [Ability.STRENGTH, Ability.DEXTERITY, Ability.CONSTITUTION],
          capped: [Ability.STRENGTH, Ability.DEXTERITY],
        }),
      );
      expect(few.featAsked()).toBe(1);
      few.toggleFeatAbility(Ability.CONSTITUTION);
      expect(few.missingIn('abilities')).toEqual([]);
      expect(few.choices().abilityIncrease).toEqual({ constitution: 1 });
    });

    it('has no feat summary while the increase is taken', () => {
      expect(wizard().featSummary()).toBeNull();
    });
  });
});

describe('LevelUpDraft: choices an earlier level left open (PM-05)', () => {
  const style = create(ChoiceSchema, {
    key: 'style',
    labelPt: 'Estilo de Luta (Guerreiro, nível 1)',
    kind: ChoiceKind.OPTIONS,
    picks: 1,
    missing: 1,
    options: [
      create(ChoiceOptionSchema, { key: 'defense', storedKey: 'stored:defense', namePt: 'Defesa' }),
      create(ChoiceOptionSchema, { key: 'duel', storedKey: 'stored:duel', namePt: 'Duelo' }),
    ],
  });
  const terrain = create(ChoiceSchema, {
    key: 'terrain',
    labelPt: 'Explorador Natural',
    kind: ChoiceKind.OPTIONS,
    picks: 1,
    missing: 1,
    options: [
      create(ChoiceOptionSchema, { key: 'forest', storedKey: 'stored:forest', namePt: 'Floresta' }),
    ],
  });
  const withChoices = () =>
    new LevelUpDraft(
      fighterOptions({
        lateChoices: [create(ChoiceGroupSchema, { choices: [style] })],
        newChoices: [create(ChoiceGroupSchema, { choices: [terrain] })],
      }),
      WIZARD_KEYS,
      catalog,
    );

  it('asks for the late choices first and holds "Próximo" of the step until they are made', () => {
    const d = withChoices();
    expect(d.missingIn('picks').map((m) => [m.id, m.text])).toEqual([
      ['late', 'Falta escolher 1 escolha que ficou para trás.'],
      ['new', 'Falta escolher 1 escolha do nível.'],
    ]);
    d.selectLate(style, ['duel']);
    expect(d.missingIn('picks').map((m) => m.id)).toEqual(['new']);
    d.selectNew(terrain, ['forest']);
    expect(d.missingIn('picks')).toEqual([]);
  });

  it('sends the late picks apart from the feature options, and the new ones among them', () => {
    const d = withChoices();
    d.selectLate(style, ['duel']);
    d.selectNew(terrain, ['forest']);
    expect(d.choices()).toMatchObject({
      lateChoiceKeys: ['stored:duel'],
      featureChoiceKeys: ['stored:forest'],
      featureChoiceText: {},
    });
  });

  it('is dirty once a late choice was picked, and takes the picks over after the sheet is read again', () => {
    const d = withChoices();
    expect(d.dirty()).toBe(false);
    d.selectLate(style, ['defense']);
    expect(d.dirty()).toBe(true);
    const next = withChoices();
    next.adopt(d);
    expect(next.choices()).toMatchObject({ lateChoiceKeys: ['stored:defense'] });
  });
});

describe('LevelUpDraft: a class taken as a later class (SRD 5.1, "Multiclassing")', () => {
  const pick = (
    kind: LevelUpProficiencyKind,
    count: number,
    from: [string, string, boolean][],
  ) => ({
    kind,
    count,
    from: from.map(([key, namePt, alreadyHave]) => ({ key, namePt, alreadyHave })),
  });
  const rogue = () =>
    fighterOptions({
      classKey: 'class:rogue',
      classNamePt: 'Ladino',
      isNewClass: true,
      fromLevel: 0,
      toLevel: 1,
      expertiseChoices: 2,
      proficiencyChoices: [
        pick(LevelUpProficiencyKind.SKILL, 1, [
          ['skill:arcana', 'Arcanismo', false],
          ['skill:history', 'História', true],
          ['skill:stealth', 'Furtividade', false],
        ]),
      ],
    });
  const bard = () =>
    fighterOptions({
      classKey: 'class:bard',
      classNamePt: 'Bardo',
      isNewClass: true,
      fromLevel: 0,
      toLevel: 1,
      proficiencyChoices: [
        pick(LevelUpProficiencyKind.SKILL, 1, [['skill:stealth', 'Furtividade', false]]),
        pick(LevelUpProficiencyKind.INSTRUMENT, 1, [
          ['proficiency:lute', 'Alaúde', false],
          ['proficiency:drum', 'Tambor', true],
        ]),
      ],
    });

  it('asks for the skill of the class list, with the ones the character has turned off, and the expertise after it', () => {
    const d = new LevelUpDraft(rogue(), { ...WIZARD_KEYS, skills: ['skill:history'] }, catalog);
    expect(d.steps()).toEqual(['class', 'hp', 'picks', 'summary']);
    expect(d.skillsAsked()).toBe(1);
    expect(d.skillItems().map((i) => [i.key, i.disabled ?? ''])).toEqual([
      ['skill:arcana', ''],
      ['skill:stealth', ''],
      ['skill:history', 'Você já tem'],
    ]);
    // A turned off skill cannot be picked.
    d.toggleSkill('skill:history');
    expect(d.skills().size).toBe(0);
    d.toggleSkill('skill:stealth');
    expect(d.missing().map((m) => m.id)).toEqual(['expertise']);
    expect(d.choices().skillProficiencyKeys).toEqual(['skill:stealth']);
  });

  it("asks for the Bard's instrument in its own pick, and sends it as instrumentKey", () => {
    const d = new LevelUpDraft(bard(), WIZARD_KEYS, catalog);
    expect(d.steps()).toContain('picks');
    expect(d.instrumentAsked()).toBe(1);
    expect(d.missing().map((m) => m.id)).toEqual(['skills', 'instrument']);
    d.toggleInstrument('proficiency:drum'); // already has it: nothing happens
    expect(d.instrument()).toBe('');
    d.toggleInstrument('proficiency:lute');
    d.toggleSkill('skill:stealth');
    expect(d.missing()).toEqual([]);
    expect(d.choices().instrumentKey).toBe('proficiency:lute');
    expect(d.dirty()).toBe(true);
    d.toggleInstrument('proficiency:lute');
    expect(d.instrument()).toBe('');
  });

  it('opens the flow with the class step, always, and has no picks for a class that asks for none', () => {
    const d = new LevelUpDraft(fighterOptions(), WIZARD_KEYS, catalog);
    expect(d.steps()).toEqual(['class', 'hp', 'summary']);
    expect(d.choices().instrumentKey).toBe('');
  });
});
