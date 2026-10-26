import { ComponentFixture, TestBed } from '@angular/core/testing';

import type { GetMapResponse } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { MapMoveStop } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { FakeTokenMoveClient } from '../../../core/maps/token-move-testing';
import { MapState } from '../../../core/maps/map-state';
import { mapMessage, mapToken } from '../../../core/maps/maps-testing';
import { centerOf } from '../../../core/maps/token-move';
import { TokenMover } from '../../../core/maps/token-mover';
import { nextTurn } from '../../../../test-settle';
import { TokenMovePanel } from './token-move-panel';

const GRID = { columns: 24, rows: 16 };
const flat = (e: Element | null | undefined) => {
  if (!e) {
    return undefined;
  }
  const copy = e.cloneNode(true) as Element;
  copy.querySelectorAll('mat-icon').forEach((i) => i.remove());
  return copy.textContent
    ?.replace(/\u00a0/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
};

describe('TokenMovePanel ("Mover Tor", B2-01, B2-03)', () => {
  let fixture: ComponentFixture<TokenMovePanel>;
  let el: HTMLElement;
  let api: FakeTokenMoveClient;
  let mover: TokenMover;

  async function setup(grid = true) {
    api = new FakeTokenMoveClient();
    const state = new MapState(
      async () =>
        ({
          map: mapMessage('m1', 'Cripta', grid ? { gridColumns: 24, gridRows: 16 } : {}),
          points: [],
          tokens: [
            mapToken('tor', 'Tor', { mapId: 'm1', ...centerOf(5, 5, GRID) }),
            mapToken('brisa', 'Brisa', { mapId: 'm1', ...centerOf(3, 2, GRID) }),
            mapToken('pens', 'Pensantus', { mapId: 'm1', ...centerOf(2, 2, GRID) }),
          ],
        }) as unknown as GetMapResponse,
    );
    await state.open('m1');
    mover = new TokenMover(api, state, () => 'c1');
    TestBed.resetTestingModule();
    fixture = TestBed.createComponent(TokenMovePanel);
    fixture.componentRef.setInput('mover', mover);
    fixture.detectChanges();
    el = fixture.nativeElement;
  }

  async function settle() {
    for (let i = 0; i < 3; i++) {
      await fixture.whenStable();
      await nextTurn();
      fixture.detectChanges();
    }
  }
  const button = (label: string) =>
    el.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`)!;
  const go = () => el.querySelector<HTMLButtonElement>('.go')!;

  it('draws nothing while no token is chosen', async () => {
    await setup();
    expect(el.querySelector('.mp')).toBeNull();
  });

  it('says "Mover Tor" with the pad, the steps and a button that waits for a destination', async () => {
    await setup();
    mover.pick('tor');
    fixture.detectChanges();
    expect(flat(el.querySelector('h3'))).toBe('Mover Tor');
    expect(el.querySelectorAll('.pad__btn')).toHaveLength(8 + 2);
    expect(flat(go())).toBe('Mover Tor');
    expect(go().getAttribute('aria-disabled')).toBe('true');
    expect(flat(el.querySelector('#mp-why'))).toBe('Escolha o quadrado de destino.');
    expect(flat(el.querySelector('.steps__n'))).toBe('1');
    expect(flat(el)).toContain('de 1 a 20 · cada quadrado tem 1,5 m');
  });

  it('a direction and the steps set the destination, and the summary says it in words and meters', async () => {
    await setup();
    mover.pick('tor');
    fixture.detectChanges();
    button('Ir para a direita').click();
    fixture.detectChanges();
    button('Mais um quadrado').click();
    fixture.detectChanges();
    button('Mais um quadrado').click();
    fixture.detectChanges();
    expect(button('Ir para a direita').getAttribute('aria-pressed')).toBe('true');
    expect(flat(el.querySelector('.box__title'))).toBe('Tor vai 3 quadrados para a direita');
    expect(flat(el.querySelector('.box__detail'))).toBe('Distância: 4,5 m');
    expect(go().getAttribute('aria-disabled')).toBeNull();
    button('Menos um quadrado').click();
    fixture.detectChanges();
    expect(mover.steps()).toBe(2);
    expect(flat(el.querySelector('.steps__n'))).toBe('2');
  });

  it('moves, and says it stopped before the marked square, with the amber notice and the line asked', async () => {
    await setup();
    mover.pick('tor');
    fixture.detectChanges();
    button('Ir para a direita').click();
    mover.setSteps(10);
    api.moveOutcome = {
      moved: true,
      blocked: '',
      results: [
        {
          characterId: 'tor',
          name: 'Tor',
          ...centerOf(10, 5, GRID),
          asked: 10,
          moved: 5,
          stopped: MapMoveStop.NO_PASS,
        },
      ],
    };
    fixture.detectChanges();
    go().click();
    await settle();
    expect(api.calls.at(-1)).toContain('moveTokens m1 tor');
    const notice = el.querySelector('.mr-notice--warning')!;
    expect(flat(notice)).toBe(
      'O movimento parou antes. O próximo quadrado está marcado como “Não dá para passar”. Tor andou 7,5 m.',
    );
    expect(flat(el.querySelector('.box__detail'))).toBe(
      'Pedido: 10 quadrados (15 m) · andou 5 (7,5 m)',
    );
  });

  it('shows "Movendo Tor…" and a dashed button while the server has not answered', async () => {
    await setup();
    mover.pick('tor');
    mover.tap(centerOf(9, 5, GRID));
    fixture.detectChanges();
    let release: () => void = () => undefined;
    api.moveTokens = () =>
      new Promise(
        (resolve) => (release = () => resolve({ moved: true, blocked: '', results: [] })),
      );
    fixture.detectChanges();
    go().click();
    fixture.detectChanges();
    expect(flat(el.querySelector('.busy'))).toBe('Movendo Tor…');
    expect(go().getAttribute('aria-disabled')).toBe('true');
    release();
    await settle();
    expect(el.querySelector('.busy')).toBeNull();
  });

  it('a group: "3 selecionados", "Mover os 3 juntos", and "Limpar seleção"', async () => {
    await setup();
    mover.pick('tor');
    mover.pick('brisa', true);
    mover.pick('pens', true);
    mover.tap(centerOf(8, 5, GRID));
    fixture.detectChanges();
    expect(flat(el.querySelector('h3'))).toBe('3 selecionados');
    expect(el.querySelector('.pad')).toBeNull();
    expect(flat(el.querySelector('.mp__lead'))).toContain('os outros vão junto, na mesma formação');
    expect(flat(el.querySelector('.box__title'))).toBe('Destino: 3 quadrados para a direita');
    expect(flat(go())).toBe('Mover os 3 juntos');
    el.querySelector<HTMLButtonElement>('.clear')!.click();
    fixture.detectChanges();
    expect(el.querySelector('.mp')).toBeNull();
    expect(mover.selected()).toEqual([]);
  });

  it('writes a refusal in red and says the token went back', async () => {
    await setup();
    mover.pick('tor');
    mover.tap(centerOf(9, 5, GRID));
    fixture.detectChanges();
    api.failWith = new (await import('@connectrpc/connect')).ConnectError('x', 5);
    fixture.detectChanges();
    go().click();
    await settle();
    expect(flat(el.querySelector('.mr-notice--danger'))).toBe(
      'Não deu para mover Tor. Ele não está mais neste mapa. O token voltou para onde estava.',
    );
  });

  it('on a map without a grid it says so and offers no pad', async () => {
    await setup(false);
    mover.pick('tor');
    fixture.detectChanges();
    expect(flat(el.querySelector('.mp__lead'))).toContain('Este mapa não tem grade');
    expect(el.querySelector('.pad')).toBeNull();
    expect(el.querySelector('.go')).toBeNull();
  });
});
