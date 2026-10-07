import { TestBed } from '@angular/core/testing';

import { CombatantKind, EncounterStatus } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { InitiativeSetup } from './initiative-setup';

describe('InitiativeSetup', () => {
  const rolled = (id: string, label: string, total: number, extra = {}) =>
    combatant({
      id,
      label,
      initiative: total,
      initiativeFace: total - 2,
      initiativeBonus: 2,
      ...extra,
    });
  const list = [
    rolled('brisa', 'Brisa', 19, { kind: CombatantKind.PLAYER }),
    rolled('g1', 'Goblin 1', 12, { tieUnresolved: true }),
    rolled('g2', 'Goblin 2', 12, { tieUnresolved: true }),
    combatant({ id: 'toren', label: 'Toren', kind: CombatantKind.PLAYER, initiativeBonus: 2 }),
  ];

  function setup() {
    const fixture = TestBed.createComponent(InitiativeSetup);
    fixture.componentRef.setInput(
      'encounter',
      encounter({ status: EncounterStatus.SETUP, round: 0, combatants: list }),
    );
    const orders: string[][] = [];
    const submits: { id: string; face: number }[] = [];
    fixture.componentInstance.order.subscribe((o) => orders.push(o));
    fixture.componentInstance.submit.subscribe((s) => submits.push(s));
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, orders, submits };
  }

  it('shows the formulas and the tie once, over its two rows', () => {
    const { el } = setup();
    expect(el.textContent).toContain('1d20 (17) + 2 = 19');
    expect(el.textContent?.match(/Empate em 12/g)).toHaveLength(1);
    expect(el.textContent).toContain(
      'Empate em 12: Goblin 1 e Goblin 2. Escolha a ordem com as setas.',
    );
  });

  it('moves a tied combatant down with the arrow, and the outer arrows do nothing', () => {
    const { el, orders } = setup();
    const button = (name: string) => el.querySelector<HTMLButtonElement>(`[aria-label="${name}"]`)!;
    button('Subir Goblin 1 na ordem').click();
    expect(orders).toEqual([]);
    expect(button('Subir Goblin 1 na ordem').getAttribute('aria-disabled')).toBe('true');
    button('Descer Goblin 1 na ordem').click();
    expect(orders).toEqual([['g2', 'g1']]);
  });

  it('says who the master waits for and offers to type the roll', () => {
    const { el } = setup();
    expect(el.textContent).toContain('Esperando Toren');
    expect(el.textContent).toContain('1d20 + 2 = —');
    expect(el.textContent).toContain('Digitar pelo jogador');
  });

  it('types a face for the player: the total updates and only 1 to 20 saves', async () => {
    const { fixture, el, submits } = setup();
    Array.from(el.querySelectorAll('button'))
      .find((b) => b.textContent?.includes('Digitar pelo jogador'))!
      .click();
    fixture.detectChanges();
    const input = el.querySelector<HTMLInputElement>('input')!;
    input.value = '99';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(el.textContent).toContain('Digite um número de 1 a 20.');
    input.value = '7';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(el.textContent).toContain('= 9');
    Array.from(el.querySelectorAll('button'))
      .find((b) => b.textContent?.trim() === 'Salvar')!
      .click();
    expect(submits).toEqual([{ id: 'toren', face: 7 }]);
  });
});
