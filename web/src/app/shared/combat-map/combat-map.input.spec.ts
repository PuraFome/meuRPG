import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { combatant } from '../../core/combat/combat-testing';
import { CombatMap, type TokenDrop } from './combat-map';

/** The keyboard and the pointer of the battle map: what it reports to the page (`choose`, `confirm`, `tokenDrop`) and what it draws as it goes. */
describe('CombatMap, keyboard and pointer', () => {
  const tokens = [
    combatant({ id: 'a', label: 'Toren', col: 8, row: 7 }),
    combatant({ id: 'b', label: 'Lia', col: 2, row: 2 }),
  ];

  function setup(inputs: Record<string, unknown> = {}) {
    const fixture = TestBed.createComponent(CombatMap);
    const ref = fixture.componentRef;
    ref.setInput('image', { url: '/images/x', width: 2000, height: 1400 });
    ref.setInput('columns', 20);
    ref.setInput('rows', 14);
    ref.setInput('combatants', tokens);
    for (const [k, v] of Object.entries(inputs)) {
      ref.setInput(k, v);
    }
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    const surface = root.querySelector<HTMLElement>('.cm__surface')!;
    // 100 px a square: a point in the page is its square times 100.
    surface.getBoundingClientRect = () =>
      ({ left: 0, top: 0, width: 2000, height: 1400 }) as DOMRect;
    const chose: unknown[] = [];
    const drops: TokenDrop[] = [];
    let confirmed = 0;
    fixture.componentInstance.choose.subscribe((s) => chose.push(s));
    fixture.componentInstance.tokenDrop.subscribe((d) => drops.push(d));
    fixture.componentInstance.confirm.subscribe(() => confirmed++);
    return { fixture, root, surface, chose, drops, confirmed: () => confirmed };
  }

  function key(target: HTMLElement, k: string): KeyboardEvent {
    const event = new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true });
    target.dispatchEvent(event);
    return event;
  }

  function pointer(
    target: HTMLElement,
    type: string,
    over: { x?: number; y?: number; id?: number; button?: number } = {},
  ): void {
    const event = new MouseEvent(type, {
      bubbles: true,
      clientX: over.x ?? 0,
      clientY: over.y ?? 0,
      button: over.button ?? 0,
    });
    Object.defineProperty(event, 'pointerId', { value: over.id ?? 1 });
    target.dispatchEvent(event);
  }

  function tokenEl(root: HTMLElement, id: string): HTMLElement {
    return root.querySelector<HTMLElement>(`[data-id="${id}"]`)!;
  }

  describe('the keyboard', () => {
    it('does nothing where nobody may move, and leaves the keys alone', () => {
      const { surface, drops } = setup();
      expect(key(surface, 'ArrowRight').defaultPrevented).toBe(false);
      expect(key(surface, 'Enter').defaultPrevented).toBe(false);
      expect(drops).toEqual([]);
    });

    it('leaves Enter and the arrows to a door button', () => {
      const { surface, drops } = setup({ masterMoves: true });
      const door = document.createElement('button');
      door.setAttribute('data-door', '');
      surface.append(door);
      expect(key(door, 'Enter').defaultPrevented).toBe(false);
      expect(key(door, 'ArrowDown').defaultPrevented).toBe(false);
      expect(drops).toEqual([]);
    });

    it('moves a cursor from the mover token with the arrows, and Enter drops the token there', () => {
      const { surface, root, drops, fixture } = setup({ ownMoveId: 'a' });
      expect(key(surface, 'ArrowRight').defaultPrevented).toBe(true);
      key(surface, 'ArrowDown');
      fixture.detectChanges();
      expect(root.querySelector('.cm__frame, [class*="frame"]')).not.toBeNull();
      expect(key(surface, 'Enter').defaultPrevented).toBe(true);
      expect(drops).toEqual([{ id: 'a', col: 9, row: 8 }]);
      // The cursor is gone: a second Enter drops nothing.
      key(surface, 'Enter');
      expect(drops).toHaveLength(1);
    });

    it('starts the cursor from the token on turn when nobody is picked', () => {
      const { surface, drops } = setup({ masterMoves: true, currentId: 'b' });
      key(surface, 'ArrowDown');
      // Enter needs a picked or own token: the master has picked none.
      key(surface, 'Enter');
      expect(drops).toEqual([]);
    });

    it('drops the picked token of the master where the cursor is', () => {
      const { surface, root, drops } = setup({ masterMoves: true });
      pointer(tokenEl(root, 'b'), 'pointerdown');
      pointer(tokenEl(root, 'b'), 'pointerup');
      key(surface, 'ArrowUp');
      key(surface, 'ArrowLeft');
      key(surface, 'Enter');
      expect(drops).toEqual([{ id: 'b', col: 1, row: 1 }]);
    });

    it('does nothing with an arrow when there is no square to start from', () => {
      const { surface, drops } = setup({ masterMoves: true });
      expect(key(surface, 'ArrowDown').defaultPrevented).toBe(true);
      key(surface, 'Enter');
      expect(drops).toEqual([]);
    });

    it('clears the cursor on Escape and on blur, and ignores other keys', () => {
      const { surface, drops, fixture } = setup({ ownMoveId: 'a' });
      expect(key(surface, 'x').defaultPrevented).toBe(false);
      key(surface, 'ArrowRight');
      key(surface, 'Escape');
      key(surface, 'Enter');
      expect(drops).toEqual([]);
      key(surface, 'ArrowRight');
      surface.dispatchEvent(new FocusEvent('blur'));
      key(surface, 'Enter');
      expect(drops).toEqual([]);
      fixture.detectChanges();
    });

    it('in "pickSquares" mode the arrows choose a square from the chosen one, then from the origin of the reach, and Enter confirms', () => {
      const { surface, chose, confirmed, fixture } = setup({
        pickSquares: true,
        ownMoveId: 'a',
        reach: { origin: { col: 8, row: 7 }, leftDft: 300, squares: [] },
      });
      key(surface, 'ArrowRight');
      expect(chose).toEqual([{ col: 9, row: 7 }]);
      fixture.componentRef.setInput('chosen', { square: { col: 9, row: 7 }, refused: false });
      fixture.detectChanges();
      key(surface, 'ArrowUp');
      expect(chose[1]).toEqual({ col: 9, row: 6 });
      expect(key(surface, 'Enter').defaultPrevented).toBe(true);
      expect(confirmed()).toBe(1);
    });

    it('in "pickSquares" mode with no reach and no chosen square an arrow chooses nothing', () => {
      const { surface, chose } = setup({ pickSquares: true });
      key(surface, 'ArrowUp');
      expect(chose).toEqual([]);
    });
  });

  describe('the pointer', () => {
    it('drags a token the master may move, and reports the drop on release', () => {
      const { root, drops } = setup({ masterMoves: true });
      const el = tokenEl(root, 'a');
      pointer(el, 'pointerdown', { id: 5 });
      pointer(el, 'pointermove', { id: 5, x: 1250, y: 450 });
      pointer(el, 'pointerup', { id: 5 });
      expect(drops).toEqual([{ id: 'a', col: 12, row: 4 }]);
    });

    it('draws the token where it is dragged, and puts it back on release', () => {
      const { root, fixture } = setup({ masterMoves: true });
      const el = tokenEl(root, 'a');
      pointer(el, 'pointerdown');
      pointer(el, 'pointermove', { x: 150, y: 150 });
      fixture.detectChanges();
      expect(tokenEl(root, 'a').style.left).toBe(`${((1 + 0.5) / 20) * 100}%`);
      pointer(el, 'pointerup');
      fixture.detectChanges();
      expect(tokenEl(root, 'a').style.left).toBe(`${((8 + 0.5) / 20) * 100}%`);
    });

    it('lets a player drag their own token only', () => {
      const { root, drops } = setup({ ownMoveId: 'b' });
      pointer(tokenEl(root, 'a'), 'pointerdown');
      pointer(tokenEl(root, 'a'), 'pointermove', { x: 100, y: 100 });
      pointer(tokenEl(root, 'a'), 'pointerup');
      expect(drops).toEqual([]);
      pointer(tokenEl(root, 'b'), 'pointerdown');
      pointer(tokenEl(root, 'b'), 'pointermove', { x: 100, y: 100 });
      pointer(tokenEl(root, 'b'), 'pointerup');
      expect(drops).toEqual([{ id: 'b', col: 1, row: 1 }]);
    });

    it('drops nothing for a click without a move, a cancel, another pointer, a secondary button or a press off the tokens', () => {
      const { root, surface, drops } = setup({ masterMoves: true });
      const el = tokenEl(root, 'a');
      pointer(el, 'pointerdown');
      pointer(el, 'pointerup');
      pointer(el, 'pointerdown');
      pointer(el, 'pointermove', { x: 300, y: 300 });
      pointer(el, 'pointercancel');
      pointer(el, 'pointerdown', { id: 1 });
      pointer(el, 'pointermove', { id: 2, x: 300, y: 300 });
      pointer(el, 'pointerup', { id: 2 });
      pointer(el, 'pointerdown', { button: 2 });
      pointer(el, 'pointermove', { x: 300, y: 300 });
      pointer(surface, 'pointerdown');
      pointer(surface, 'pointermove', { x: 300, y: 300 });
      pointer(surface, 'pointerup');
      pointer(surface, 'pointerup');
      expect(drops).toEqual([]);
    });

    it('chooses the square clicked in "pickSquares" mode, and not one under a token', () => {
      const { root, surface, chose } = setup({ pickSquares: true, ownMoveId: 'a' });
      surface.dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: 350, clientY: 250 }));
      expect(chose).toEqual([{ col: 3, row: 2 }]);
      tokenEl(root, 'b').dispatchEvent(new MouseEvent('click', { bubbles: true }));
      expect(chose).toHaveLength(1);
    });

    it('ignores a click when squares are not being picked', () => {
      const { surface, chose } = setup({ masterMoves: true });
      surface.dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: 350, clientY: 250 }));
      expect(chose).toEqual([]);
    });
  });

  describe('what it draws', () => {
    function lineOf(root: HTMLElement): SVGLineElement | null {
      return root.querySelector('line.cm__path, line[class*="path"]');
    }

    it('keeps the cost pill off a door and off the edge of the map', () => {
      const { fixture } = setup({
        layers: { doors: [{ col: 4, row: 6 }] },
      });
      const map = fixture.componentInstance as unknown as {
        costAbove(at: { col: number; row: number }): boolean;
      };
      expect(map.costAbove({ col: 3, row: 5 })).toBe(false);
      expect(map.costAbove({ col: 4, row: 5 })).toBe(true);
      expect(map.costAbove({ col: 4, row: 12 })).toBe(true);
      expect(map.costAbove({ col: 4, row: 0 })).toBe(false);
      expect(lineOf(fixture.nativeElement)).toBeNull();
    });

    it('describes each token for a screen reader, with its conditions', () => {
      const { root } = setup({
        combatants: [combatant({ id: 'a', label: 'Toren', col: 8, row: 7 })],
      });
      expect(root.textContent).toContain('Toren, coluna 9, linha 8');
    });
  });
});
