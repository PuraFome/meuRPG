import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { of } from 'rxjs';

import type { GetMapResponse } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { MapFormation } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { MapState } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
import { mapMessage, mapToken } from '../../../core/maps/maps-testing';
import { TokenMoveClient } from '../../../core/maps/token-move-client';
import { FakeTokenMoveClient } from '../../../core/maps/token-move-testing';
import { centerOf } from '../../../core/maps/token-move';
import { TokenMover } from '../../../core/maps/token-mover';
import { nextTurn } from '../../../../test-settle';
import { SessionTokens } from './session-tokens';

const GRID = { columns: 24, rows: 16 };
const flat = (e: Element | null | undefined) => {
  if (!e) {
    return undefined;
  }
  const copy = e.cloneNode(true) as Element;
  copy.querySelectorAll('mat-icon').forEach((icon) => icon.remove());
  return copy.textContent?.replace(/ /g, ' ').replace(/\s+/g, ' ').trim();
};

describe('SessionTokens (the "Tokens" tab under the master\'s map, B2)', () => {
  const open = vi.fn();

  async function setup(map = { gridColumns: 24, gridRows: 16 }) {
    const api = new FakeTokenMoveClient();
    const state = new MapState(
      async () =>
        ({
          map: mapMessage('m1', 'Cripta', map),
          points: [],
          tokens: [
            mapToken('tor', 'Tor', { mapId: 'm1', ...centerOf(5, 5, GRID) }),
            mapToken('goblin', 'Goblin 1', { mapId: 'm1', kind: 2, ...centerOf(9, 9, GRID) }),
            mapToken('tor', 'Nanquim', {
              mapId: 'm1',
              creatureId: 'raven',
              ...centerOf(6, 5, GRID),
            }),
          ],
        }) as unknown as GetMapResponse,
    );
    await state.open('m1');
    const mover = new TokenMover(api, state, () => 'c1');
    open
      .mockReset()
      .mockReturnValue({ afterClosed: () => of(undefined), afterDismissed: () => of(undefined) });
    TestBed.configureTestingModule({
      providers: [
        { provide: MapsClient, useValue: {} },
        { provide: TokenMoveClient, useValue: api },
        { provide: MatDialog, useValue: { open } },
        { provide: MatBottomSheet, useValue: { open } },
      ],
    });
    const fixture = TestBed.createComponent(SessionTokens);
    fixture.componentRef.setInput('campaignId', 'c1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('mover', mover);
    fixture.componentRef.setInput('party', [
      { id: 'tor', name: 'Tor', playerName: 'Caio' },
      { id: 'brisa', name: 'Brisa', playerName: 'Ana' },
    ]);
    fixture.componentRef.setInput(
      'info',
      new Map([['tor', { classSummary: 'Guerreiro 5', playerName: 'Caio' }]]),
    );
    fixture.detectChanges();
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        await nextTurn();
        fixture.detectChanges();
      }
    };
    return { fixture, api, state, mover, settle, el: fixture.nativeElement as HTMLElement };
  }

  it('has the switch, "Pôr o grupo no mapa", "Selecionar" and the tokens with a box for each character', async () => {
    const { el } = await setup();
    const toggle = el.querySelector<HTMLButtonElement>('button[role="switch"]')!;
    expect(flat(el.querySelector('app-keep-switch'))).toContain(
      'Os jogadores movem os próprios personagens',
    );
    expect(toggle.getAttribute('aria-checked')).toBe('false');
    expect(flat(el.querySelector('app-keep-switch .hint'))).toBe(
      'Só o mestre move os tokens neste mapa.',
    );
    const names = Array.from(el.querySelectorAll('.actions button')).map((b) => flat(b));
    expect(names.some((n) => n?.includes('Pôr o grupo no mapa'))).toBe(true);
    expect(names.some((n) => n?.includes('Selecionar'))).toBe(true);
    // A box for each character, none for a creature's token.
    expect(el.querySelectorAll('input.row__pick')).toHaveLength(2);
    expect(el.querySelectorAll('.row--pick')).toHaveLength(3);
  });

  it('turns "Os jogadores movem os próprios personagens" on with the server\'s answer', async () => {
    const { el, api, state, settle } = await setup();
    el.querySelector<HTMLButtonElement>('button[role="switch"]')!.click();
    await settle();
    expect(api.calls).toContain('setPlayersMove m1 true');
    expect(state.map()?.playersMoveTokens).toBe(true);
    expect(el.querySelector('button[role="switch"]')?.getAttribute('aria-checked')).toBe('true');
    expect(flat(el.querySelector('app-keep-switch .hint'))).toContain(
      'Cada jogador arrasta só o próprio token',
    );
  });

  it('writes a refusal of the switch and leaves it as it was', async () => {
    const { el, api, settle } = await setup();
    api.failWith = new (await import('@connectrpc/connect')).ConnectError('x', 7);
    el.querySelector<HTMLButtonElement>('button[role="switch"]')!.click();
    await settle();
    expect(flat(el.querySelector('[role="alert"]'))).toBe('Só o mestre da campanha muda isso.');
    expect(el.querySelector('button[role="switch"]')?.getAttribute('aria-checked')).toBe('false');
  });

  it('a box chooses a character and "Mover Tor" appears; two choose a group; "Selecionar" is a pressed toggle', async () => {
    const { el, fixture, mover } = await setup();
    const boxes = () => Array.from(el.querySelectorAll<HTMLInputElement>('input.row__pick'));
    boxes()[0].click();
    fixture.detectChanges();
    expect(mover.selected()).toEqual(['tor']);
    expect(flat(el.querySelector('app-token-move-panel h3'))).toBe('Mover Tor');
    const select = Array.from(el.querySelectorAll<HTMLButtonElement>('.actions button')).find((b) =>
      b.textContent?.includes('Selecionar'),
    )!;
    expect(select.getAttribute('aria-pressed')).toBe('false');
    select.click();
    fixture.detectChanges();
    expect(select.getAttribute('aria-pressed')).toBe('true');
    boxes()[1].click();
    fixture.detectChanges();
    expect(mover.selected()).toEqual(['tor', 'goblin']);
    expect(flat(el.querySelector('app-token-move-panel h3'))).toBe('2 selecionados');
  });

  it('opens the dialog with the party and the map, and puts the tokens the server answers', async () => {
    const { el, api, state, mover, settle } = await setup();
    Array.from(el.querySelectorAll<HTMLButtonElement>('.actions button'))
      .find((b) => b.textContent?.includes('Pôr o grupo no mapa'))!
      .click();
    const data = open.mock.calls[0][1].data;
    expect(data.members).toEqual([
      { id: 'tor', name: 'Tor', sub: 'Guerreiro 5' },
      { id: 'brisa', name: 'Brisa', sub: '' },
    ]);
    expect(data.grid).toEqual(GRID);
    expect(data.mapName).toBe('Cripta');
    await data.put(['tor', 'brisa'], MapFormation.LINE, { xBp: 1000, yBp: 1000 });
    await settle();
    expect(api.calls).toContain('putParty m1 tor,brisa 1 1000 1000');
    expect(mover.characters().map((t) => t.characterId)).toContain('brisa');
    expect(state.tokens().find((t) => t.characterId === 'brisa')).toBeDefined();
    expect(flat(el.querySelector('.mr-notice--success'))).toBe('2 no mapa.');
  });

  it('without a grid it explains why there is no group to put and no pad to use', async () => {
    const { el } = await setup({ gridColumns: 0, gridRows: 0 });
    expect(flat(el.querySelector('#tokens-why'))).toContain('Este mapa não tem grade');
    expect(el.querySelectorAll('input.row__pick')).toHaveLength(0);
    const put = Array.from(el.querySelectorAll<HTMLButtonElement>('.actions button')).find((b) =>
      b.textContent?.includes('Pôr o grupo no mapa'),
    )!;
    expect(put.getAttribute('aria-disabled')).toBe('true');
  });
});
