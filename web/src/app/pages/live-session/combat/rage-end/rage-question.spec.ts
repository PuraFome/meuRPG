import { TestBed } from '@angular/core/testing';

import { combatant } from '../../../../core/combat/combat-testing';
import { RageQuestion } from './rage-question';

const toren = combatant({ id: 't', label: 'Toren' });

function setup(master: boolean, subject = toren) {
  const fixture = TestBed.createComponent(RageQuestion);
  fixture.componentRef.setInput('subject', subject);
  fixture.componentRef.setInput('master', master);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

const buttons = (el: HTMLElement) => Array.from(el.querySelectorAll<HTMLButtonElement>('button'));

describe('RageQuestion', () => {
  it("asks the owner 'A sua fúria vai acabar?' in an alertdialog named by the question", () => {
    const { el } = setup(false);
    const dialog = el.querySelector('[role="alertdialog"]')!;
    expect(dialog.getAttribute('aria-labelledby')).toBe('rage-q');
    expect(el.querySelector('#rage-q')?.textContent).toContain('A sua fúria vai acabar?');
  });

  it('asks the master about the character by name', () => {
    const { el } = setup(true);
    expect(el.querySelector('#rage-q')?.textContent).toContain('A fúria de Toren vai acabar?');
  });

  it('answers with the id for each button', () => {
    const { fixture, el } = setup(false);
    const back: string[] = [];
    const end: string[] = [];
    fixture.componentInstance.back.subscribe((id) => back.push(id));
    fixture.componentInstance.letEnd.subscribe((id) => end.push(id));
    const [stay, letEnd] = buttons(el);
    expect(stay.textContent).toContain('Voltar e atacar');
    expect(letEnd.textContent).toContain('Deixar a fúria acabar');
    stay.click();
    letEnd.click();
    expect(back).toEqual(['t']);
    expect(end).toEqual(['t']);
  });

  it('puts the focus on the answer that loses nothing', async () => {
    const { fixture, el } = setup(false);
    await fixture.whenStable();
    expect(document.activeElement).toBe(buttons(el)[0]);
  });

  it('goes back to the turn on Escape', () => {
    const { fixture, el } = setup(false);
    const back: string[] = [];
    fixture.componentInstance.back.subscribe((id) => back.push(id));
    el.querySelector('[role="alertdialog"]')!.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape' }),
    );
    expect(back).toEqual(['t']);
  });

  it('holds the buttons while a call is in the air', () => {
    const { fixture, el } = setup(false);
    fixture.componentRef.setInput('busy', true);
    fixture.detectChanges();
    expect(buttons(el).every((b) => b.disabled)).toBe(true);
  });

  it('draws nothing without a combatant', () => {
    const fixture = TestBed.createComponent(RageQuestion);
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('[role="alertdialog"]')).toBeNull();
  });
});
