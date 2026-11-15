import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { create } from '@bufbuild/protobuf';

import {
  ItemEmptyRule,
  ItemLoss,
  ItemRechargeKind,
  ItemRechargeSchema,
} from '../../../../gen/meurpg/characters/v1/characters_pb';
import { type LastChargeData, LastChargeSheet } from './last-charge-sheet';

const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim();

describe('LastChargeSheet', () => {
  const roll = vi.fn();
  const close = vi.fn();

  beforeEach(() => {
    roll.mockReset().mockResolvedValue({ kind: 'done', d20: 14 });
    close.mockReset();
  });

  function render(over: Partial<LastChargeData> = {}) {
    const data: LastChargeData = {
      itemName: 'Varinha de teia',
      max: 7,
      canApp: true,
      canType: true,
      preferApp: true,
      roll,
      ...over,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close, disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(LastChargeSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const button = (name: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        (b.getAttribute('aria-label') ?? flat(b) ?? '').includes(name),
      )!;
    const field = () => el.querySelector<HTMLInputElement>('input');
    const type = (text: string) => {
      const f = field()!;
      f.value = text;
      f.dispatchEvent(new Event('input'));
      fixture.detectChanges();
    };
    const settle = async () => {
      await fixture.whenStable();
      fixture.detectChanges();
    };
    return { el, button, field, type, settle, fixture };
  }

  it('is a dialog titled "A última carga se foi", with the item and its count', () => {
    const { el } = render();
    expect(flat(el.querySelector('h2'))).toBe('A última carga se foi');
    expect(flat(el)).toContain('Varinha de teia · 0 de 7');
    expect(flat(el)).toContain('O item pede um d20 agora.');
  });

  it('leads with "Rolar no app" when the player rolls in the app, and types on request', () => {
    const { el, button, fixture } = render();
    expect(el.querySelector('input')).toBeNull();
    expect(button('Rolar no app')).toBeTruthy();
    button('Digitar o resultado').click();
    fixture.detectChanges();
    expect(flat(el.querySelector('label'))).toBe('Face do d20');
  });

  it('rolls in the app under one key and closes with true', async () => {
    const { button, settle } = render();
    button('Rolar no app').click();
    await settle();
    expect(roll).toHaveBeenCalledWith({ inApp: true }, expect.any(String));
    expect(close).toHaveBeenCalledWith(true);
  });

  it('at a physical-dice table leads with "Face do d20" and keeps "Rolar no app" beside it', async () => {
    const { el, button, type, settle } = render({ preferApp: false });
    expect(flat(el.querySelector('label'))).toBe('Face do d20');
    expect(flat(el)).toContain('De 1 a 20');
    expect(button('Rolar no app')).toBeTruthy();
    type('14');
    button('Confirmar').click();
    await settle();
    expect(roll).toHaveBeenCalledWith({ face: 14 }, expect.any(String));
    expect(close).toHaveBeenCalledWith(true);
  });

  it('keeps "Confirmar" off until the face is a number from 1 to 20', () => {
    const { el, button, type } = render({ preferApp: false });
    expect(button('Confirmar').getAttribute('aria-disabled')).toBe('true');
    type('21');
    expect(button('Confirmar').getAttribute('aria-disabled')).toBe('true');
    expect(flat(el.querySelector('[role=alert]'))).toContain('Digite um número de 1 a 20');
    type('20');
    expect(button('Confirmar').getAttribute('aria-disabled')).toBeNull();
  });

  it('forced physical dice: no "Rolar no app" at all', () => {
    const { el, button } = render({ canApp: false, preferApp: false });
    expect(flat(el.querySelector('label'))).toBe('Face do d20');
    expect(
      Array.from(el.querySelectorAll('button')).some((b) => flat(b)?.includes('Rolar no app')),
    ).toBe(false);
    expect(button('Confirmar')).toBeTruthy();
  });

  it('forced app dice: only "Rolar no app", never a field', () => {
    const { el } = render({ canType: false });
    expect(el.querySelector('input')).toBeNull();
    expect(flat(el)).not.toContain('Digitar o resultado');
  });

  it('asks for the face of the regain die after a typed 20 the server could not count, and sends both', async () => {
    roll.mockResolvedValueOnce({ kind: 'regain', faces: 6 });
    const { el, button, type, settle } = render({ preferApp: false });
    type('20');
    button('Confirmar').click();
    await settle();
    expect(flat(el)).toContain('Face do d6');
    expect(close).not.toHaveBeenCalled();
    type('4');
    button('Confirmar').click();
    await settle();
    expect(roll).toHaveBeenLastCalledWith({ face: 20, regainFace: 4 }, expect.any(String));
    expect(close).toHaveBeenCalledWith(true);
  });

  it('shows a refusal and keeps the same key for the same roll', async () => {
    roll.mockResolvedValueOnce({ kind: 'error', text: 'Não foi possível gastar a carga.' });
    const { el, button, type, settle } = render({ preferApp: false });
    type('5');
    button('Confirmar').click();
    await settle();
    expect(flat(el.querySelector('[role=alert]'))).toContain('Não foi possível gastar a carga.');
    button('Confirmar').click();
    await settle();
    expect(roll.mock.calls[1][1]).toBe(roll.mock.calls[0][1]);
  });

  it('closing leaves the roll for later and says so', () => {
    const { el, button } = render();
    expect(flat(el)).toContain('Fechar não pula o dado');
    button('Fechar').click();
    expect(roll).not.toHaveBeenCalled();
    expect(close).toHaveBeenCalled();
    expect(close).not.toHaveBeenCalledWith(true);
  });

  it('closes on Esc too, with no roll: the dialog answers Esc itself, since the backdrop stays blocked', () => {
    const { field, el } = render({ preferApp: false });
    field()!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(roll).not.toHaveBeenCalled();
    expect(close).toHaveBeenCalledTimes(1);
    expect(close).not.toHaveBeenCalledWith(true);
    expect(el.querySelector('button[aria-label="Fechar"]')).not.toBeNull();
  });

  it('does not close on Esc while a roll is in the air', async () => {
    let finish!: (out: unknown) => void;
    roll.mockReturnValue(new Promise((resolve) => (finish = resolve)));
    const { button, el, settle } = render();
    button('Rolar no app').click();
    el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(close).not.toHaveBeenCalled();
    finish({ kind: 'done', d20: 3 });
    await settle();
    expect(close).toHaveBeenCalledWith(true);
  });

  it('says what the d20 decides for this item, as the board has it', () => {
    const { el } = render({
      recharge: create(ItemRechargeSchema, {
        kind: ItemRechargeKind.DAWN,
        empty: ItemEmptyRule.D20,
        onLoss: ItemLoss.GONE,
      }),
    });
    expect(flat(el.querySelector('.body'))).toBe(
      'O item pede um d20 agora. Com 1, a varinha vira cinzas e fica na ficha como destruída. Com 2 ou mais, ela espera o amanhecer.',
    );
    expect(flat(el.querySelector('.body strong'))).toBe('Com 1, a varinha vira cinzas');
  });
});
