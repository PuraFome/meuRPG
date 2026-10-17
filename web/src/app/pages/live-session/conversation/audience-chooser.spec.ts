import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { AudienceChooser, type AudienceData } from './audience-chooser';
import type { ChooserPlayer } from './conversation-roster';

const PLAYERS: ChooserPlayer[] = [
  { userId: 'u-p', name: 'Pensantus', sub: 'Mago 3 · Gnomo' },
  { userId: 'u-b', name: 'Brisa', sub: 'Ladina 2 · Halfling' },
  { userId: 'u-t', name: 'Toren', sub: 'Guerreiro 3 · Humano' },
];

describe('AudienceChooser', () => {
  function setup(selected: string[] = []) {
    const close = vi.fn();
    const data: AudienceData = { players: PLAYERS, selected };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close, disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(AudienceChooser);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const radio = (name: string) =>
      Array.from(el.querySelectorAll('label')).find((l) => l.textContent?.includes(name))
        ?.querySelector('input') as HTMLInputElement;
    const button = (name: string) =>
      Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.trim() === name)!;
    const status = () => el.querySelector('[role="status"]')?.textContent?.trim();
    const click = (target: HTMLElement) => {
      target.click();
      fixture.detectChanges();
    };
    return { fixture, el, close, radio, button, status, click };
  }

  it('opens on "Toda a mesa" when no one was chosen, with the players unchecked and their class and race', () => {
    const { radio, el, status } = setup();
    expect(radio('Toda a mesa').checked).toBe(true);
    expect(radio('Só algumas pessoas').checked).toBe(false);
    expect(el.textContent).toContain('Mago 3 · Gnomo');
    expect(status()).toBe('Toda a mesa lê esta mensagem.');
  });

  it('opens with the current selection', () => {
    const { radio, el, status } = setup(['u-p', 'u-b']);
    expect(radio('Só algumas pessoas').checked).toBe(true);
    const boxes = Array.from(el.querySelectorAll<HTMLInputElement>('input[type="checkbox"]'));
    expect(boxes.map((b) => b.checked)).toEqual([true, true, false]);
    expect(status()).toBe('Pensantus e Brisa leem esta mensagem.');
  });

  it('says the master always reads and the others never see the line', () => {
    const { el } = setup();
    expect(el.textContent).toContain('Você sempre lê');
    expect(el.textContent).toContain('não vê a mensagem nem sabe que ela existe');
  });

  it('follows the boxes in the live line and answers with the chosen accounts in the table order', () => {
    const { radio, el, click, close, button, status } = setup();
    click(radio('Só algumas pessoas'));
    expect(status()).toBe('Ninguém foi escolhido. Marque quem lê esta mensagem.');
    const boxes = Array.from(el.querySelectorAll<HTMLInputElement>('input[type="checkbox"]'));
    click(boxes[2]);
    click(boxes[0]);
    expect(status()).toBe('Pensantus e Toren leem esta mensagem.');
    click(button('Pronto'));
    expect(close).toHaveBeenCalledWith(['u-p', 'u-t']);
  });

  it('answers with no one chosen (the whole table) when "Toda a mesa" is picked', () => {
    const { radio, click, button, close } = setup(['u-p']);
    click(radio('Toda a mesa'));
    click(button('Pronto'));
    expect(close).toHaveBeenCalledWith([]);
  });

  it('does not close on "Pronto" while "Só algumas pessoas" has no one', () => {
    const { radio, click, button, close } = setup();
    click(radio('Só algumas pessoas'));
    click(button('Pronto'));
    expect(button('Pronto').getAttribute('aria-disabled')).toBe('true');
    expect(close).not.toHaveBeenCalled();
  });

  it('"Cancelar" closes with nothing, which keeps the selection', () => {
    const { click, button, close, radio } = setup(['u-p']);
    click(radio('Toda a mesa'));
    click(button('Cancelar'));
    expect(close).toHaveBeenCalledWith(undefined);
  });

  it('keeps the checkboxes after "Toda a mesa" is picked and back', () => {
    const { radio, click, el } = setup(['u-p']);
    click(radio('Toda a mesa'));
    click(radio('Só algumas pessoas'));
    const boxes = Array.from(el.querySelectorAll<HTMLInputElement>('input[type="checkbox"]'));
    expect(boxes[0].checked).toBe(true);
  });
});
