import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { Subject } from 'rxjs';

import type { GetMapResponse } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { MapMoveStop } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { MapState } from '../../../core/maps/map-state';
import { mapMessage, mapToken } from '../../../core/maps/maps-testing';
import { centerOf } from '../../../core/maps/token-move';
import { FakeTokenMoveClient } from '../../../core/maps/token-move-testing';
import { TokenMover } from '../../../core/maps/token-mover';
import { PlayerMove, NOTICE_MS } from './player-move';

const GRID = { columns: 24, rows: 16 };
const flat = (e: Element | null | undefined) => {
  if (!e) {
    return undefined;
  }
  const copy = e.cloneNode(true) as Element;
  copy.querySelectorAll('mat-icon').forEach((i) => i.remove());
  return copy.textContent?.replace(/ /g, ' ').replace(/\s+/g, ' ').trim();
};

describe('PlayerMove (C9-01 to C9-03)', () => {
  const open = vi.fn();
  let phone = false;

  async function setup(playersMove = false) {
    const api = new FakeTokenMoveClient();
    const response = (on: boolean) =>
      ({
        map: mapMessage('m1', 'Estrada do Vale', {
          gridColumns: 24,
          gridRows: 16,
          playersMoveTokens: on,
        }),
        points: [],
        tokens: [
          mapToken('brisa', 'Brisa', { mapId: 'm1', mine: true, ...centerOf(5, 5, GRID) }),
          mapToken('pens', 'Pensantus', { mapId: 'm1', ...centerOf(4, 4, GRID) }),
        ],
      }) as unknown as GetMapResponse;
    let on = playersMove;
    const state = new MapState(async () => response(on));
    await state.open('m1');
    const mover = new TokenMover(
      api,
      state,
      () => 'c1',
      () => false,
    );
    const closed = new Subject<undefined>();
    open.mockReset().mockReturnValue({ afterClosed: () => closed, afterDismissed: () => closed });
    vi.stubGlobal('matchMedia', (q: string) => ({
      matches: phone && q.includes('max-width'),
      media: q,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      addListener: () => undefined,
      removeListener: () => undefined,
    }));
    TestBed.configureTestingModule({
      providers: [
        { provide: MatDialog, useValue: { open } },
        { provide: MatBottomSheet, useValue: { open } },
      ],
    });
    const fixture = TestBed.createComponent(PlayerMove);
    fixture.componentRef.setInput('mover', mover);
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('characterId', 'brisa');
    fixture.componentRef.setInput('name', 'Brisa');
    fixture.componentRef.setInput('drag', true);
    fixture.detectChanges();
    const master = async (next: boolean) => {
      on = next;
      await state.refresh();
      fixture.detectChanges();
    };
    return { fixture, api, state, mover, master, closed, el: fixture.nativeElement as HTMLElement };
  }

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    phone = false;
  });

  it('with the switch off it says only that the master moves the tokens, and has no control (C9-01)', async () => {
    const { el } = await setup(false);
    expect(flat(el.querySelector('.pm__line'))).toBe('O mestre move os tokens agora.');
    expect(el.querySelector('button')).toBeNull();
  });

  it('with the switch on it says how to move and offers "Mover Brisa" for her own character only (C9-02)', async () => {
    const { el } = await setup(true);
    expect(flat(el.querySelector('.pm__line'))).toBe(
      'Arraste Brisa no mapa ou use “Mover Brisa”. Os outros ficam com o mestre.',
    );
    const buttons = el.querySelectorAll('button');
    expect(buttons).toHaveLength(1);
    expect(flat(buttons[0])).toBe('Mover Brisa');
  });

  it('where the map cannot be dragged (a phone) the line leaves the dragging out', async () => {
    const { el, fixture } = await setup(true);
    fixture.componentRef.setInput('drag', false);
    fixture.detectChanges();
    expect(flat(el.querySelector('.pm__line'))).toBe(
      'Use “Mover Brisa”. Os outros ficam com o mestre.',
    );
  });

  it('on a computer the button opens the panel in the card, with her token chosen, and "Fechar" puts it away', async () => {
    const { el, fixture, mover } = await setup(true);
    el.querySelector<HTMLButtonElement>('.pm__go')!.click();
    fixture.detectChanges();
    expect(mover.selected()).toEqual(['brisa']);
    expect(el.querySelector('.pm__go')).toBeNull();
    expect(el.querySelector('app-token-move-panel')).not.toBeNull();
    expect(flat(el.querySelector('#mp-steps'))).toBe('Quadrados por clique');
    expect(el.querySelector('.go--outline')).not.toBeNull();
    el.querySelector<HTMLButtonElement>('.close')!.click();
    fixture.detectChanges();
    expect(mover.selected()).toEqual([]);
    expect(el.querySelector('.pm__go')).not.toBeNull();
  });

  it('moves Brisa with the plan and says "Brisa andou 2 (3 m)."', async () => {
    const { el, fixture, api, mover } = await setup(true);
    el.querySelector<HTMLButtonElement>('.pm__go')!.click();
    fixture.detectChanges();
    el.querySelector<HTMLButtonElement>('button[aria-label="Ir para a direita"]')!.click();
    mover.setSteps(2);
    fixture.detectChanges();
    expect(flat(el.querySelector('.box__title'))).toBe('Brisa vai 2 quadrados para a direita');
    api.moveOutcome = {
      moved: true,
      blocked: '',
      results: [
        {
          characterId: 'brisa',
          name: 'Brisa',
          ...centerOf(7, 5, GRID),
          asked: 2,
          moved: 2,
          stopped: MapMoveStop.UNSPECIFIED,
        },
      ],
    };
    el.querySelector<HTMLButtonElement>('.go')!.click();
    for (let i = 0; i < 3; i++) {
      await fixture.whenStable();
      fixture.detectChanges();
    }
    expect(flat(el.querySelector('.mr-notice--success'))).toBe('Brisa andou 2 (3 m).');
  });

  it("on a phone the button opens the sheet instead, with the panel's own data", async () => {
    phone = true;
    const { el, fixture, mover, closed } = await setup(true);
    el.querySelector<HTMLButtonElement>('.pm__go')!.click();
    fixture.detectChanges();
    expect(open).toHaveBeenCalledTimes(1);
    expect(open.mock.calls[0][1].data.name).toBe('Brisa');
    expect(el.querySelector('app-token-move-panel')).toBeNull();
    expect(mover.selected()).toEqual(['brisa']);
    closed.next(undefined);
    expect(mover.selected()).toEqual([]);
  });

  describe('when the master frees or locks the moving while she looks (C9-03)', () => {
    it('says it in one polite line for 8 s, then goes back to the usual line', async () => {
      vi.useFakeTimers();
      const { el, fixture, master } = await setup(false);
      await master(true);
      expect(flat(el.querySelector('.pm__line--notice'))).toBe(
        'O mestre liberou: agora você move Brisa.',
      );
      expect(flat(el.querySelector('[role="status"]'))).toBe(
        'O mestre liberou: agora você move Brisa.',
      );
      expect(el.querySelector('.pm__go')).not.toBeNull();
      vi.advanceTimersByTime(NOTICE_MS - 1);
      fixture.detectChanges();
      expect(el.querySelector('.pm__line--notice')).not.toBeNull();
      vi.advanceTimersByTime(2);
      fixture.detectChanges();
      expect(el.querySelector('.pm__line--notice')).toBeNull();
      expect(flat(el.querySelector('.pm__line'))).toContain('Arraste Brisa no mapa');
    });

    it('locking says "O mestre voltou a mover os tokens.", removes the control and puts the open panel away without moving', async () => {
      vi.useFakeTimers();
      const { el, fixture, master, mover, api } = await setup(true);
      el.querySelector<HTMLButtonElement>('.pm__go')!.click();
      fixture.detectChanges();
      expect(el.querySelector('app-token-move-panel')).not.toBeNull();
      await master(false);
      expect(flat(el.querySelector('.pm__line--notice'))).toBe(
        'O mestre voltou a mover os tokens.',
      );
      expect(el.querySelector('app-token-move-panel')).toBeNull();
      expect(el.querySelector('.pm__go')).toBeNull();
      expect(mover.selected()).toEqual([]);
      expect(api.calls).toEqual([]);
      vi.advanceTimersByTime(NOTICE_MS + 1);
      fixture.detectChanges();
      expect(flat(el.querySelector('.pm__line'))).toBe('O mestre move os tokens agora.');
    });

    it('says nothing the first time the map is read', async () => {
      const { el } = await setup(true);
      expect(el.querySelector('.pm__line--notice')).toBeNull();
      expect(flat(el.querySelector('[role="status"]'))).toBe('');
    });
  });
});
