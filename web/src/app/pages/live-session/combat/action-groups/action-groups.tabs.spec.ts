import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import { CombatantSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  ActionEconomy,
  ActionOptionSchema,
  AttackOptionSchema,
  SpellOptionSchema,
  TurnOptionsSchema,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { ActionGroups } from './action-groups';

const attack = (key: string, namePt: string) =>
  create(AttackOptionSchema, {
    attack: {
      key,
      name: namePt,
      namePt,
      attackBonus: 5,
      damage: '1d8',
      damageTypePt: 'cortante',
      rangeFt: 5,
    },
    enabled: true,
  });
const spell = (key: string, namePt: string, economy = ActionEconomy.ACTION) =>
  create(SpellOptionSchema, {
    spell: { key, name: namePt, namePt, level: 1, schoolNamePt: 'Evocação' },
    economy,
    enabled: true,
  });
const feature = (key: string, namePt: string, economy: ActionEconomy) =>
  create(ActionOptionSchema, { action: { key, namePt, economy }, enabled: true });
const standard = (key: string, namePt: string) =>
  create(ActionOptionSchema, {
    action: { key: `standard:${key}`, namePt, economy: ActionEconomy.ACTION },
    enabled: true,
  });

describe('ActionGroups: the action tabs (C4-03, C4-04, C6-07)', () => {
  function setup(
    options: Parameters<typeof create<typeof TurnOptionsSchema>>[1],
    own: Parameters<typeof create<typeof CombatantSchema>>[1] = {},
  ) {
    const fixture = TestBed.createComponent(ActionGroups);
    fixture.componentRef.setInput('options', create(TurnOptionsSchema, options));
    fixture.componentRef.setInput(
      'own',
      create(CombatantSchema, { movementLeftFt: 30, speedFt: 30, ...own }),
    );
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const tabs = () => [...el.querySelectorAll<HTMLButtonElement>('[role="tab"]')];
    const panel = (name: string) =>
      el.querySelector<HTMLElement>(`[role="tabpanel"][aria-labelledby$="-tab-${name}"]`)!;
    const open = (label: string) => {
      tabs()
        .find((t) => t.textContent?.includes(label))!
        .click();
      fixture.detectChanges();
    };
    return { fixture, el, tabs, panel, open };
  }
  const names = (root: Element) =>
    [...root.querySelectorAll('.row__name')].map((n) => n.textContent!.trim());
  const wizard = {
    attacks: [attack('equipment:dagger', 'Adaga'), attack('equipment:staff', 'Cajado')],
    spells: [
      spell('spell:magic-missile', 'Mísseis Mágicos'),
      spell('spell:web', 'Teia'),
      spell('spell:misty-step', 'Passo Nebuloso', ActionEconomy.BONUS_ACTION),
      spell('spell:shield', 'Escudo Arcano', ActionEconomy.REACTION),
    ],
    standardActions: ['dash', 'dodge', 'help'].map((k) => standard(k, k)),
  };

  it('draws Ataques, Magias and Outras for a caster, each with its count in the tab’s name', () => {
    const { tabs } = setup(wizard);
    expect(tabs().map((t) => t.getAttribute('aria-label'))).toEqual([
      'Ataques, 2 ataques',
      'Magias, 4 magias',
      'Outras, 3 ações',
    ]);
  });

  it('keeps one tab stop and shows the first tab, with the other panels mounted and hidden', () => {
    const { tabs, panel } = setup(wizard);
    expect(tabs().map((t) => t.getAttribute('tabindex'))).toEqual(['0', '-1', '-1']);
    expect(tabs()[0].getAttribute('aria-selected')).toBe('true');
    expect(panel('attacks').hidden).toBe(false);
    expect(panel('spells').hidden).toBe(true);
    expect(panel('other').hidden).toBe(true);
    // Mounted: a draft, a focus and the place in a list survive a change of tab.
    expect(names(panel('spells'))).toContain('Teia');
  });

  it('shows a panel when its tab is chosen, and only that one', () => {
    const { panel, open } = setup(wizard);
    open('Magias');
    expect(panel('spells').hidden).toBe(false);
    expect(panel('attacks').hidden).toBe(true);
  });

  it('has no Magias tab for a fighter and names the class’s tab for what it grants', () => {
    const { tabs, panel } = setup({
      attacks: [attack('equipment:longsword', 'Espada longa')],
      featureActions: [
        feature('feature:second-wind', 'Retomar o Fôlego', ActionEconomy.BONUS_ACTION),
        feature('feature:action-surge-1-use', 'Surto de Ação', ActionEconomy.FREE),
      ],
      standardActions: [standard('dash', 'Disparada')],
    });
    expect(tabs().map((t) => t.getAttribute('aria-label'))).toEqual([
      'Ataques, 1 ataque',
      'Guerreiro, 2 habilidades',
      'Outras, 1 ação',
    ]);
    expect(names(panel('class'))).toEqual(['Retomar o Fôlego', 'Surto de Ação']);
  });

  it('folds a spent economy to one line that opens to what it had', () => {
    const { panel, fixture } = setup(wizard, { bonusActionUsed: true, reactionUsed: true });
    const spells = panel('spells');
    expect(names(spells)).toEqual(['Mísseis Mágicos', 'Teia']);
    const folds = [...spells.querySelectorAll<HTMLButtonElement>('.mr-fold__button')];
    expect(folds.map((b) => b.textContent!.replace(/\s+/g, ' ').trim())).toEqual([
      expect.stringContaining('Ação bônus · Usada'),
      expect.stringContaining('Reação · Usada'),
    ]);
    expect(folds[0].getAttribute('aria-expanded')).toBe('false');
    folds[0].click();
    fixture.detectChanges();
    expect(folds[0].getAttribute('aria-expanded')).toBe('true');
    expect(names(spells)).toContain('Passo Nebuloso');
  });

  it('never folds what costs nothing, even with the Action spent', () => {
    const { panel } = setup(
      {
        featureActions: [
          feature('feature:second-wind', 'Retomar o Fôlego', ActionEconomy.BONUS_ACTION),
          feature('feature:action-surge-1-use', 'Surto de Ação', ActionEconomy.FREE),
        ],
      },
      { bonusActionUsed: true, actionUsed: true },
    );
    expect(names(panel('class'))).toEqual(['Surto de Ação']);
    expect(panel('class').querySelectorAll('.mr-fold__button')).toHaveLength(1);
  });

  it('folds the standard actions once the Action is spent, and leaves them open while it is there', () => {
    const open = setup(wizard);
    expect(open.panel('other').querySelectorAll('.std__btn')).toHaveLength(3);
    expect(open.panel('other').querySelector('.mr-fold__button')).toBeNull();
    TestBed.resetTestingModule();
    const spent = setup(wizard, { actionUsed: true });
    expect(spent.panel('other').querySelectorAll('.std__btn')).toHaveLength(0);
    const fold = spent.panel('other').querySelector<HTMLButtonElement>('.mr-fold__button')!;
    expect(fold.textContent).toContain('Ação · Usada');
    fold.click();
    spent.fixture.detectChanges();
    expect(spent.panel('other').querySelectorAll('.std__btn')).toHaveLength(3);
  });

  it('keeps the Action open while Extra Attack has attacks left', () => {
    const fixture = TestBed.createComponent(ActionGroups);
    fixture.componentRef.setInput('options', create(TurnOptionsSchema, wizard));
    fixture.componentRef.setInput('own', create(CombatantSchema, { actionUsed: true }));
    fixture.componentRef.setInput('attacksLeft', 1);
    fixture.componentRef.setInput('attacksPerAction', 2);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelectorAll('[role="tabpanel"]:first-of-type .mr-fold__button')).toHaveLength(0);
    expect(
      [...el.querySelectorAll('.row__name')].map((n) => n.textContent!.trim()).slice(0, 2),
    ).toEqual(['Adaga', 'Cajado']);
  });

  it('answers the arrow keys, Home and End on the tab list', () => {
    const { tabs, fixture } = setup(wizard);
    tabs()[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    fixture.detectChanges();
    expect(tabs()[1].getAttribute('aria-selected')).toBe('true');
    tabs()[1].dispatchEvent(new KeyboardEvent('keydown', { key: 'End', bubbles: true }));
    fixture.detectChanges();
    expect(tabs()[2].getAttribute('aria-selected')).toBe('true');
  });

  it('has no "Mover": the row under the map has the one button', () => {
    const { el } = setup(wizard);
    expect([...el.querySelectorAll('button')].map((b) => b.textContent!.trim())).not.toContain(
      'Mover',
    );
  });
});
