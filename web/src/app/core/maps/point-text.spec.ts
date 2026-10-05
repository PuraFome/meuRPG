import { describe, expect, it } from 'vitest';

import { MapPointKind, TrapState } from '../../../gen/meurpg/maps/v1/maps_pb';
import { pointHidden, pointAriaLabel } from '../../shared/map-view/map-labels';
import { mapPoint } from './maps-testing';
import { pointTags } from './point-text';

const found = { seconds: 1n, nanos: 0 } as never;

describe('pointHidden: what the map draws as hidden', () => {
  it('a plain point is hidden until it is revealed', () => {
    expect(pointHidden(mapPoint('a', 'A', { kind: MapPointKind.SCENE, revealed: false }))).toBe(true);
    expect(pointHidden(mapPoint('a', 'A', { kind: MapPointKind.SCENE, revealed: true }))).toBe(false);
  });

  it('a found treasure is known to everyone, whatever "revealed" says; a treasure not found is hidden', () => {
    expect(pointHidden(mapPoint('c', 'C', { kind: MapPointKind.TREASURE, revealed: false, treasureFoundAt: found }))).toBe(false);
    expect(pointHidden(mapPoint('c', 'C', { kind: MapPointKind.TREASURE, revealed: false }))).toBe(true);
  });

  it('a trap is known once revealed to a character or fired', () => {
    expect(pointHidden(mapPoint('t', 'T', { kind: MapPointKind.TRAP, revealed: false, trap: { state: TrapState.ARMED } as never }))).toBe(true);
    expect(pointHidden(mapPoint('t', 'T', { kind: MapPointKind.TRAP, revealed: false, trap: { state: TrapState.TRIGGERED } as never }))).toBe(false);
    expect(pointHidden(mapPoint('t', 'T', { kind: MapPointKind.TRAP, revealed: false, trapRevealedTo: [{}] as never }))).toBe(false);
  });

  it('a light is never "escondida": it is the master\'s alone', () => {
    expect(pointHidden(mapPoint('l', 'L', { kind: MapPointKind.LIGHT, revealed: false }))).toBe(false);
    expect(pointAriaLabel(mapPoint('l', 'Tocha', { kind: MapPointKind.LIGHT }))).toBe('Tocha, Luz, só você vê');
  });
});

describe('pointTags', () => {
  it('a trap says its state and who sees it; a light says "Só você vê" and nothing hidden', () => {
    const trap = pointTags(mapPoint('t', 'T', { kind: MapPointKind.TRAP, revealed: false, trap: { state: TrapState.ARMED } as never }));
    expect(trap.map((t) => t.text)).toEqual(['Armada', 'Escondido']);
    const light = pointTags(mapPoint('l', 'L', { kind: MapPointKind.LIGHT }));
    expect(light.map((t) => t.text)).toEqual(['Só você vê']);
  });

  it('a trap the master revealed says "Revelado"', () => {
    const trap = pointTags(mapPoint('t', 'T', { kind: MapPointKind.TRAP, revealed: true, trap: { state: TrapState.ARMED } as never }));
    expect(trap.map((t) => t.text)).toEqual(['Armada', 'Revelado']);
  });

  it('a treasure says found or not with one icon for "Não encontrado"', () => {
    expect(pointTags(mapPoint('c', 'C', { kind: MapPointKind.TREASURE }))[0]).toEqual({ icon: 'close', text: 'Não encontrado' });
    expect(pointTags(mapPoint('c', 'C', { kind: MapPointKind.TREASURE, treasureFoundAt: found })).map((t) => t.text)).toEqual(['Encontrado']);
  });
});
