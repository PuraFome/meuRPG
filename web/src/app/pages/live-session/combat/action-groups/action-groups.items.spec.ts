import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import { CombatantSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  ActionEconomy,
  ActionOptionSchema,
  AttackOptionSchema,
  TurnOptionsSchema,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import type { AttunementEntryVm } from '../../../../core/characters/attunement';
import type { CombatItemRowVm, ConsumableNotice } from '../../../../core/characters/consumables';
import { ActionGroups } from './action-groups';

const unit = (id: string, key: string, namePt: string, effectPt: string): AttunementEntryVm => ({
  id,
  key,
  namePt,
  kind: 'magic',
  attuned: false,
  asks: false,
  restrictionPt: '',
  restriction: 'none',
  sameItemAttuned: false,
  effectPt,
});

const row = (
  over: Partial<CombatItemRowVm> & Pick<CombatItemRowVm, 'key' | 'namePt'>,
): CombatItemRowVm => ({
  id: `consumable:${over.key}`,
  kind: 'potion',
  icon: 'science',
  detail: 'cura 2d4 + 2',
  count: 1,
  economy: 'action',
  first: unit(`u-${over.key}`, over.key, over.namePt, 'Cura 2d4 + 2'),
  ...over,
});

const HEAL = row({
  key: 'item:potion-of-healing',
  namePt: 'Poção de cura',
  count: 2,
  detail: 'cura 2d4 + 2',
});
const FIRE = row({
  key: 'item:potion-of-resistance-fire',
  namePt: 'Poção de resistência',
  detail: 'resistência a fogo por 1 hora',
});
const OIL = row({
  key: 'item:oil-of-sharpness',
  namePt: 'Óleo de afiar',
  detail: 'só mostra o texto',
  economy: null,
});
const SCROLL = row({
  key: 'item:spell-scroll-3rd',
  namePt: 'Pergaminho de magia',
  kind: 'scroll',
  icon: 'description',
  detail: 'só mostra o texto',
  economy: null,
});

describe('ActionGroups: the Itens tab', () => {
  function setup(
    inputs: {
      items?: readonly CombatItemRowVm[];
      itemsNotice?: ConsumableNotice | null;
      locked?: string;
      busy?: boolean;
    } = {},
    own: Parameters<typeof create<typeof CombatantSchema>>[1] = {},
    options: Parameters<typeof create<typeof TurnOptionsSchema>>[1] = {
      attacks: [
        create(AttackOptionSchema, {
          attack: { key: 'equipment:dagger', name: 'Adaga', namePt: 'Adaga', attackBonus: 5 },
          enabled: true,
        }),
      ],
      featureActions: [
        create(ActionOptionSchema, {
          action: {
            key: 'feature:second-wind',
            namePt: 'Retomar o Fôlego',
            economy: ActionEconomy.BONUS_ACTION,
          },
          enabled: true,
        }),
      ],
    },
  ) {
    const fixture = TestBed.createComponent(ActionGroups);
    fixture.componentRef.setInput('options', create(TurnOptionsSchema, options));
    fixture.componentRef.setInput(
      'own',
      create(CombatantSchema, { movementLeftFt: 30, speedFt: 30, ...own }),
    );
    for (const [k, v] of Object.entries(inputs)) {
      fixture.componentRef.setInput(k, v);
    }
    const used: AttunementEntryVm[] = [];
    fixture.componentInstance.useItem.subscribe((e) => used.push(e));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const tabs = () => [...el.querySelectorAll<HTMLButtonElement>('[role="tab"]')];
    const panel = () =>
      el.querySelector<HTMLElement>('[role="tabpanel"][aria-labelledby$="-tab-items"]');
    const open = () => {
      tabs()
        .find((t) => t.textContent?.includes('Itens'))!
        .click();
      fixture.detectChanges();
    };
    return { fixture, el, tabs, panel, open, used };
  }
  const text = (n: Element | null | undefined) =>
    (n?.textContent ?? '').replace(/\s+/g, ' ').trim();
  const names = (root: Element) =>
    [...root.querySelectorAll('.row__name')].map((n) => n.textContent!.trim());

  it('is a tab after the class tab and before Outras, with its icon, its label and its count', () => {
    const { tabs } = setup({ items: [HEAL, FIRE, OIL, SCROLL] });
    expect(tabs().map((t) => t.getAttribute('aria-label'))).toEqual([
      'Ataques 1, 1 ataque',
      'Guerreiro 1, 1 habilidade',
      'Itens 4, 4 itens',
      'Outras 0, 0 ações',
    ]);
    expect(tabs()[2].querySelector('mat-icon')?.textContent?.trim()).toBe('science');
  });

  it('is absent with no consumable', () => {
    const { tabs, panel } = setup({ items: [] });
    expect(tabs().map((t) => t.textContent)).not.toContain(expect.stringContaining('Itens'));
    expect(panel()).toBeNull();
  });

  it('lists a row for each item with its name, its line and "Usar" with the name of the item', () => {
    const { panel, open } = setup({ items: [HEAL, FIRE] });
    open();
    const p = panel()!;
    expect(p.hidden).toBe(false);
    expect(names(p)).toEqual(['Poção de cura', 'Poção de resistência']);
    expect([...p.querySelectorAll('.row__detail')].map(text)).toEqual([
      'cura 2d4 + 2',
      'resistência a fogo por 1 hora',
    ]);
    const buttons = [...p.querySelectorAll<HTMLButtonElement>('.row__btn')];
    expect(buttons.map((b) => b.getAttribute('aria-label'))).toEqual([
      'Usar Poção de cura',
      'Usar Poção de resistência',
    ]);
    expect(buttons.map(text)).toEqual(['science Usar', 'science Usar']);
  });

  it('says "×2" beside the name of a group, and reads it as "2 unidades"', () => {
    const { panel, open } = setup({ items: [HEAL, FIRE] });
    open();
    const rows = [...panel()!.querySelectorAll('app-action-row')];
    const times = rows[0].querySelector('.row__times')!;
    expect(text(times.querySelector('[aria-hidden="true"]'))).toBe('×2');
    expect(times.hasAttribute('aria-label')).toBe(false);
    expect(text(times.querySelector('.mr-visually-hidden'))).toBe(', 2 unidades');
    expect(names(rows[0])).toEqual(['Poção de cura']);
    expect(rows[1].querySelector('.row__times')).toBeNull();
  });

  it('marks the Ação on a potion and nothing on an oil or a scroll', () => {
    const { panel, open } = setup({ items: [HEAL, OIL, SCROLL] });
    open();
    const econ = [...panel()!.querySelectorAll('.row__text')].map((r) =>
      text(r.querySelector('.row__econ')),
    );
    expect(econ).toEqual(['Ação', '', '']);
  });

  it('draws a page on the button of a scroll, "Usar" like every other row (no "Ler" yet)', () => {
    const { panel, open } = setup({ items: [SCROLL] });
    open();
    expect(text(panel()!.querySelector('.row__btn'))).toBe('description Usar');
    expect(text(panel()!)).not.toContain('Ler');
  });

  it('sends the unit of the group when "Usar" is pressed', () => {
    const { panel, open, used } = setup({ items: [HEAL, FIRE] });
    open();
    panel()!.querySelectorAll<HTMLButtonElement>('.row__btn')[0].click();
    expect(used).toEqual([HEAL.first]);
  });

  it('folds the rows that cost the Ação to "Ação · Usada" once it is spent, and keeps an oil open', () => {
    const { panel, open, fixture } = setup({ items: [HEAL, FIRE, OIL] }, { actionUsed: true });
    open();
    const p = panel()!;
    expect(names(p)).toEqual(['Óleo de afiar']);
    const fold = p.querySelector<HTMLButtonElement>('.mr-fold__button')!;
    expect(text(fold)).toContain('Ação · Usada');
    expect(fold.getAttribute('aria-expanded')).toBe('false');
    fold.click();
    fixture.detectChanges();
    expect(names(p)).toEqual(['Óleo de afiar', 'Poção de cura', 'Poção de resistência']);
  });

  it('folds them with Extra Attack too: the first attack already spent the Ação a potion costs', () => {
    const { panel, open } = setup({ items: [HEAL], attacksLeft: 1, attacksPerAction: 2 } as never, {
      actionUsed: true,
    });
    open();
    expect(names(panel()!)).toEqual([]);
    expect(text(panel()!.querySelector('.mr-fold__button'))).toContain('Ação · Usada');
  });

  it('says "Indisponível" for who cannot act', () => {
    const { panel, open } = setup({ items: [HEAL] }, { conditions: ['condition:stunned'] });
    open();
    expect(text(panel()!.querySelector('.mr-fold__button'))).toContain('Ação · Indisponível');
  });

  it('turns every "Usar" off while an answer is awaited, and says why', () => {
    const { panel, open, used } = setup({ items: [HEAL], locked: 'Esperando a reação do mestre' });
    open();
    const button = panel()!.querySelector<HTMLButtonElement>('.row__btn')!;
    expect(button.getAttribute('aria-disabled') === 'true' || button.disabled).toBe(true);
    button.click();
    expect(used).toEqual([]);
  });

  it('says what the Ação costs and what an oil or a scroll does not, in a line under the rows', () => {
    const { panel, open } = setup({ items: [HEAL] });
    open();
    expect(text(panel()!.querySelector('.items__rule span'))).toBe(
      'Beber ou dar uma poção gasta a ação. Óleos e pergaminhos não gastam a ação: o mestre decide o tempo.',
    );
  });

  it('shows what the last use said in a live region of the tab, with the item’s text when the app applies nothing', () => {
    const { panel, open, fixture } = setup({ items: [HEAL] });
    const live = panel()!.querySelector('[role="status"]')!;
    expect(live.textContent?.trim()).toBe('');
    fixture.componentRef.setInput('itemsNotice', {
      tone: 'good',
      icon: 'favorite',
      headline: 'Brisa recebeu 7 PV e acorda.',
      detail: '2d4 + 2: 3 + 2 + 2 = 7. Resta 1 Poção de cura.',
      text: [],
    } satisfies ConsumableNotice);
    fixture.detectChanges();
    open();
    expect(text(live)).toContain('Brisa recebeu 7 PV e acorda.');
    expect(text(live)).toContain('2d4 + 2: 3 + 2 + 2 = 7. Resta 1 Poção de cura.');
  });

  it('keeps the tab for the notice of the last unit, which leaves the list', () => {
    const { tabs, panel, open } = setup({
      items: [],
      itemsNotice: {
        tone: 'good',
        icon: 'favorite',
        headline: 'Tor recebeu 7 PV.',
        detail: '2d4 + 2: 3 + 2 + 2 = 7. Era a última unidade.',
        text: [],
      },
    });
    expect(tabs().map((t) => t.getAttribute('aria-label'))).toContain('Itens 0, 0 itens');
    open();
    expect(text(panel()!)).toContain('Era a última unidade.');
  });
});
