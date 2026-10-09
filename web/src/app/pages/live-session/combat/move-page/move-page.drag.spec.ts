import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import {
  CombatantKind,
  DraggedSquareSchema,
  GetMoveOptionsResponseSchema,
  MoveRefusal,
  ReachableSquareSchema,
  RefusedSquareSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { textOf } from '../../../../core/combat/contest-testing';
import { MovePage } from './move-page';

// Toren at (8, 7) holds the Hobgoblin at (9, 7): with 9,0 m the server halves it to 4,5 m.
const toren = combatant({
  id: 'toren',
  label: 'Toren',
  kind: CombatantKind.PLAYER,
  mine: true,
  col: 8,
  row: 7,
  speedFt: 30,
  speedDft: 300,
  movementLeftFt: 15,
  movementLeftDft: 150,
});
const hobgoblin = combatant({ id: 'h', label: 'Hobgoblin', col: 9, row: 7 });

const dragging = create(GetMoveOptionsResponseSchema, {
  movementLeftDft: 150,
  draggingCombatantId: 'h',
  draggingHalved: true,
  reachable: [
    create(ReachableSquareSchema, {
      col: 6,
      row: 7,
      costDft: 100,
      draggedTo: create(DraggedSquareSchema, { col: 7, row: 7 }),
    }),
    create(ReachableSquareSchema, {
      col: 8,
      row: 5,
      costDft: 100,
      draggedTo: create(DraggedSquareSchema, { col: 8, row: 6 }),
    }),
  ],
  refused: [create(RefusedSquareSchema, { col: 8, row: 9, reason: MoveRefusal.NO_ROOM_TO_DRAG })],
});

function setup(options = dragging) {
  const fixture = TestBed.createComponent(MovePage);
  const ref = fixture.componentRef;
  ref.setInput(
    'encounter',
    encounter({
      combatants: [toren, hobgoblin],
      currentCombatantId: 'toren',
      turnGroupIds: ['toren'],
    }),
  );
  ref.setInput('image', { url: '/images/x', width: 2000, height: 1400 });
  ref.setInput('mapName', 'A caverna do Vale Seco');
  ref.setInput('sessionNumber', 6);
  ref.setInput('options', options);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const choose = (col: number, row: number) => {
    const surface = el.querySelector<HTMLElement>('.cm__surface')!;
    surface.getBoundingClientRect = () =>
      ({ left: 0, top: 0, width: 200, height: 140, right: 200, bottom: 140, x: 0, y: 0 }) as DOMRect;
    surface.dispatchEvent(
      new MouseEvent('click', { clientX: col * 10 + 5, clientY: row * 10 + 5, bubbles: true }),
    );
    fixture.detectChanges();
  };
  return { fixture, el, choose };
}

describe('MovePage: moving with a grappled creature (board W7-Xb 6)', () => {
  it('says the speed is half, because of whom is dragged, before a square is chosen', () => {
    const { el } = setup();
    expect(textOf(el.querySelector('[data-testid="drag-line"]')!)).toBe(
      'Deslocamento: 4,5 m (metade, arrastando o Hobgoblin)',
    );
    expect(textOf(el.querySelector('.move__lead')!)).toContain('Restam 4,5 m de 9,0 m');
  });

  it('names the dashed token in the legend', () => {
    const { el } = setup();
    const legend = Array.from(el.querySelectorAll('.mr-legend li'), (li) => textOf(li));
    expect(legend).toContain('Onde o Hobgoblin termina');
    expect(el.querySelector('.mr-swatch--dragged')).not.toBeNull();
  });

  it('draws the creature on the square the server chose, and says the path (board 6)', () => {
    const { el, choose } = setup();
    expect(el.querySelector('.cm__dragged')).toBeNull();
    choose(6, 7);
    const mark = el.querySelector<HTMLElement>('.cm__dragged')!;
    expect(mark.textContent?.trim()).toBe('H');
    expect(mark.style.left).toBe(`${(7 / 20) * 100}%`);
    expect(mark.style.top).toBe(`${(7 / 14) * 100}%`);
    expect(textOf(el.querySelector('[data-testid="drag-text"]')!)).toBe(
      'Você anda 2 casas para o oeste; o Hobgoblin, que você agarra, vem atrás e para na casa que você deixou. Cabem até 4,5 m neste turno.',
    );
  });

  it('follows the chosen square: another square, another end', () => {
    const { el, choose } = setup();
    choose(6, 7);
    choose(8, 5);
    expect(el.querySelector<HTMLElement>('.cm__dragged')!.style.top).toBe(`${(6 / 14) * 100}%`);
    expect(textOf(el.querySelector('[data-testid="drag-text"]')!)).toContain('para o norte');
  });

  it('refuses a square with no room behind, saying why and drawing nothing', () => {
    const { el, choose } = setup();
    choose(8, 9);
    expect(el.querySelector('.cm__dragged')).toBeNull();
    expect(el.querySelector('[data-testid="drag-text"]')).toBeNull();
    expect(textOf(el.querySelector('.mr-notice--danger')!)).toContain('Sem casa para arrastar');
    expect(textOf(el.querySelector('.mr-notice--danger')!)).toContain(
      'Não há casa livre atrás de você para quem você segura.',
    );
  });

  it('says none of it when the mover drags nobody', () => {
    const { el, choose } = setup(
      create(GetMoveOptionsResponseSchema, {
        movementLeftDft: 150,
        reachable: [create(ReachableSquareSchema, { col: 6, row: 7, costDft: 100 })],
      }),
    );
    choose(6, 7);
    expect(el.querySelector('[data-testid="drag-line"]')).toBeNull();
    expect(el.querySelector('[data-testid="drag-text"]')).toBeNull();
    expect(el.querySelector('.cm__dragged')).toBeNull();
    expect(el.querySelector('.mr-swatch--dragged')).toBeNull();
  });
});
