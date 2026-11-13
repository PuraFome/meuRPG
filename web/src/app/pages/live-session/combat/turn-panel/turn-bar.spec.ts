import { TestBed } from '@angular/core/testing';

import { combatant } from '../../../../core/combat/combat-testing';
import { TurnBar } from './turn-bar';

describe('TurnBar', () => {
  function setup(over: Record<string, unknown> = {}, inputs: Record<string, unknown> = {}) {
    const fixture = TestBed.createComponent(TurnBar);
    fixture.componentRef.setInput('own', combatant({ id: 'p', label: 'Pensantus', ...over }));
    for (const [name, value] of Object.entries(inputs)) {
      fixture.componentRef.setInput(name, value);
    }
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }
  const text = (el: Element | null) => (el?.textContent ?? '').replace(/\s+/g, ' ').trim();

  it('holds only the one filled "Encerrar turno": the economy is the first row of "Ações", not the bar\'s (584-01)', () => {
    const el = setup({ movementLeftFt: 30, movementLeftDft: 300, bonusActionUsed: true });
    const row = el.querySelector('.row');
    expect(text(row)).toBe('flagEncerrar turno');
    expect(text(el)).not.toContain('Ainda tem');
    expect(el.querySelector('li')).toBeNull();
    expect(text(el)).not.toContain('Mover');
    expect(el.querySelector('button.end--filled')).not.toBeNull();
    expect(el.querySelector('button.end--block')).not.toBeNull();
  });

  it('is the same bar with everything spent: nothing but the button', () => {
    const el = setup({
      actionUsed: true,
      bonusActionUsed: true,
      reactionUsed: true,
      movementLeftFt: 0,
      movementLeftDft: 0,
    });
    expect(el.querySelector('ul')).toBeNull();
    expect(text(el.querySelector('.row'))).toBe('flagEncerrar turno');
  });

  it('says why the turn cannot end while an opportunity attack waits', () => {
    const el = setup({}, { waiting: 'Esperando a reação do mestre' });
    expect(text(el.querySelector('.row'))).toContain(
      'Esperando a reação do mestre: a vez continua quando responderem.',
    );
    expect(el.querySelector<HTMLButtonElement>('.end')?.getAttribute('aria-disabled')).toBe('true');
  });

  it("leaves the master's wait to the turn card: the bar shows what is left, and the button stays off", () => {
    const el = setup({}, { waiting: 'Esperando o mestre' });
    expect(text(el)).not.toContain('Esperando o mestre');
    expect(text(el.querySelector('.row'))).toBe('flagEncerrar turno');
    expect(el.querySelector<HTMLButtonElement>('.end')?.getAttribute('aria-disabled')).toBe('true');
  });

  it('says "Passar o turno" for a surprised character (W7-X)', () => {
    const el = setup({}, { surprised: true });
    expect(text(el.querySelector('.row'))).toContain('Passar o turno');
    expect(text(el.querySelector('.row'))).not.toContain('Encerrar turno');
  });
});
