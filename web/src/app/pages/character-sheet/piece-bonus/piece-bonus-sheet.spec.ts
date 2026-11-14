import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { PieceBonusSheet, type PieceBonusData } from './piece-bonus-sheet';

function setup(over: Partial<PieceBonusData> = {}) {
  const closed: (boolean | undefined)[] = [];
  const saves: number[] = [];
  const data: PieceBonusData = {
    pieceName: 'Escudo',
    kind: 'shield',
    characterName: 'Tor',
    isMaster: false,
    bonus: 2,
    save: (b) => {
      saves.push(b);
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
  const fixture = TestBed.createComponent(PieceBonusSheet);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
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
  return { fixture, el, radio, text, press, choose, closeFrame, saves, closed };
}

describe('PieceBonusSheet ("Bônus · <peça>")', () => {
  it('is titled with the piece and describes what the bonus adds to', () => {
    const shield = setup();
    expect(shield.el.querySelector('h2')!.textContent).toBe('Bônus · Escudo');
    expect(shield.text('.bonus__intro')).toBe('Soma na CA com o escudo em uso.');
    TestBed.resetTestingModule();
    const armor = setup({ kind: 'armor', pieceName: 'Cota de malha' });
    expect(armor.text('.bonus__intro')).toBe('Soma na CA enquanto a armadura está equipada.');
  });

  it('starts the master’s description with the character', () => {
    expect(setup({ isMaster: true }).text('.bonus__intro')).toBe(
      'Tor · soma na CA com o escudo em uso.',
    );
    TestBed.resetTestingModule();
    expect(setup({ isMaster: true, kind: 'armor' }).text('.bonus__intro')).toBe(
      'Tor · soma na CA enquanto a armadura está equipada.',
    );
  });

  it('starts from the stored bonus and saves the chosen number', async () => {
    const { radio, choose, press, saves, closed, fixture } = setup();
    expect(radio('+2').checked).toBe(true);
    choose('+3');
    expect(radio('+3').checked).toBe(true);
    expect(radio('+2').checked).toBe(false);
    press('Salvar');
    await fixture.whenStable();
    expect(saves).toEqual([3]);
    expect(closed).toEqual([true]);
  });

  it('saves 0 for "Nenhum"', async () => {
    const { choose, press, saves, fixture } = setup();
    choose('Nenhum');
    press('Salvar');
    await fixture.whenStable();
    expect(saves).toEqual([0]);
  });

  it('shows the reason only when the bonus does not count', () => {
    expect(setup().el.querySelector('.bonus__reason')).toBeNull();
    TestBed.resetTestingModule();
    expect(setup({ reason: 'Pede sintonização.' }).text('.bonus__reason')).toBe(
      'blockPede sintonização.',
    );
  });

  it('is muted and inert while saving, and cannot be closed', () => {
    let release!: (why: string) => void;
    const { fixture, el, press, closeFrame, closed } = setup({
      save: () => new Promise<string>((r) => (release = r)),
    });
    press('Salvar');
    fixture.detectChanges();
    expect(el.querySelector('fieldset')!.disabled).toBe(true);
    expect(el.querySelector('.pair__btn')!.textContent).toContain('Salvando…');
    closeFrame();
    fixture.componentInstance['cancel']();
    expect(closed).toEqual([]);
    release('x');
  });

  it('saves once when "Salvar" is pressed twice', async () => {
    const calls: number[] = [];
    const { press, fixture } = setup({
      save: (b) => {
        calls.push(b);
        return new Promise<string>(() => undefined);
      },
    });
    press('Salvar');
    press('Salvar');
    await fixture.whenStable();
    expect(calls).toEqual([2]);
  });

  it('says the refusal in an alert, keeps the choice and puts "Salvar" back', async () => {
    const { el, radio, choose, press, closed, fixture } = setup({
      save: () => Promise.resolve('A ficha mudou enquanto você editava.'),
    });
    choose('+1');
    press('Salvar');
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('[role="alert"]')!.textContent).toContain(
      'A ficha mudou enquanto você editava.',
    );
    expect(radio('+1').checked).toBe(true);
    expect(el.querySelector('.pair__btn')!.textContent).toContain('Salvar');
    expect(closed).toEqual([]);
  });

  it('tells the owner and the master who changes it after the lock', () => {
    expect(setup().text('.lock')).toBe(
      'lockO dono só muda enquanto a ficha não trava. Depois, só o mestre.',
    );
    TestBed.resetTestingModule();
    expect(setup({ isMaster: true }).text('.lock')).toBe(
      'lockMuda a ficha de Tor: depois da trava, só você muda o bônus. Tor o lê na ficha.',
    );
  });
});
