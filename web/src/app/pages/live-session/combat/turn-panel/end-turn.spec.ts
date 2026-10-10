import { TestBed } from '@angular/core/testing';

import { EndTurn } from './end-turn';

describe('EndTurn', () => {
  function setup(own: { actionUsed: boolean; bonusActionUsed: boolean; conditions?: string[] }) {
    const fixture = TestBed.createComponent(EndTurn);
    fixture.componentRef.setInput('own', own);
    const ended: number[] = [];
    fixture.componentInstance.endTurn.subscribe(() => ended.push(1));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { fixture, el, ended, end: () => el.querySelector<HTMLButtonElement>('.end')! };
  }

  it('is an outline while the action or the bonus action is still available (timeline.md, decision 2)', () => {
    expect(setup({ actionUsed: false, bonusActionUsed: false }).end().classList).not.toContain(
      'end--filled',
    );
    expect(setup({ actionUsed: true, bonusActionUsed: false }).end().classList).not.toContain(
      'end--filled',
    );
  });

  it('ends the turn at once, without asking, when a condition leaves no action to use', () => {
    const { fixture, el, ended, end } = setup({
      actionUsed: false,
      bonusActionUsed: false,
      conditions: ['condition:paralyzed'],
    });
    expect(end().classList).toContain('end--filled');
    end().click();
    fixture.detectChanges();
    expect(el.querySelector('[role="alertdialog"]')).toBeNull();
    expect(ended).toEqual([1]);
  });

  it('ignores the attacks the server still counts when a condition leaves no action to use', () => {
    const { fixture, el, ended, end } = setup({
      actionUsed: false,
      bonusActionUsed: false,
      conditions: ['condition:paralyzed'],
    });
    fixture.componentRef.setInput('attacksLeft', 1);
    fixture.detectChanges();
    expect(end().classList).toContain('end--filled');
    end().click();
    fixture.detectChanges();
    expect(el.querySelector('[role="alertdialog"]')).toBeNull();
    expect(ended).toEqual([1]);
  });

  it('is the filled button once both are used', () => {
    expect(setup({ actionUsed: true, bonusActionUsed: true }).end().classList).toContain(
      'end--filled',
    );
  });

  it('asks in place with the action unused, the focus on "Voltar", and ends only when confirmed', async () => {
    const { fixture, el, ended, end } = setup({ actionUsed: false, bonusActionUsed: false });
    end().click();
    fixture.detectChanges();
    await fixture.whenStable();
    expect(ended).toEqual([]);
    expect(el.querySelector('[role="alertdialog"]')?.textContent).toContain(
      'Ainda tem ação disponível. Encerrar mesmo?',
    );
    const [back, go] = Array.from(el.querySelectorAll<HTMLButtonElement>('.ask__btn'));
    expect(back.textContent?.trim()).toBe('Voltar');
    back.click();
    fixture.detectChanges();
    expect(el.querySelector('[role="alertdialog"]')).toBeNull();
    end().click();
    fixture.detectChanges();
    el.querySelectorAll<HTMLButtonElement>('.ask__btn')[1].click();
    expect(ended).toEqual([1]);
    expect(go.textContent?.trim()).toBe('Encerrar turno');
  });

  it('ends at once when the action is used', () => {
    const { end, ended } = setup({ actionUsed: true, bonusActionUsed: false });
    end().click();
    expect(ended).toEqual([1]);
  });

  it('while a reaction is awaited it stays reachable, dashed, with the reason read with it', () => {
    const { fixture, el, ended, end } = setup({ actionUsed: true, bonusActionUsed: true });
    fixture.componentRef.setInput('waiting', 'Esperando o mestre');
    fixture.detectChanges();
    expect(end().getAttribute('aria-disabled')).toBe('true');
    expect(end().classList).toContain('end--off');
    expect(end().tabIndex).toBeGreaterThanOrEqual(0);
    const why = el.querySelector(`#${end().getAttribute('aria-describedby')}`);
    expect(why?.textContent).toBe('Esperando o mestre: a vez continua quando responderem.');
    end().click();
    expect(ended).toEqual([]);
  });

  it('passes the turn of a surprised character at once, with nothing to ask (W7-X)', () => {
    const { fixture, el, ended, end } = setup({ actionUsed: false, bonusActionUsed: false });
    fixture.componentRef.setInput('surprised', true);
    fixture.componentRef.setInput('label', 'Passar o turno');
    fixture.detectChanges();
    expect(end().classList).toContain('end--filled');
    expect(end().textContent?.trim()).toContain('Passar o turno');
    end().click();
    expect(ended).toEqual([1]);
    expect(el.querySelector('[role="alertdialog"]')).toBeNull();
  });
});
