import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { MapPointKind, TrapState } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { MapState } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
import { FakeMapsClient, mapMessage, mapPoint, mapResponse, mapToken } from '../../../core/maps/maps-testing';
import { PlayerMap } from './player-map';

describe('PlayerMap: a map with no fog', () => {
  async function render(): Promise<HTMLElement> {
    const api = new FakeMapsClient();
    const trap = mapPoint('t1', 'Fosso escondido', { kind: MapPointKind.TRAP, revealed: false, xBp: 3000, yBp: 3000, trap: { state: TrapState.TRIGGERED, areaSize: 2 } as never });
    const chest = mapPoint('c1', 'Baú de moedas', { kind: MapPointKind.TREASURE, revealed: false, treasureFoundAt: { seconds: 1n, nanos: 0 } as never, xBp: 7000, yBp: 7000 });
    const state = new MapState(async () =>
      mapResponse(mapMessage('map-1', 'A caverna', { gridColumns: 24, gridRows: 16, fogEnabled: false }), [trap, chest], [mapToken('c-1', 'Pensantus', { mine: true })]),
    );
    await state.open('map-1');
    TestBed.configureTestingModule({ providers: [provideRouter([]), { provide: MapsClient, useValue: api }] });
    const fixture = TestBed.createComponent(PlayerMap);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('state', state);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('draws a fired trap and a found treasure as the map\'s own marks (the area, the chest), not a bare name', async () => {
    const el = await render();
    expect(el.querySelector('app-map-pins .area')).not.toBeNull();
    expect(el.querySelector('app-map-pins .pin--found')).not.toBeNull();
    expect(el.querySelectorAll('app-map-marker')).toHaveLength(2);
    // The marker is only the hit area: the mark under it is drawn by the pins.
    expect(el.querySelectorAll('app-map-marker .pt__shape--pin')).toHaveLength(2);
  });

  it('names the marks in the legend, and draws the found chest solid, not "escondido"', async () => {
    const el = await render();
    expect(el.querySelector('app-map-pins-legend')?.textContent).toContain('Tesouro encontrado');
    expect(el.querySelector('app-map-pins .pin--found')?.classList).not.toContain('pin--hidden');
    expect(el.querySelector('.lbl--hidden')).toBeNull();
  });
});
