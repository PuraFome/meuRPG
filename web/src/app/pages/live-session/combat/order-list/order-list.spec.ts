import { TestBed } from '@angular/core/testing';

import { CombatantKind, CombatantSide, CoverDegree, CoverSource } from '../../../../../gen/meurpg/play/v1/combat_pb';
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

  describe('cover and sides (E9-07)', () => {
    function withCover(over: { marked?: CoverDegree; ally?: boolean } = {}) {
      const fixture = TestBed.createComponent(OrderList);
      const g2 = combatant({
        id: 'g2',
        label: 'Goblin 2',
        hitPointsCurrent: 7,
        hitPointsMax: 7,
        initiative: 12,
        coverMark: over.marked ?? CoverDegree.NONE,
        side: over.ally ? CombatantSide.PARTY : CombatantSide.ENEMY,
      });
      fixture.componentRef.setInput('encounter', encounter({ combatants: [rows[0], rows[1], g2], currentCombatantId: 'pen' }));
      fixture.componentRef.setInput(
        'coverAgainst',
        new Map([
          ['cap', { cover: CoverDegree.THREE_QUARTERS, source: CoverSource.MAP }],
          ['g2', { cover: CoverDegree.HALF, source: CoverSource.MAP }],
        ]),
      );
      fixture.componentRef.setInput('turnLabel', 'Pensantus');
      const picked: { id: string; cover: CoverDegree }[] = [];
      const sides: { id: string; side: CombatantSide }[] = [];
      fixture.componentInstance.cover.subscribe((c) => picked.push(c));
      fixture.componentInstance.side.subscribe((c) => sides.push(c));
      fixture.detectChanges();
      return { fixture, el: fixture.nativeElement as HTMLElement, picked, sides };
    }

    const plain = (t: string | null | undefined) => (t ?? '').replace(/\u00a0/g, ' ').replace(/\s+/g, ' ').trim();

    it('says each enemy\'s cover against whoever has the turn, with its source', () => {
      const { el } = withCover();
      const lines = Array.from(el.querySelectorAll('app-row-cover .cover'), (c) => plain(c.textContent));
      expect(lines).toEqual(['Três quartos (do mapa) contra o Pensantus', 'Meia cobertura (do mapa) contra o Pensantus']);
    });

    it('opens the mark in place, a radio group named for the combatant, applying at once', () => {
      const { fixture, el, picked } = withCover({ marked: CoverDegree.HALF });
      const open = Array.from(el.querySelectorAll<HTMLButtonElement>('.action')).find((b) => b.getAttribute('aria-label') === 'Marcar cobertura de Goblin 2')!;
      open.click();
      fixture.detectChanges();
      const group = el.querySelector('[role="radiogroup"]')!;
      expect(group.getAttribute('aria-label')).toBe('Cobertura marcada de Goblin 2');
      const radios = Array.from(group.querySelectorAll<HTMLInputElement>('input'));
      expect(radios.map((r) => r.checked)).toEqual([false, true, false, false]);
      radios[2].click();
      expect(picked).toEqual([{ id: 'g2', cover: CoverDegree.THREE_QUARTERS }]);
      Array.from(el.querySelectorAll('button')).find((b) => plain(b.textContent) === 'Fechar')!.click();
      fixture.detectChanges();
      expect(el.querySelector('[role="radiogroup"]')).toBeNull();
    });

    it('says "Aliado" as a side, apart from the conditions, and leaves the mark to the cover line', () => {
      const { el } = withCover({ marked: CoverDegree.THREE_QUARTERS, ally: true });
      const row = Array.from(el.querySelectorAll('li.row')).find((r) => r.textContent?.includes('Goblin 2'))!;
      expect(plain(row.querySelector('.row__meta')?.textContent)).toContain('Aliado');
      // Not inside "Condições de Goblin 2", and the master's list has no mark pill: the line says it.
      expect(Array.from(row.querySelectorAll('.tag'), (t) => plain(t.textContent))).toEqual([]);
    });
  });
});
