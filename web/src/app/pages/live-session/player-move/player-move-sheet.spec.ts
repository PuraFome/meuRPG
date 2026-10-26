import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MAT_BOTTOM_SHEET_DATA, MatBottomSheetRef } from '@angular/material/bottom-sheet';

import type { GetMapResponse } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { MapState } from '../../../core/maps/map-state';
import { mapMessage, mapToken } from '../../../core/maps/maps-testing';
import { centerOf } from '../../../core/maps/token-move';
import { FakeTokenMoveClient } from '../../../core/maps/token-move-testing';
import { TokenMover } from '../../../core/maps/token-mover';
import { PlayerMoveSheet, type PlayerMoveSheetData } from './player-move-sheet';

const flat = (e: Element | null | undefined) => e?.textContent?.replace(/\s+/g, ' ').trim();

describe('PlayerMoveSheet ("Mover Brisa" on a phone, C9-02 and C9-03)', () => {
  async function setup() {
    const grid = { columns: 24, rows: 16 };
    const state = new MapState(
      async () =>
        ({
          map: mapMessage('m1', 'Estrada', {
            gridColumns: 24,
            gridRows: 16,
            playersMoveTokens: true,
          }),
          points: [],
          tokens: [
            mapToken('brisa', 'Brisa', { mapId: 'm1', mine: true, ...centerOf(5, 5, grid) }),
          ],
        }) as unknown as GetMapResponse,
    );
    await state.open('m1');
    const mover = new TokenMover(
      new FakeTokenMoveClient(),
      state,
      () => 'c1',
      () => false,
    );
    mover.pick('brisa');
    const canMove = signal(true);
    const dismiss = vi.fn();
    const data: PlayerMoveSheetData = { mover, name: 'Brisa', canMove: () => canMove() };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_BOTTOM_SHEET_DATA, useValue: data },
        { provide: MatBottomSheetRef, useValue: { dismiss, disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(PlayerMoveSheet);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, canMove, dismiss, mover };
  }

  it('is titled "Mover Brisa", holds the panel with its filled button and a "Cancelar" that closes it', async () => {
    const { el, dismiss, fixture } = await setup();
    expect(flat(el.querySelector('h2'))).toBe('Mover Brisa');
    expect(el.querySelector('.go--outline')).toBeNull();
    const cancel = Array.from(el.querySelectorAll('button')).find((b) => flat(b) === 'Cancelar')!;
    cancel.click();
    fixture.detectChanges();
    expect(dismiss).toHaveBeenCalled();
  });

  it('closes by itself when the master takes the switch back', async () => {
    const { fixture, canMove, dismiss } = await setup();
    expect(dismiss).not.toHaveBeenCalled();
    canMove.set(false);
    TestBed.tick();
    fixture.detectChanges();
    expect(dismiss).toHaveBeenCalled();
  });
});
