import { create } from '@bufbuild/protobuf';

import {
  DraggedSquareSchema,
  GetMoveOptionsResponseSchema,
  ReachableSquareSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { combatant } from './combat-testing';
import {
  compass,
  dragLegend,
  dragMovementLine,
  dragSentence,
  dragged,
  draggedTo,
  squaresOf,
} from './drag-plan';

const plain = (s: string) => s.replace(/\u00a0/g, ' ');
const hob = combatant({ id: 'h', label: 'Hobgoblin' });

describe('moving with a grappled creature (W7-X)', () => {
  it('finds who the mover drags from the options', () => {
    const options = create(GetMoveOptionsResponseSchema, { draggingCombatantId: 'h' });
    expect(dragged(options, [hob])?.label).toBe('Hobgoblin');
    expect(dragged(create(GetMoveOptionsResponseSchema), [hob])).toBeNull();
    expect(dragged(null, [hob])).toBeNull();
  });

  it('says the speed, halved by the server, and who is dragged', () => {
    expect(plain(dragMovementLine(150, true, hob))).toBe(
      'Deslocamento: 4,5 m (metade, arrastando o Hobgoblin)',
    );
    expect(plain(dragMovementLine(300, false, hob))).toBe(
      'Deslocamento: 9,0 m (arrastando o Hobgoblin)',
    );
    expect(dragLegend(hob)).toBe('Onde o Hobgoblin termina');
  });

  it('says the path in squares and the compass, and where the creature stops', () => {
    expect(squaresOf({ col: 5, row: 4 }, { col: 3, row: 4 })).toBe(2);
    expect(compass({ col: 5, row: 4 }, { col: 3, row: 4 })).toBe('oeste');
    expect(compass({ col: 5, row: 4 }, { col: 7, row: 4 })).toBe('leste');
    expect(compass({ col: 5, row: 4 }, { col: 5, row: 2 })).toBe('norte');
    expect(compass({ col: 5, row: 4 }, { col: 5, row: 6 })).toBe('sul');
    expect(compass({ col: 5, row: 4 }, { col: 3, row: 2 })).toBe('noroeste');
    expect(compass({ col: 5, row: 4 }, { col: 7, row: 2 })).toBe('nordeste');
    expect(compass({ col: 5, row: 4 }, { col: 3, row: 6 })).toBe('sudoeste');
    expect(compass({ col: 5, row: 4 }, { col: 7, row: 6 })).toBe('sudeste');
    expect(plain(dragSentence({ col: 5, row: 4 }, { col: 3, row: 4 }, hob, 150))).toBe(
      'Você anda 2 casas para o oeste; o Hobgoblin, que você agarra, vem atrás e para na casa que você deixou. Cabem até 4,5 m neste turno.',
    );
    expect(dragSentence({ col: 5, row: 4 }, { col: 4, row: 4 }, hob, 150)).toContain(
      'Você anda 1 casa para',
    );
  });

  it('reads the square the server chose for the dragged creature, never one of its own', () => {
    const square = create(ReachableSquareSchema, {
      col: 3,
      row: 4,
      draggedTo: create(DraggedSquareSchema, { col: 4, row: 4 }),
    });
    expect(draggedTo(square)).toEqual({ col: 4, row: 4 });
    expect(draggedTo(create(ReachableSquareSchema, { col: 3, row: 4 }))).toBeNull();
    expect(draggedTo(undefined)).toBeNull();
  });
});
