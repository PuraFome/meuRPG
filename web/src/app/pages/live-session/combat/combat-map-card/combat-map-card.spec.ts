import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';

import { create } from '@bufbuild/protobuf';

import {
  CombatantKind,
  CombatantSide,
  HiddenRevealQuestionSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { CombatMap } from '../../../../shared/combat-map/combat-map';
import { CombatMapCard } from './combat-map-card';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();

function mount(inputs: Record<string, unknown> = {}) {
  TestBed.resetTestingModule();
  const fixture = TestBed.createComponent(CombatMapCard);
  fixture.componentRef.setInput(
    'encounter',
    encounter({
      currentCombatantId: 'p',
      combatants: [
        combatant({
          id: 'p',
          label: 'Pensantus',
          kind: CombatantKind.PLAYER,
          side: CombatantSide.PARTY,
        }),
      ],
    }),
  );
  fixture.componentRef.setInput('image', { url: '', width: 100, height: 100 });
  fixture.componentRef.setInput('isMaster', true);
  fixture.componentRef.setInput('mapName', 'A caverna do Vale Seco');
  for (const [name, value] of Object.entries(inputs)) {
    fixture.componentRef.setInput(name, value);
  }
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

describe('CombatMapCard: "Movimento forçado"', () => {
  it('is not drawn unless the page offers it (the setup map, a player)', () => {
    const { el } = mount();
    expect(plain(el.textContent)).not.toContain('Movimento forçado');
    expect(el.querySelector('[role="status"]')).toBeNull();
  });

  it('is a box of 44 px that says what it does, after the range box, off by default', () => {
    const { el } = mount({
      forcedSwitch: true,
      reachSwitch: { name: 'do Pensantus', on: false },
    });
    const boxes = [...el.querySelectorAll<HTMLInputElement>('label.card__check input')];
    expect(boxes.map((b) => plain(b.parentElement?.textContent))).toEqual([
      'Mostrar o alcance do Pensantus no mapa',
      'Movimento forçado',
    ]);
    expect(boxes[1].checked).toBe(false);
    expect(plain(el.textContent)).toContain('Vale só para o próximo arrasto.');
    expect(plain(el.textContent)).toContain(
      'Arraste um token para movê-lo. O mestre anda sem limite.',
    );
    expect(el.querySelector('.card__state')).toBeNull();
  });

  it('on, says so in words and changes the hint of the map; the box tells the page', () => {
    const { fixture, el } = mount({ forcedSwitch: true });
    const changes: boolean[] = [];
    fixture.componentInstance.forcedChange.subscribe((v) => changes.push(v));
    const box = [...el.querySelectorAll<HTMLInputElement>('label.card__check input')].pop()!;
    box.click();
    expect(changes).toEqual([true]);
    fixture.componentRef.setInput('forced', true);
    fixture.detectChanges();
    expect(box.checked).toBe(true);
    expect(plain(el.querySelector('.card__state')?.textContent)).toBe(
      'Ligado para o próximo arrasto',
    );
    expect(plain(el.textContent)).toContain(
      'Arraste um token: o movimento é forçado e não provoca ataque de oportunidade.',
    );
    expect(plain(el.textContent)).not.toContain('O mestre anda sem limite.');
  });

  it('keeps the live region on the page, empty, and fills it with the result of a forced drag', () => {
    const { fixture, el } = mount({ forcedSwitch: true });
    const region = el.querySelector('[role="status"]')!;
    expect(plain(region.textContent)).toBe('');
    fixture.componentRef.setInput(
      'forcedNote',
      'Pensantus foi movido à força. Ninguém recebeu oferta de ataque de oportunidade.',
    );
    fixture.detectChanges();
    expect(el.querySelector('[role="status"]')).toBe(region);
    expect(plain(region.textContent)).toContain('Pensantus foi movido à força.');
  });

  it("draws its checkboxes with the design system's visible outline, not the browser's", () => {
    const { el } = mount({ forcedSwitch: true, reachSwitch: { name: 'do Pensantus', on: false } });
    const boxes = el.querySelectorAll('label.card__check input[type="checkbox"]');
    expect(boxes).toHaveLength(2);
    boxes.forEach((b) => expect(b.classList).toContain('mr-check-input'));
  });

  describe('the dragged token of a forced move', () => {
    type Drag = { set(v: { id: string; col: number; row: number } | null): void };
    function dragged(forced: boolean, to: { col: number; row: number }) {
      const { fixture, el } = mount({ forcedSwitch: true, forced });
      const map = fixture.debugElement.query(By.directive(CombatMap))
        .componentInstance as unknown as {
        drag: Drag;
      };
      map.drag.set({ id: 'p', ...to });
      fixture.detectChanges();
      return el;
    }

    it('marks the square the token left with a dashed ring and a dashed line from it', () => {
      const el = dragged(true, { col: 5, row: 4 });
      expect(el.querySelector('.cm__origin')).not.toBeNull();
      expect(el.querySelectorAll('.cm__path line')).toHaveLength(1);
    });

    it('draws nothing while the box is off, or while the token is still on its square', () => {
      expect(dragged(false, { col: 5, row: 4 }).querySelector('.cm__origin')).toBeNull();
      expect(dragged(true, { col: 0, row: 0 }).querySelector('.cm__origin')).toBeNull();
    });
  });
});

describe('CombatMapCard: "Área da última magia"', () => {
  const asking = () =>
    encounter({
      currentCombatantId: 'p',
      gridColumns: 10,
      gridRows: 8,
      combatants: [
        combatant({
          id: 'p',
          label: 'Pensantus',
          kind: CombatantKind.PLAYER,
          side: CombatantSide.PARTY,
        }),
      ],
      pendingHiddenReveals: [
        create(HiddenRevealQuestionSchema, {
          id: 'q1',
          casterId: 'p',
          spellKey: 'spell:fireball',
          combatantIds: ['g3'],
          area: {
            origin: { col: 4, row: 3 },
            squares: [
              { col: 4, row: 3 },
              { col: 5, row: 3 },
            ],
          },
        }),
      ],
    });

  it("draws the master's pending area and origin, with their two legend entries", () => {
    const { el } = mount({ encounter: asking() });
    expect(el.querySelector('app-area-overlay')).not.toBeNull();
    expect(el.querySelector('app-area-overlay .ao__fill')?.getAttribute('d')).toBe(
      'M4 3h1v1h-1zM5 3h1v1h-1z',
    );
    expect(el.querySelectorAll('app-area-overlay .ao__diamond')).toHaveLength(1);
    const legend = plain(el.querySelector('.legend')?.textContent);
    expect(legend).toContain('Área da última magia');
    expect(legend).toContain('Ponto de origem');
  });

  it('draws nothing for a player, and nothing without a question', () => {
    const { el } = mount({ encounter: asking(), isMaster: false });
    expect(el.querySelector('app-area-overlay')).toBeNull();
    expect(plain(el.querySelector('.legend')?.textContent)).not.toContain('Área da última magia');
    expect(mount().el.querySelector('app-area-overlay')).toBeNull();
  });
});
