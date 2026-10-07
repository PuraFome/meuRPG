import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import { MapPointKind, MapPointSchema } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import { FoundTreasures } from './found-treasures';

const treasure = (id: string, found: boolean) =>
  create(MapPointSchema, {
    id,
    kind: MapPointKind.TREASURE,
    name: `Baú ${id}`,
    treasureValuePo: 250,
    treasureFoundAt: found ? timestampFromDate(new Date()) : undefined,
    treasureFoundBy: found
      ? [
          { characterId: 'b', characterName: 'Brisa' },
          { characterId: 't', characterName: 'Toren' },
        ]
      : [],
  });

describe('FoundTreasures', () => {
  it('lists only the treasures that were found, with both finders', () => {
    const fixture = TestBed.createComponent(FoundTreasures);
    fixture.componentRef.setInput('points', [treasure('a', true), treasure('b', false)]);
    fixture.detectChanges();
    const rows = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('.ft__row'));
    expect(rows).toHaveLength(1);
    expect(rows[0].textContent).toContain('Baú a');
    expect(rows[0].textContent).toContain('encontrado por Brisa e Toren');
  });

  it('draws nothing when nothing was found', () => {
    const fixture = TestBed.createComponent(FoundTreasures);
    fixture.componentRef.setInput('points', [treasure('b', false)]);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).querySelector('.ft')).toBeNull();
  });
});
