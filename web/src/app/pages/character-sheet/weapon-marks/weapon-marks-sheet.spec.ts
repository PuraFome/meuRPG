import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { WeaponMarksSheet, type WeaponChange, type WeaponMarksData } from './weapon-marks-sheet';

const none = { silvered: false, adamantine: false, magical: false };

function setup(over: Partial<WeaponMarksData> = {}) {
  const closed: (boolean | undefined)[] = [];
  const saves: WeaponChange[] = [];
  const data: WeaponMarksData = {
    weaponName: 'Espada curta',
    characterName: 'Tor',
    isMaster: false,
    marks: { ...none, silvered: true },
    bonus: 1,
    copies: 1,
    save: (c) => {
      saves.push(c);
      return Promise.resolve('');
    },
    ...over,
  };
  TestBed.configureTestingModule({
    providers: [
      { provide: MAT_DIALOG_DATA, useValue: data },
      {
        provide: MatDialogRef,
        useValue: { close: (r?: boolean) => closed.push(r), disableClose: false },
      },
    ],
  });
  const fixture = TestBed.createComponent(WeaponMarksSheet);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const box = (word: string) =>
    Array.from(el.querySelectorAll('.marks__row'))
      .find((r) => r.querySelector('.marks__word')!.textContent === word)!
      .querySelector('input')!;
  const radio = (word: string) =>
    Array.from(el.querySelectorAll('.bg__opt'))
      .find((l) => l.textContent!.includes(word))!
      .querySelector('input')!;
  const choose = (word: string) => {
    const input = radio(word);
    input.checked = true;
    input.dispatchEvent(new Event('change', { bubbles: true }));
    fixture.detectChanges();
  };
  const closeFrame = () =>
    fixture.debugElement.query(By.css('app-sheet-frame')).triggerEventHandler('closed');
  const text = (sel: string) => el.querySelector(sel)!.textContent!.replace(/\s+/g, ' ').trim();
  const press = (label: string) =>
    Array.from(el.querySelectorAll('button'))
      .find((b) => b.textContent!.includes(label))!
      .click();
  return { fixture, el, box, radio, text, press, choose, closeFrame, saves, closed };
}

