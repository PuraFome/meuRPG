import { TestBed } from '@angular/core/testing';

import { CombatantKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { mineTabs } from '../../../../core/combat/mine';
import { flat } from '../../../../core/creatures/creatures-testing';
import { MineTabs } from './mine-tabs';

describe('MineTabs (E9-12)', () => {
  const salvia = combatant({
    id: 's',
    label: 'Sálvia',
    kind: CombatantKind.PLAYER,
    mine: true,
    controlledByMe: true,
    initiative: 13,
  });
  const wolf = (n: number) =>
    combatant({
      id: `w${n}`,
      label: `Lobo atroz ${n}`,
      kind: CombatantKind.CREATURE,
      controlledByMe: true,
      summonGroupId: 'c',
      monsterKey: 'k',
      monsterNamePt: 'Lobo atroz',
      initiative: 10,
    });

  function setup() {
    const e = encounter({
      combatants: [salvia, wolf(1), wolf(2)],
      currentCombatantId: 'w1',
      turnGroupIds: ['w1', 'w2'],
    });
    const fixture = TestBed.createComponent(MineTabs);
    fixture.componentRef.setInput('tabs', mineTabs(e));
    fixture.componentRef.setInput('selected', 'c');
    const picked: string[] = [];
    fixture.componentInstance.select.subscribe((id) => picked.push(id));
    fixture.detectChanges();
    return { el: fixture.nativeElement as HTMLElement, picked, fixture };
  }

  it('is a tablist with a tab each, the word of each and the chosen one marked', () => {
    const { el } = setup();
    expect(el.querySelector('[role=tablist]')).not.toBeNull();
    const tabs = Array.from(el.querySelectorAll<HTMLElement>('[role=tab]'));
    expect(tabs.map((t) => flat(t))).toEqual([
      'Sálvia Já agiu · 13',
      'Lobos atrozes (2) Sua vez · 10',
    ]);
    expect(tabs.map((t) => t.getAttribute('aria-selected'))).toEqual(['false', 'true']);
  });

  it('a tap picks the tab; the arrows move between them', () => {
    const { el, picked, fixture } = setup();
    const tabs = Array.from(el.querySelectorAll<HTMLButtonElement>('[role=tab]'));
    tabs[0].click();
    tabs[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    tabs[1].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }));
    fixture.detectChanges();
    expect(picked).toEqual(['character', 'c', 'character']);
  });
});
