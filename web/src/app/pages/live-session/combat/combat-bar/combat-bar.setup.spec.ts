import { TestBed } from '@angular/core/testing';

import { CombatantKind, EncounterStatus } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { CombatBar } from './combat-bar';

describe('CombatBar in the initiative setup', () => {
  function setup(initiatives: Record<string, number | undefined>) {
    const fixture = TestBed.createComponent(CombatBar);
    fixture.componentRef.setInput(
      'encounter',
      encounter({
        status: EncounterStatus.SETUP,
        combatants: Object.entries(initiatives).map(([label, initiative]) =>
          combatant({ id: label.toLowerCase(), label, initiative }),
        ),
      }),
    );
    const begun = vi.fn();
    const cancelled = vi.fn();
    fixture.componentInstance.begin.subscribe(begun);
    fixture.componentInstance.cancelCombat.subscribe(cancelled);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const button = (text: string) =>
      Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes(text));
    return { fixture, el, button, begun, cancelled };
  }

  it('has "Começar o combate" as the filled button, where "Próximo turno" will be', () => {
    const { button, begun } = setup({ Toren: 14, Goblin: 12 });
    const begin = button('Começar o combate')!;
    expect(begin.classList).toContain('mat-mdc-unelevated-button');
    expect(begin.getAttribute('aria-disabled')).not.toBe('true');
    begin.click();
    expect(begun).toHaveBeenCalledTimes(1);
  });

  it('waits while someone has not rolled: dashed, aria-disabled, still focusable, pointing at the reason', () => {
    const { button, begun } = setup({ Toren: undefined, Goblin: 12 });
    const begin = button('Começar o combate')!;
    expect(begin.getAttribute('aria-disabled')).toBe('true');
    expect(begin.classList).toContain('begin--blocked');
    expect(begin.getAttribute('aria-describedby')).toBe('begin-reason');
    begin.click();
    expect(begun).not.toHaveBeenCalled();
  });

  it('asks in place before "Cancelar combate", with the focus on the safe "Voltar"', async () => {
    const { fixture, el, button, cancelled } = setup({ Toren: 14 });
    button('Cancelar combate')!.click();
    fixture.detectChanges();
    await fixture.whenStable();
    const ask = el.querySelector('[role="alertdialog"]')!;
    expect(ask.textContent).toContain('Cancelar o combate?');
    expect(ask.textContent).toContain('Ele acaba sem ter começado.');
    expect(document.activeElement?.textContent?.trim()).toBe('Voltar');
    expect(cancelled).not.toHaveBeenCalled();
    Array.from(ask.querySelectorAll('button'))
      .find((b) => b.textContent?.includes('Cancelar combate'))!
      .click();
    expect(cancelled).toHaveBeenCalledTimes(1);
  });

  it('goes back with "Voltar"', () => {
    const { fixture, el, button } = setup({ Toren: 14 });
    button('Cancelar combate')!.click();
    fixture.detectChanges();
    button('Voltar')!.click();
    fixture.detectChanges();
    expect(el.querySelector('[role="alertdialog"]')).toBeNull();
    expect(button('Começar o combate')).toBeDefined();
  });
});

describe('CombatBar while the master acts for a character', () => {
  it('says "Você age por Pensantus" and its filled button ends that turn', () => {
    const fixture = TestBed.createComponent(CombatBar);
    fixture.componentRef.setInput(
      'encounter',
      encounter({
        status: EncounterStatus.ACTIVE,
        currentCombatantId: 'p',
        combatants: [combatant({ id: 'p', label: 'Pensantus', kind: CombatantKind.PLAYER })],
      }),
    );
    fixture.componentRef.setInput('actingFor', 'Pensantus');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.textContent).toContain('Você age por Pensantus');
    const labels = [...el.querySelectorAll('button')].map((b) => b.textContent?.trim() ?? '');
    expect(labels.some((t) => t.includes('Encerrar o turno de Pensantus'))).toBe(true);
    expect(labels.some((t) => t.includes('Próximo turno'))).toBe(false);
  });
});