describe('WeaponMarksSheet ("Marcas · <arma>")', () => {
  it('is titled "Marcas · <arma>" and says "A arma é:"', () => {
    const { el, text } = setup();
    expect(el.querySelector('h2')!.textContent).toBe('Marcas · Espada curta');
    expect(text('.marks__intro')).toBe('A arma é:');
  });

  it('has one row for each mark, then the bonus group and its help line', () => {
    const { el, text } = setup();
    const rows = Array.from(el.querySelectorAll('.marks__row')).map((r) => ({
      word: r.querySelector('.marks__word')!.textContent,
      icon: r.querySelector('mat-icon')!.textContent,
    }));
    expect(rows).toEqual([
      { word: 'Prateada', icon: 'dark_mode' },
      { word: 'Adamantina', icon: 'diamond' },
      { word: 'Mágica', icon: 'auto_awesome' },
    ]);
    expect(el.querySelector('fieldset legend')!.textContent).toBe('Bônus mágico');
    expect(text('.marks__help')).toBe('Soma no ataque e no dano, e a arma conta como mágica.');
  });

  it('says the copies at the top only when there are several', () => {
    expect(setup().el.querySelector('.marks__copies')).toBeNull();
    TestBed.resetTestingModule();
    const { text, el } = setup({ copies: 2 });
    expect(text('.marks__copies')).toBe(
      'info A ficha tem 2 cópias: marcas e bônus valem para as duas.',
    );
    expect(el.querySelector('.marks__copies')!.nextElementSibling!.className).toBe('marks__intro');
  });

  it('says "para todas" for more than two copies', () => {
    const { text } = setup({ copies: 3 });
    expect(text('.marks__copies')).toBe(
      'info A ficha tem 3 cópias: marcas e bônus valem para todas.',
    );
  });

  it('shows the reason with a block icon only when there is one', () => {
    expect(setup().el.querySelector('.marks__reason')).toBeNull();
    TestBed.resetTestingModule();
    const { text } = setup({ reason: 'Carregado, não equipado.' });
    expect(text('.marks__reason')).toBe('blockCarregado, não equipado.');
  });

  it('saves marks and bonus together with the one "Salvar"', async () => {
    const { box, radio, choose, press, saves, closed, fixture, el } = setup();
    expect(box('Prateada').checked).toBe(true);
    expect(radio('+1').checked).toBe(true);
    box('Mágica').checked = true;
    box('Mágica').dispatchEvent(new Event('change'));
    choose('+3');
    expect(radio('+3').checked).toBe(true);
    expect(radio('+1').checked).toBe(false);
    expect(el.textContent).not.toContain('Salvar marcas');
    press('Salvar');
    await fixture.whenStable();
    expect(saves).toEqual([
      { marks: { silvered: true, adamantine: false, magical: true }, bonus: 3 },
    ]);
    expect(closed).toEqual([true]);
  });

  it('saves 0 for "Nenhum"', async () => {
    const { choose, press, saves, fixture } = setup();
    choose('Nenhum');
    press('Salvar');
    await fixture.whenStable();
    expect(saves[0].bonus).toBe(0);
  });

  it('is muted, inert and cannot be dismissed while saving', () => {
    let release!: (why: string) => void;
    const { fixture, el, box, press, closeFrame, closed } = setup({
      save: () => new Promise<string>((r) => (release = r)),
    });
    press('Salvar');
    fixture.detectChanges();
    expect(el.querySelector('fieldset')!.disabled).toBe(true);
    expect(box('Prateada').disabled).toBe(true);
    expect(el.querySelector('.pair__btn')!.textContent).toContain('Salvando…');
    expect(el.querySelector('.pair__spin')).not.toBeNull();
    closeFrame();
    fixture.componentInstance['cancel']();
    expect(closed).toEqual([]);
    release('x');
  });

  it('saves once when "Salvar" is pressed twice', async () => {
    const calls: WeaponChange[] = [];
    const { press, fixture } = setup({
      save: (c) => {
        calls.push(c);
        return new Promise<string>(() => undefined);
      },
    });
    press('Salvar');
    press('Salvar');
    await fixture.whenStable();
    expect(calls).toHaveLength(1);
  });

  it('says the refusal and stays open with the choices as they were, "Salvar" back', async () => {
    const { el, box, radio, choose, press, closed, fixture } = setup({
      save: () => Promise.resolve('A ficha mudou enquanto você editava.'),
    });
    box('Adamantina').checked = true;
    box('Adamantina').dispatchEvent(new Event('change'));
    choose('+2');
    press('Salvar');
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('[role="alert"]')!.textContent).toContain(
      'A ficha mudou enquanto você editava.',
    );
    expect(box('Adamantina').checked).toBe(true);
    expect(radio('+2').checked).toBe(true);
    expect(el.querySelector('.pair__btn')!.textContent).toContain('Salvar');
    expect(el.querySelector('.pair__btn')!.textContent).not.toContain('Salvando');
    expect(closed).toEqual([]);
  });

  it('tells the owner when the sheet locks, and the master whose sheet it is', () => {
    expect(setup().text('.lock')).toBe(
      'lockO dono só muda enquanto a ficha não trava. Depois, só o mestre.',
    );
    TestBed.resetTestingModule();
    const m = setup({ isMaster: true });
    expect(m.text('.lock')).toContain(
      'Muda a ficha de Tor: depois da trava, só você muda marcas e bônus. Tor os lê na ficha e nos ataques.',
    );
    expect(m.text('.marks__intro')).toBe('Tor · a arma é:');
  });

  it('has "Salvar" and "Cancelar" of one size in a dialog, and "Cancelar" closes without saving', () => {
    const { el, press, saves, closed } = setup();
    const buttons = Array.from(el.querySelectorAll<HTMLButtonElement>('.pair__btn'));
    expect(buttons.map((b) => b.textContent!.replace(/\s+/g, ' ').trim())).toEqual([
      'save Salvar',
      'Cancelar',
    ]);
    expect(el.querySelector('.pair')!.getAttribute('data-equal-width')).toBe('.pair__btn');
    press('Cancelar');
    expect(saves).toEqual([]);
    expect(closed).toEqual([undefined]);
  });
});
