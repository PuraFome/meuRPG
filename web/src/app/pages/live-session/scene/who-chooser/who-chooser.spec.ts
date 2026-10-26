import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import type { ChoosablePlayer } from '../../../../core/play/scene-audience';
import { imageChooserLine, sceneChooserLine } from '../../../../core/play/scene-audience';
import { WhoChooser, type WhoData } from './who-chooser';

const PLAYERS: ChoosablePlayer[] = [
  { userId: 'u-p', name: 'Pensantus', sub: 'na cena' },
  { userId: 'u-b', name: 'Brisa', sub: 'na cena' },
  { userId: 'u-t', name: 'Toren', sub: 'fora da cena' },
];

describe('WhoChooser ("Quem está na cena?", "Quem vê “<imagem>”?")', () => {
  function setup(selected: string[], line: WhoData['line'] = imageChooserLine) {
    const close = vi.fn();
    const data: WhoData = {
      title: 'Quem vê “Vista do salão”?',
      subtitle: 'Você sempre vê. Quem não for escolhido não vê a imagem.',
      players: PLAYERS,
      selected,
      line,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close, disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(WhoChooser);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const boxes = () => Array.from(el.querySelectorAll<HTMLInputElement>('input[type="checkbox"]'));
    const button = (name: string) =>
      Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes(name))!;
    const status = () => {
      const copy = el.querySelector('[role="status"]')!.cloneNode(true) as Element;
      copy.querySelectorAll('mat-icon').forEach((i) => i.remove());
      return copy.textContent?.trim();
    };
    const click = (target: HTMLElement) => {
      target.click();
      fixture.detectChanges();
    };
    return { el, close, boxes, button, status, click };
  }

  it('opens with "Todos" first, then each player with where the character is', () => {
    const { el, boxes, status } = setup(['u-p', 'u-b']);
    expect(el.querySelector('h2')?.textContent).toContain('Quem vê “Vista do salão”?');
    expect(boxes().map((b) => b.checked)).toEqual([false, true, true, false]);
    expect(boxes()[0].indeterminate).toBe(true);
    expect(el.textContent).toContain('fora da cena');
    expect(status()).toBe('Pensantus e Brisa veem a imagem.');
  });

  it('"Todos" marks and clears everyone, and the line follows', () => {
    const { boxes, click, status } = setup([]);
    click(boxes()[0]);
    expect(boxes().every((b) => b.checked)).toBe(true);
    expect(status()).toBe('Toda a mesa vê a imagem.');
    click(boxes()[0]);
    expect(boxes().some((b) => b.checked)).toBe(false);
    expect(status()).toBe('Ninguém foi escolhido. Marque quem vê a imagem.');
  });

  it("answers the accounts chosen, in the table's order, and the whole table when all are", () => {
    const { boxes, button, click, close } = setup(['u-t']);
    click(boxes()[2]);
    click(button('Pronto'));
    expect(close).toHaveBeenLastCalledWith({ everyone: false, userIds: ['u-b', 'u-t'] });
    click(boxes()[0]);
    click(boxes()[0]);
    click(boxes()[0]);
    click(button('Pronto'));
    expect(close).toHaveBeenLastCalledWith({
      everyone: true,
      userIds: ['u-p', 'u-b', 'u-t'],
    });
  });

  it('keeps "Pronto" dashed and answers nothing while nobody is chosen; "Cancelar" answers nothing', () => {
    const { button, click, close } = setup([]);
    expect(button('Pronto').getAttribute('aria-disabled')).toBe('true');
    click(button('Pronto'));
    expect(close).not.toHaveBeenCalled();
    click(button('Cancelar'));
    expect(close).toHaveBeenCalledWith(undefined);
  });

  it("says the scene's own live line when it is the scene that is chosen", () => {
    const { status } = setup(['u-p'], sceneChooserLine);
    expect(status()).toBe('Pensantus está na cena.');
  });
});
