import { MapPointKind } from '../../../gen/meurpg/maps/v1/maps_pb';
import { pointKindIcon, pointKindLabel } from './map-labels';
import { pointSub } from '../map-lists/map-points-list';
import { mapPoint } from '../../core/maps/maps-testing';

describe('a generated dungeon\'s stair in the labels', () => {
  it('is "Escada" with its arrow, never "Submapa" with the submap glyph', () => {
    expect(pointKindLabel(MapPointKind.SUBMAP, 1)).toBe('Escada');
    expect(pointKindLabel(MapPointKind.SUBMAP, 2)).toBe('Escada');
    expect(pointKindLabel(MapPointKind.SUBMAP)).toBe('Submapa');
    expect(pointKindIcon(MapPointKind.SUBMAP, 1)).toBe('arrow_upward');
    expect(pointKindIcon(MapPointKind.SUBMAP, 2)).toBe('arrow_downward');
    expect(pointKindIcon(MapPointKind.SUBMAP)).toBe('stairs');
  });

  it('is "Escada" in the points list', () => {
    expect(pointSub(mapPoint('s', 'Escada para cima', { kind: MapPointKind.SUBMAP, stairs: 1 }))).toBe('Escada');
    expect(pointSub(mapPoint('c', 'Cripta', { kind: MapPointKind.SUBMAP }))).toBe('Submapa');
  });
});
