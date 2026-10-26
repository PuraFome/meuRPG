import { TestBed } from '@angular/core/testing';

import { CombatantKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant } from '../../../../core/combat/combat-testing';
import { ActAsCard } from './act-as-card';
import { ActingBand } from './acting-band';

describe('ActAsCard', () => {
  it('names the character, says its player plays on their own screen and offers "Agir por"', () => {
    const fixture = TestBed.createComponent(ActAsCard);
    fixture.componentRef.setInput(
      'who',
      combatant({ id: 'p', label: 'Pensantus', kind: CombatantKind.PLAYER, armorClass: 12 }),
    );
    fixture.componentRef.setInput('title', 'Vez do Pensantus');
    fixture.componentRef.setInput('classLine', 'Mago 3');
    fixture.componentRef.setInput('hitPoints', { current: 23, max: 23 });
    const acted: string[] = [];
    fixture.componentInstance.act.subscribe(() => acted.push('act'));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const text = (el.textContent ?? '').replace(/\s+/g, ' ');
    expect(text).toContain('Vez do Pensantus');
    expect(text).toContain('Mago 3 · o jogador joga na tela dele');
    expect(text).toContain('23 de 23');
    expect(text).toContain(
      'Esperando Pensantus. Se o jogador não estiver, jogue este turno por ele.',
    );
    [...el.querySelectorAll('button')]
      .find((b) => b.textContent?.includes('Agir por Pensantus'))!
      .click();
    expect(acted).toEqual(['act']);
  });
});

describe('ActingBand', () => {
  it('says whose turn the master plays and gives the rail back with "Voltar ao mestre"', () => {
    const fixture = TestBed.createComponent(ActingBand);
    fixture.componentRef.setInput('name', 'Pensantus');
    let back = 0;
    fixture.componentInstance.back.subscribe(() => back++);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.textContent).toContain('Você está agindo por Pensantus');
    expect(el.textContent).toContain('O que você fizer aqui é dele, como se ele fizesse.');
    el.querySelector('button')!.click();
    expect(back).toBe(1);
  });
});
