import { TestBed } from '@angular/core/testing';

import { MapPointKind } from '../../../../gen/meurpg/maps/v1/maps_pb';
import type { ViewPoint } from '../map-geometry';
import { MapMarker } from './map-marker';

function marker(point: Partial<ViewPoint> & { name: string; kind: number }): HTMLElement {
  TestBed.resetTestingModule();
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(MapMarker);
  fixture.componentRef.setInput('point', {
    id: 'p',
    xBp: 5000,
    yBp: 5000,
    revealed: true,
    ...point,
  });
  fixture.detectChanges();
  return fixture.nativeElement;
}

describe("MapMarker: a generated dungeon's stairs (MAP-LANGUAGE-E10.md)", () => {
  it('draws a point the server marks as a stair with its arrow, up or down, for the master and for a player alike', () => {
    expect(
      marker({ name: 'Escada para cima', kind: MapPointKind.SUBMAP, stairs: 1 }).querySelector(
        'mat-icon',
      )?.textContent,
    ).toBe('arrow_upward');
    expect(
      marker({ name: 'Escada para baixo', kind: MapPointKind.SUBMAP, stairs: 2 }).querySelector(
        'mat-icon',
      )?.textContent,
    ).toBe('arrow_downward');
  });

  it('keeps the stairs glyph of a submap for any point the server does not mark, whatever its name', () => {
    expect(
      marker({ name: 'Cripta', kind: MapPointKind.SUBMAP }).querySelector('mat-icon')?.textContent,
    ).toBe('stairs');
    expect(
      marker({ name: 'Escada para cima', kind: MapPointKind.SUBMAP }).querySelector('mat-icon')
        ?.textContent,
    ).toBe('stairs');
  });

  it('names a stair by its own name, with no "Submapa"', () => {
    const el = marker({
      name: 'Escada para cima',
      kind: MapPointKind.SUBMAP,
      stairs: 1,
      revealed: false,
    });
    expect(el.querySelector('button')?.getAttribute('aria-label')).toBe(
      'Escada para cima, escondido',
    );
    expect(
      marker({ name: 'Cripta', kind: MapPointKind.SUBMAP })
        .querySelector('button')
        ?.getAttribute('aria-label'),
    ).toBe('Cripta, Submapa');
  });
});
