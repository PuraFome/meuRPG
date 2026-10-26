import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { MapFormation } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { mapToken } from '../../../core/maps/maps-testing';
import { nextTurn } from '../../../../test-settle';
import { PutPartyDialog, type PutPartyData } from './put-party-dialog';

const flat = (e: Element | null | undefined) =>
  e?.textContent?.replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

describe('PutPartyDialog ("Pôr o grupo no mapa", B2-05)', () => {
  function setup(put = vi.fn(async () => ({ movedForRoom: 0 }))) {
    const close = vi.fn();
    const data: PutPartyData = {
      mapName: 'A caverna do Vale Seco',
      members: [
        { id: 'p', name: 'Pensantus', sub: 'Mago 5' },
        { id: 'b', name: 'Brisa', sub: 'Bardo 5' },
        { id: 't', name: 'Tor', sub: 'Guerreiro 5' },
        { id: 'j', name: 'Jabuti', sub: 'Pequenino · nível 5' },
      ],
      image: { url: '/images/m1', width: 800, height: 600 },
      layers: null,
      grid: { columns: 20, rows: 10 },
      tokens: [mapToken('p', 'Pensantus', { xBp: 1000, yBp: 1000 })],
      start: { xBp: 5000, yBp: 5000 },
      put,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close, disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(PutPartyDialog);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const click = (target: Element) => {
      (target as HTMLElement).click();
      fixture.detectChanges();
    };
    const button = (text: string) =>
      Array.from(el.querySelectorAll('button')).find((b) => flat(b)?.includes(text))!;
    return { fixture, el, close, put, click, button };
  }

  it('lists who comes, all chosen at first, with "N de M", the class small under the name, and "Pôr N no mapa"', () => {
    const { el, button } = setup();
    expect(flat(el.querySelector('h2'))).toBe('Pôr o grupo no mapa');
    expect(flat(el.querySelector('.frame__sub'))).toBe('A caverna do Vale Seco · fora de combate');
    expect(flat(el.querySelector('.who__count'))).toBe('4 de 4');
    expect(el.textContent).toContain('Guerreiro 5');
    expect(flat(button('Pôr 4 no mapa'))).toBe('Pôr 4 no mapa');
  });

  it('taking someone out says "fora desta vez" and changes the button and the rectangle', () => {
    const { el, click, button } = setup();
    click(el.querySelectorAll<HTMLInputElement>('.who__row input')[3]);
    expect(flat(el.querySelector('.who__count'))).toBe('3 de 4');
    expect(el.textContent).toContain('fora desta vez');
    expect(flat(button('Pôr 3 no mapa'))).toBe('Pôr 3 no mapa');
  });

  it('"Em bloco" is chosen first with its hint; "Em arco" has no rectangle, only the square', () => {
    const { el, click, button } = setup();
    expect(button('Em bloco').getAttribute('aria-checked')).toBe('true');
    expect(flat(el.querySelector('.how__hint'))).toBe('Juntos, em até 2 fileiras.');
    expect(el.querySelector<HTMLElement>('.rect')!.style.width).toBe('10%');
    click(button('Em fila'));
    expect(flat(el.querySelector('.how__hint'))).toBe('Lado a lado, à direita do primeiro.');
    expect(el.querySelector<HTMLElement>('.rect')!.style.width).toBe('20%');
    click(button('Em arco'));
    expect(el.querySelector<HTMLElement>('.rect')!.style.width).toBe('5%');
  });

  it('a tap on the picture moves the corner, and so do the arrows', () => {
    const { el, fixture } = setup();
    const pick = el.querySelector<HTMLElement>('.pick')!;
    pick.getBoundingClientRect = () => ({ left: 0, top: 0, width: 200, height: 100 }) as DOMRect;
    pick.dispatchEvent(new MouseEvent('click', { clientX: 100, clientY: 50, bubbles: true }));
    fixture.detectChanges();
    expect(flat(el.querySelector('[role="status"]'))).toBe(
      'Canto de cima à esquerda: coluna 11, linha 6.',
    );
    pick.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }));
    fixture.detectChanges();
    expect(flat(el.querySelector('[role="status"]'))).toBe(
      'Canto de cima à esquerda: coluna 10, linha 6.',
    );
  });

  it('sends the ones chosen, the formation and the corner, then closes', async () => {
    const { fixture, click, button, put, close } = setup();
    click(button('Em fila'));
    click(button('Pôr 4 no mapa'));
    await fixture.whenStable();
    expect(put).toHaveBeenCalledWith(
      ['p', 'b', 't', 'j'],
      MapFormation.LINE,
      expect.objectContaining({ xBp: expect.any(Number) }),
    );
    expect(close).toHaveBeenCalledWith(true);
  });

  it('writes a refusal in the dialog and stays open; with nobody chosen the button is dashed and does nothing', async () => {
    const put = vi.fn(async () => {
      throw new (await import('@connectrpc/connect')).ConnectError('x', 5);
    });
    const { fixture, el, click, button, close } = setup(put);
    click(button('Pôr 4 no mapa'));
    for (let i = 0; i < 3; i++) {
      await fixture.whenStable();
      await nextTurn();
      fixture.detectChanges();
    }
    expect(close).not.toHaveBeenCalled();
    expect(el.querySelector('[role="alert"]')).not.toBeNull();
    for (const box of Array.from(el.querySelectorAll<HTMLInputElement>('.who__row input'))) {
      click(box);
    }
    expect(button('Pôr 0 no mapa').getAttribute('aria-disabled')).toBe('true');
    click(button('Pôr 0 no mapa'));
    expect(put).toHaveBeenCalledTimes(1);
  });
});
