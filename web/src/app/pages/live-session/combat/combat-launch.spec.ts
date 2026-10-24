import { TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { Code, ConnectError } from '@connectrpc/connect';
import { of } from 'rxjs';

import type { GetMapResponse } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { MapState } from '../../../core/maps/map-state';
import { mapMessage, mapResponse } from '../../../core/maps/maps-testing';
import { CombatLaunch, CombatLaunchNotice } from './combat-launch';

describe('CombatLaunch: "Iniciar combate" from the session bar (MR-013)', () => {
  let opened: { data: { map: { id: string; columns: number } | null }; width: string }[];
  let answer!: (r: GetMapResponse) => void;
  let fail!: (e: unknown) => void;
  let started: string[];

  function setup(phone = false) {
    opened = [];
    started = [];
    const state = new MapState(
      () =>
        new Promise<GetMapResponse>((resolve, reject) => {
          answer = resolve;
          fail = reject;
        }),
    );
    const dialog = {
      open: (_c: unknown, config: (typeof opened)[number]) => {
        opened.push(config);
        return { afterClosed: () => of({ name: 'Emboscada' }) };
      },
    } as unknown as MatDialog;
    const launch = new CombatLaunch(
      dialog,
      () => phone,
      () => 'camp-1',
      () => state,
      (e) => started.push(e.name),
    );
    return { state, launch };
  }

  it('waits while the current map is read, so the dialog never opens with "no map" for a session that has one', async () => {
    const { state, launch } = setup();
    const read = state.open('map-1');
    expect(launch.off()).toBe(true);
    launch.open();
    expect(opened).toHaveLength(0);

    answer(mapResponse(mapMessage('map-1', 'Estrada do Vale', { gridColumns: 20, gridRows: 14 })));
    await read;
    expect(launch.off()).toBe(false);
    launch.open();
    expect(opened).toHaveLength(1);
    expect(opened[0].data.map?.id).toBe('map-1');
    expect(opened[0].data.map?.columns).toBe(20);
    expect(started).toEqual(['Emboscada']);
  });

  it('opens at once when the session has no current map (the theatre of the mind)', async () => {
    const { state, launch } = setup();
    await state.open(null);
    expect(launch.off()).toBe(false);
    launch.open();
    expect(opened).toHaveLength(1);
    expect(opened[0].data.map).toBeNull();
  });

  it('is a full-screen dialog on a phone', async () => {
    const { state, launch } = setup(true);
    await state.open(null);
    launch.open();
    expect(opened[0].width).toBe('100vw');
  });

  it('does not start a combat in the theatre of the mind when the current map could not be read', async () => {
    const { state, launch } = setup();
    const read = state.open('map-1');
    fail(new ConnectError('blip', Code.Unavailable));
    await read;
    expect(state.status()).toBe('error');
    expect(launch.unreadable()).toBe(true);
    expect(launch.off()).toBe(true);
    launch.open();
    expect(opened).toHaveLength(0);
  });

  it('starts on the map once it is read again', async () => {
    const { state, launch } = setup();
    const read = state.open('map-1');
    fail(new ConnectError('blip', Code.Unavailable));
    await read;
    const again = state.refresh();
    answer(mapResponse(mapMessage('map-1', 'Estrada do Vale', { gridColumns: 20, gridRows: 14 })));
    await again;
    expect(launch.off()).toBe(false);
    launch.open();
    expect(opened[0].data.map?.id).toBe('map-1');
  });
});

describe('CombatLaunchNotice', () => {
  it('says the map could not be read and offers "Tentar de novo"', () => {
    const fixture = TestBed.createComponent(CombatLaunchNotice);
    const retried = vi.fn();
    fixture.componentInstance.retry.subscribe(retried);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.textContent).toContain('Não deu para ler o mapa atual');
    const button = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Tentar de novo'),
    )!;
    button.click();
    expect(retried).toHaveBeenCalledTimes(1);
  });
});
