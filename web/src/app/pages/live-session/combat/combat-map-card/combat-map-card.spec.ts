import { TestBed } from '@angular/core/testing';

import { CombatantKind, CombatantSide } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
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
});
