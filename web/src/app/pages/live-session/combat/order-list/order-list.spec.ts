import { TestBed } from '@angular/core/testing';

import { CombatantKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { OrderList } from './order-list';

describe('OrderList', () => {
  const rows = [
    combatant({ id: 'pen', label: 'Pensantus', kind: CombatantKind.PLAYER, characterId: 'pen-c', hitPointsCurrent: 17, hitPointsMax: 23, initiative: 14, armorClass: 13 }),
    combatant({ id: 'cap', label: 'Capitão Goblin', hitPointsCurrent: 19, hitPointsMax: 27, initiative: 16, armorClass: 17 }),
    combatant({ id: 'g3', label: 'Goblin 3', hidden: true, hitPointsCurrent: 7, hitPointsMax: 7, initiative: 9, armorClass: 15 }),
  ];

  function setup() {
    const fixture = TestBed.createComponent(OrderList);
    fixture.componentRef.setInput('encounter', encounter({ combatants: rows, currentCombatantId: 'cap' }));
    fixture.componentRef.setInput('adjustable', new Set(['pen-c']));
    const adjusted: string[] = [];
    const npcAdjusted: string[] = [];
    const revealed: { id: string; hidden: boolean }[] = [];
    fixture.componentInstance.adjust.subscribe((id) => adjusted.push(id));
    fixture.componentInstance.adjustNpc.subscribe((id) => npcAdjusted.push(id));
    fixture.componentInstance.reveal.subscribe((r) => revealed.push(r));
    fixture.detectChanges();
    return { el: fixture.nativeElement as HTMLElement, adjusted, npcAdjusted, revealed };
  }

  it('marks the turn and shows the hit points the master sees', () => {
    const { el } = setup();
    const turn = el.querySelector('.row--turn');
    expect(turn?.textContent).toContain('Capitão Goblin');
    expect(turn?.textContent).toContain('Vez');
    expect(el.textContent).toContain('17 de 23');
  });

  it('says the armor class of each combatant, which only the master gets', () => {
    const { el } = setup();
    expect(el.textContent).toContain('CA 13');
    expect(el.textContent).toContain('CA 17');
  });

  it('opens "Dano/Cura" for a player\'s character by its sheet and for an NPC by its combatant', () => {
    const { el, adjusted, npcAdjusted } = setup();
    const buttons = Array.from(el.querySelectorAll<HTMLButtonElement>('.row__adjust'));
    expect(buttons.map((b) => b.getAttribute('aria-label'))).toEqual([
      'Dano ou cura em Pensantus',
      'Dano ou cura em Capitão Goblin',
      'Dano ou cura em Goblin 3',
    ]);
    buttons[0].click();
    buttons[1].click();
    expect(adjusted).toEqual(['pen-c']);
    expect(npcAdjusted).toEqual(['cap']);
  });

  it('says a hidden NPC is only the master\'s and reveals it with one tap', () => {
    const { el, revealed } = setup();
    expect(el.textContent).toContain('Só você vê este combatente.');
    Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes('Revelar aos jogadores'))!.click();
    expect(revealed).toEqual([{ id: 'g3', hidden: false }]);
  });
});
