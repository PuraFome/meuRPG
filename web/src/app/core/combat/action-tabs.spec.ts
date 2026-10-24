import { create } from '@bufbuild/protobuf';

import {
  ActionEconomy,
  ActionOptionSchema,
  AttackOptionSchema,
  SpellOptionSchema,
  TurnOptionsSchema,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { actionCounts, actionTabs, classTabName, featureGroup, partByEconomy } from './action-tabs';

const feature = (key: string, economy: ActionEconomy) =>
  create(ActionOptionSchema, { action: { key, namePt: key, economy }, enabled: true });

describe('classTabName', () => {
  it('names the tab for the class that grants the features', () => {
    expect(
      classTabName([
        feature('feature:second-wind', ActionEconomy.BONUS_ACTION),
        feature('feature:action-surge-1-use', ActionEconomy.FREE),
      ]),
    ).toBe('Guerreiro');
  });

  it('calls the Monk\'s tab "Chi"', () => {
    expect(
      classTabName([feature('feature:step-of-the-wind:dash', ActionEconomy.BONUS_ACTION)]),
    ).toBe('Chi');
  });

  it("tells the Paladin's Channel Divinity from the Cleric's", () => {
    expect(
      classTabName([feature('feature:channel-divinity-sacred-weapon', ActionEconomy.ACTION)]),
    ).toBe('Paladino');
    expect(
      classTabName([feature('feature:channel-divinity-turn-undead', ActionEconomy.ACTION)]),
    ).toBe('Clérigo');
  });

  it('falls back to "Habilidades" for a feature of no known class, or of two classes', () => {
    expect(classTabName([feature('trait:breath-weapon', ActionEconomy.ACTION)])).toBe(
      'Habilidades',
    );
    expect(
      classTabName([
        feature('feature:rage', ActionEconomy.BONUS_ACTION),
        feature('feature:cunning-action', ActionEconomy.BONUS_ACTION),
      ]),
    ).toBe('Habilidades');
  });
});

describe('featureGroup', () => {
  it('gives each feature the economy it spends, and none to a free one', () => {
    expect(featureGroup(feature('a', ActionEconomy.ACTION))).toBe('action');
    expect(featureGroup(feature('a', ActionEconomy.BONUS_ACTION))).toBe('bonus');
    expect(featureGroup(feature('a', ActionEconomy.REACTION))).toBe('reaction');
    expect(featureGroup(feature('a', ActionEconomy.FREE))).toBeNull();
  });
});

describe('partByEconomy', () => {
  const rows = [
    { n: 'Raio', g: 'action' as const },
    { n: 'Passo Nebuloso', g: 'bonus' as const },
    { n: 'Escudo', g: 'reaction' as const },
    { n: 'Surto', g: null },
  ];
  const groupFor = (r: (typeof rows)[number]) => r.g;

  it('keeps everything open while nothing is spent', () => {
    const parts = partByEconomy(
      rows,
      groupFor,
      { action: false, bonus: false, reaction: false },
      'Usada',
    );
    expect(parts.open).toHaveLength(4);
    expect(parts.folded).toEqual([]);
  });

  it('folds a spent group to one titled line, in the order Ação, Ação bônus, Reação', () => {
    const parts = partByEconomy(
      rows,
      groupFor,
      { action: false, bonus: true, reaction: true },
      'Usada',
    );
    expect(parts.open.map((r) => r.n)).toEqual(['Raio', 'Surto']);
    expect(parts.folded.map((f) => f.title)).toEqual(['Ação bônus · Usada', 'Reação · Usada']);
    expect(parts.folded[0].rows.map((r) => r.n)).toEqual(['Passo Nebuloso']);
  });

  it('never folds a row with no economy', () => {
    const parts = partByEconomy(
      rows,
      groupFor,
      { action: true, bonus: true, reaction: true },
      'Indisponível',
    );
    expect(parts.open.map((r) => r.n)).toEqual(['Surto']);
    expect(parts.folded[0].title).toBe('Ação · Indisponível');
  });
});

describe('actionTabs', () => {
  const counts = { attacks: 2, spells: 4, features: 0, standard: 8 };

  it('draws Ataques, Magias and Outras for a caster, each with its count in words', () => {
    const tabs = actionTabs(counts, 'Guerreiro');
    expect(tabs.map((t) => t.label)).toEqual(['Ataques', 'Magias', 'Outras']);
    expect(tabs.map((t) => t.badge)).toEqual(['2', '4', '8']);
    expect(tabs[1].words).toBe('4 magias');
  });

  it('adds the class tab when the class grants actions, and leaves Magias out for a fighter', () => {
    const tabs = actionTabs({ ...counts, spells: 0, features: 2 }, 'Guerreiro');
    expect(tabs.map((t) => t.label)).toEqual(['Ataques', 'Guerreiro', 'Outras']);
    expect(tabs[1].words).toBe('2 habilidades');
  });

  it('keeps the class tab for a beast, whose way back is there', () => {
    expect(
      actionTabs({ ...counts, features: 0 }, 'Druida', { beast: true }).map((t) => t.id),
    ).toContain('class');
  });

  it('says one in the singular', () => {
    expect(actionTabs({ ...counts, attacks: 1 }, 'x')[0].words).toBe('1 ataque');
  });
});

describe('actionCounts', () => {
  it('counts the contest attacks and Escapar with the attacks', () => {
    const options = create(TurnOptionsSchema, {
      attacks: [create(AttackOptionSchema), create(AttackOptionSchema)],
      spells: [create(SpellOptionSchema)],
      featureActions: [feature('feature:rage', ActionEconomy.BONUS_ACTION)],
    });
    expect(actionCounts(options, { contestAttacks: 2, escape: true, standard: 8 })).toEqual({
      attacks: 5,
      spells: 1,
      features: 1,
      standard: 8,
    });
  });
});
