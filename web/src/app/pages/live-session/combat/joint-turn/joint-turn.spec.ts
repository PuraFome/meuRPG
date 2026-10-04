import { TestBed } from '@angular/core/testing';

import { CombatantKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { EndPart } from './end-part';
import { JointCard } from './joint-card';
import { JointOthers } from './joint-others';

const P = CombatantKind.PLAYER;
const brisa = combatant({ id: 'b', label: 'Brisa', kind: P, initiative: 19, actionUsed: false });
const toren = combatant({ id: 't', label: 'Toren', kind: P, initiative: 19, mine: true, actionUsed: true, turnPartEnded: true });
const joint = encounter({ combatants: [brisa, toren], turnGroupIds: ['b', 't'], currentCombatantId: 'b' });

describe('JointCard (the master\'s)', () => {
  it('has a block per member, a button only for who still acts, and says who is missing', () => {
    const fixture = TestBed.createComponent(JointCard);
    fixture.componentRef.setInput('encounter', joint);
    const ended: string[] = [];
    fixture.componentInstance.endPart.subscribe((id) => ended.push(id));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('h2')?.textContent).toBe('Turno conjunto: Brisa e Toren');
    expect(el.textContent).toContain('Falta a Brisa. O turno passa quando ela encerrar a parte dela.');
    expect(el.querySelectorAll('.member')).toHaveLength(2);
    expect(el.textContent).toContain('Ainda age');
    expect(el.textContent).toContain('Encerrou');
    const buttons = Array.from(el.querySelectorAll('button'));
    expect(buttons.map((b) => b.textContent?.trim())).toEqual(['Encerrar a parte da Brisa']);
    buttons[0].click();
    expect(ended).toEqual(['b']);
  });

  it('shows nothing for a turn of one, and says who has the turn when the group is over', () => {
    const fixture = TestBed.createComponent(JointCard);
    fixture.componentRef.setInput('encounter', joint);
    fixture.detectChanges();
    const alone = encounter({ combatants: [combatant({ id: 'c', label: 'Capitão Goblin' })], turnGroupIds: ['c'], currentCombatantId: 'c' });
    fixture.componentRef.setInput('encounter', alone);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('.member')).toBeNull();
    expect(el.querySelector('[role="status"]')?.textContent).toBe('A vez passou para Capitão Goblin');
  });
});

describe('JointOthers (the player\'s)', () => {
  it('says what the others still have, in words, and a summary after the own part ended', () => {
    const fixture = TestBed.createComponent(JointOthers);
    fixture.componentRef.setInput('encounter', joint);
    fixture.componentRef.setInput('mode', 'others');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('h2')?.textContent).toBe('O que a Brisa ainda tem');
    expect(el.textContent).toContain('Ainda age');
    fixture.componentRef.setInput('mode', 'summary');
    fixture.detectChanges();
    expect(el.querySelector('h2')?.textContent).toBe('Neste turno conjunto');
    expect(el.textContent).toContain('Toren');
    expect(el.textContent).toContain('(você)');
    expect(el.textContent).toContain('Já usou');
  });

  it('shows only who ended for a player outside the group', () => {
    const fixture = TestBed.createComponent(JointOthers);
    fixture.componentRef.setInput('encounter', joint);
    fixture.componentRef.setInput('mode', 'outside');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('h2')?.textContent).toBe('Quem já encerrou');
    expect(el.textContent).not.toContain('Ainda tem');
  });
});

describe('EndPart', () => {
  it('asks first with "Voltar" focused, and ends the part only on the second button', () => {
    const fixture = TestBed.createComponent(EndPart);
    fixture.componentRef.setInput('left', 'Ação e 9 m · 6 quadrados');
    const ended: number[] = [];
    fixture.componentInstance.endPart.subscribe(() => ended.push(1));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    el.querySelector('button')!.click();
    fixture.detectChanges();
    expect(ended).toEqual([]);
    expect(el.querySelector('h3')?.textContent).toBe('Encerrar a sua parte?');
    expect(el.textContent).toContain('Ainda sobram Ação e 9 m · 6 quadrados.');
    const [back, go] = Array.from(el.querySelectorAll('button'));
    expect(back.textContent?.trim()).toBe('Voltar');
    go.click();
    expect(ended).toEqual([1]);
  });
});
