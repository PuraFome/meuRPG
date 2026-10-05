import { TestBed } from '@angular/core/testing';

import { textOf } from '../../../../core/format/text-testing';

import { CombatantKind, CombatantState } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { TurnPanel } from './turn-panel';

describe('TurnPanel', () => {
  const order = [
    combatant({ id: 'brisa', label: 'Brisa', kind: CombatantKind.PLAYER }),
    combatant({ id: 'cap', label: 'Capitão Goblin', state: CombatantState.HURT }),
    combatant({ id: 'pen', label: 'Pensantus', kind: CombatantKind.PLAYER, mine: true, speedFt: 25, movementLeftFt: 25, speedDft: 250, movementLeftDft: 250 }),
    combatant({ id: 'g1', label: 'Goblin 1', defeated: true, state: CombatantState.DEFEATED }),
  ];

  function text(over: Parameters<typeof encounter>[0]): HTMLElement {
    const fixture = TestBed.createComponent(TurnPanel);
    fixture.componentRef.setInput('encounter', encounter({ combatants: order, ...over }));
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('names another one on turn and says the player is next', () => {
    const el = text({ currentCombatantId: 'cap' });
    expect(el.querySelector('h2')?.textContent).toBe('Vez do Capitão Goblin');
    expect(el.textContent).toContain('Você é o próximo: depois dele, Pensantus.');
    expect(el.textContent).not.toContain('Mover');
  });

  it('shows state words for NPCs and never numbers', () => {
    const el = text({ currentCombatantId: 'cap' });
    const chips = Array.from(el.querySelectorAll('.chip'), (c) => c.textContent?.replace(/\s+/g, ' ').trim());
    expect(chips.join('|')).toContain('Capitão GoblinFerido');
    expect(chips.join('|')).toContain('Goblin 1Derrotado');
    expect(el.textContent).not.toMatch(/PV|\d+ de \d+/);
  });

  it('says "Vez do mestre" when the turn is hidden, with no one highlighted', () => {
    const el = text({ currentCombatantId: '', masterTurn: true });
    expect(el.querySelector('h2')?.textContent).toBe('Vez do mestre');
    expect(el.querySelector('.chip--turn')).toBeNull();
    expect(el.textContent).not.toContain('Você é o próximo');
  });

  it('is the hero on the player\'s own turn, with the movement and "Encerrar turno"', () => {
    const el = text({ currentCombatantId: 'pen' });
    expect(el.querySelector('h2')?.textContent).toBe('Sua vez, Pensantus');
    expect(el.textContent).toContain('7,5\u00a0m de 7,5\u00a0m');
    expect(el.textContent).toContain('5\u00a0quadrados livres');
    const buttons = Array.from(el.querySelectorAll('button'), (b) => b.textContent?.trim());
    // "Mover" lives in the Movimento group now.
    expect(buttons.some((b) => b?.endsWith('Mover'))).toBe(false);
    expect(buttons.some((b) => b?.endsWith('Encerrar turno'))).toBe(true);
    expect(el.textContent).toContain('Depois de você: Goblin 1'.replace('Goblin 1', 'Brisa'));
    expect(Array.from(el.querySelectorAll('.tile__name'), (n) => n.textContent)).toEqual(['Ação', 'Ação bônus', 'Reação', 'Movimento']);
  });

  it('says the turn waits for an opportunity attack, and what stopped a move short', () => {
    const fixture = TestBed.createComponent(TurnPanel);
    fixture.componentRef.setInput('encounter', encounter({ combatants: order, currentCombatantId: 'pen' }));
    fixture.componentRef.setInput('waiting', {
      title: 'Esperando a reação do mestre',
      detail: 'Seu movimento já valeu; a sua vez continua quando ele responder.',
    });
    fixture.componentRef.setInput('moveNote', 'Você parou antes: algo bloqueou o caminho.');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const notices = Array.from(el.querySelectorAll('.turn__notice'), (n) => n.textContent?.replace(/\s+/g, ' ').trim());
    expect(notices[0]).toContain('Esperando a reação do mestre. Seu movimento já valeu; a sua vez continua quando ele responder.');
    expect(notices[1]).toContain('Você parou antes: algo bloqueou o caminho.');
    expect(el.querySelectorAll('.turn__notice[role="status"]').length).toBe(2);
  });

  it('says the player is blind while looking through the familiar\'s eyes, and offers no attack (MR-036, E9-04)', () => {
    const blindOrder = order.map((c) => (c.id === 'pen' ? { ...c, familiarSightCreatureId: 'nanquim' } : c));
    const el = text({ combatants: blindOrder, currentCombatantId: 'pen' });
    expect(textOf(el.querySelector('[data-testid="familiar-blind"]'))).toBe('visibility_off Cego: para atacar, fale com o mestre.');
    // Without the sight there is no such line.
    expect(text({ currentCombatantId: 'pen' }).querySelector('[data-testid="familiar-blind"]')).toBeNull();
  });
});
