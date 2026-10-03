import { TestBed } from '@angular/core/testing';

import { NextTurn } from './next-turn';

describe('NextTurn', () => {
  function setup(pendingNote: string | null) {
    const fixture = TestBed.createComponent(NextTurn);
    fixture.componentRef.setInput('pendingNote', pendingNote);
    const sent: boolean[] = [];
    fixture.componentInstance.next.subscribe((discard) => sent.push(discard));
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, sent };
  }

  it('passes the turn at once when nothing is owed', () => {
    const { el, sent } = setup(null);
    el.querySelector<HTMLButtonElement>('.next')!.click();
    expect(sent).toEqual([false]);
  });

  it('asks in place when a damage is owed, "Voltar" keeps the turn and "Passar o turno" discards it', async () => {
    const { fixture, el, sent } = setup('Falta aplicar 5 de dano');
    el.querySelector<HTMLButtonElement>('.next')!.click();
    fixture.detectChanges();
    await fixture.whenStable();
    expect(sent).toEqual([]);
    expect(el.querySelector('[role="alertdialog"]')?.textContent).toContain('Há dano sem aplicar. Passar o turno mesmo assim?');
    const [back, go] = Array.from(el.querySelectorAll<HTMLButtonElement>('.ask__buttons button'));
    expect(back.textContent?.trim()).toBe('Voltar');
    back.click();
    fixture.detectChanges();
    expect(el.querySelector('[role="alertdialog"]')).toBeNull();
    el.querySelector<HTMLButtonElement>('.next')!.click();
    fixture.detectChanges();
    el.querySelectorAll<HTMLButtonElement>('.ask__buttons button')[1].click();
    expect(sent).toEqual([true]);
    expect(go.textContent?.trim()).toBe('Passar o turno');
  });
});
