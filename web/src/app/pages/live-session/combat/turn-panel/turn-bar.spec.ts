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

  it('is one slim row: the movement left and "Encerrar turno", without the tiles above repeated', () => {
    const el = setup({ movementLeftFt: 25, movementLeftDft: 250 });
    const row = el.querySelector('.row');
    expect(text(row)).toContain('Mover 7,5 m · 5 quadrados');
    expect(text(row)).toContain('Encerrar turno');
    expect(el.querySelector('ul')).toBeNull();
    expect(text(el)).not.toContain('Ainda disponível');
    expect(text(el)).not.toContain('Ação bônus');
  });

  it('keeps only the button when there is no movement left, or no map', () => {
    const still = setup({ movementLeftFt: 0, movementLeftDft: 0 });
    expect(still.querySelector('.what')).toBeNull();
    expect(text(still.querySelector('.row'))).toContain('Encerrar turno');
    const theatre = setup({ movementLeftFt: 25, movementLeftDft: 250 }, { theatre: true });
    expect(theatre.querySelector('.what')).toBeNull();
  });

  it('says why the turn cannot end while an opportunity attack waits', () => {
    const el = setup({}, { waiting: 'Esperando a reação do mestre' });
    expect(text(el.querySelector('.row'))).toContain(
      'Esperando a reação do mestre: a vez continua quando responderem.',
    );
    expect(el.querySelector<HTMLButtonElement>('.end')?.getAttribute('aria-disabled')).toBe('true');
  });
});
