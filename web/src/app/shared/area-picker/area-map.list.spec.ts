import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';
import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  AreaPlacement,
  CombatantKind,
  TargetInReachSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { SpellAreaShape } from '../../../gen/meurpg/rules/v1/rules_pb';
import type { AreaPick } from '../../core/combat/area-flow';
import { combatant } from '../../core/combat/combat-testing';
import { nextTurn } from '../../../test-settle';
import { AreaMap } from './area-map';

afterEach(() => {
  vi.useRealTimers();
});

/** Each square is 20 px on the stubbed 240 x 160 map (12 x 8 squares). */
const SQ = 20;

const caster = combatant({
  id: 'p',
  label: 'Pensantus',
  kind: CombatantKind.PLAYER,
  mine: true,
  col: 2,
  row: 4,
});
const goblin = combatant({ id: 'g1', label: 'Goblin 1', col: 6, row: 4 });
const fireball = { shape: SpellAreaShape.SPHERE, sizeFt: 20, widthFt: 0 };

/** The list of "Centrar em…", the keys that move the point and the pointer kinds the other AreaMap specs leave. */
function setup(over: { placement?: AreaPlacement; casterOnMap?: boolean; rangeFt?: number } = {}) {
  const fixture = TestBed.createComponent(AreaMap);
  const ref = fixture.componentRef;
  ref.setInput('image', { url: 'map.png', width: 1200, height: 800 });
  ref.setInput('columns', 12);
  ref.setInput('rows', 8);
  ref.setInput('combatants', over.casterOnMap === false ? [goblin] : [caster, goblin]);
  ref.setInput('casterId', 'p');
  ref.setInput('spell', 'Bola de Fogo');
  ref.setInput('placement', over.placement ?? AreaPlacement.POINT);
  ref.setInput('area', fireball);
  ref.setInput('rangeFt', over.rangeFt ?? 0);
  ref.setInput('seen', [
    create(TargetInReachSchema, { combatantId: 'g1', label: 'Goblin 1', distanceFt: 20 }),
    create(TargetInReachSchema, { combatantId: 'ghost', label: 'Fantasma' }),
  ]);
  const places: (AreaPick | null)[] = [];
  let confirms = 0;
  fixture.componentInstance.place.subscribe((p) => {
    places.push(p);
    ref.setInput('placed', p);
  });
  fixture.componentInstance.confirm.subscribe(() => confirms++);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const app = el.querySelector<HTMLElement>('[role="application"]')!;
  app.getBoundingClientRect = () =>
    ({ left: 0, top: 0, width: 12 * SQ, height: 8 * SQ, right: 240, bottom: 160 }) as DOMRect;
  const key = (k: string, shiftKey = false, target: HTMLElement = app) => {
    const ev = new KeyboardEvent('keydown', { key: k, shiftKey, bubbles: true, cancelable: true });
    target.dispatchEvent(ev);
    fixture.detectChanges();
    return ev;
  };
  const pointer = (type: string, col: number, row: number, kind = 'mouse', button = 0) => {
    const ev = new MouseEvent(type, {
      bubbles: true,
      clientX: col * SQ + SQ / 2,
      clientY: row * SQ + SQ / 2,
      button,
    });
    Object.defineProperty(ev, 'pointerType', { value: kind });
    Object.defineProperty(ev, 'pointerId', { value: 3 });
    app.dispatchEvent(ev);
    fixture.detectChanges();
  };
  async function openList(): Promise<HTMLElement> {
    key('c');
    await fixture.whenStable();
    return el.querySelector<HTMLElement>('[role="listbox"]')!;
  }
  const active = () => el.querySelector('.center__opt--active')?.textContent?.trim() ?? '';
  return { fixture, el, app, places, key, pointer, openList, active, confirms: () => confirms };
}

describe('AreaMap, the list "Centrar em…"', () => {
  it('moves the active option with the arrows, wrapping, and Home and End', async () => {
    const { key, openList, active } = setup();
    const list = await openList();
    expect(active()).toContain('Goblin 1');
    key('ArrowDown', false, list);
    expect(active()).toContain('Pensantus');
    key('ArrowDown', false, list);
    key('ArrowDown', false, list);
    expect(active()).toContain('Goblin 1');
    key('ArrowUp', false, list);
    expect(active()).toContain('Um quadrado');
    key('Home', false, list);
    expect(active()).toContain('Goblin 1');
    key('End', false, list);
    expect(active()).toContain('Um quadrado');
    expect(key('x', false, list).defaultPrevented).toBe(false);
  });

  it('places on the caster with Space, and a second pick of it confirms', async () => {
    const { key, openList, places, confirms } = setup();
    const list = await openList();
    key('ArrowDown', false, list);
    key(' ', false, list);
    expect(places).toEqual([{ kind: 'point', square: { col: 2, row: 4 } }]);
    const again = await openList();
    key('ArrowDown', false, again);
    key('Enter', false, again);
    expect(confirms()).toBe(1);
  });

  it('closes on Escape without leaving the step, and the button toggles it', async () => {
    const { fixture, el, key, openList } = setup();
    const list = await openList();
    const escape = key('Escape', false, list);
    expect(escape.defaultPrevented).toBe(true);
    expect(el.querySelector('[role="listbox"]')).toBeNull();
    el.querySelector<HTMLButtonElement>('.center__btn')!.click();
    fixture.detectChanges();
    expect(el.querySelector('[role="listbox"]')).not.toBeNull();
    el.querySelector<HTMLButtonElement>('.center__btn')!.click();
    fixture.detectChanges();
    expect(el.querySelector('[role="listbox"]')).toBeNull();
  });

  it('hands the arrows back to the map on "Um quadrado do mapa" and places nothing', async () => {
    const { fixture, el, key, openList, places } = setup();
    const list = await openList();
    key('End', false, list);
    key('Enter', false, list);
    expect(places).toEqual([]);
    expect(el.querySelector('[role="listbox"]')).toBeNull();
    fixture.detectChanges();
  });

  it('ignores a click between the options and picks the one clicked', async () => {
    const { fixture, el, openList, places } = setup();
    const list = await openList();
    list.click();
    expect(places).toEqual([]);
    el.querySelectorAll<HTMLElement>('[role="option"]')[1].click();
    fixture.detectChanges();
    expect(places).toEqual([{ kind: 'point', square: { col: 2, row: 4 } }]);
  });

  it('leaves the caster out of the list when it is not on the map, and the creatures it does not find', async () => {
    const { openList } = setup({ casterOnMap: false });
    const list = await openList();
    const names = Array.from(list.querySelectorAll('.center__name')).map((n) => n.textContent);
    expect(names).toEqual(['Goblin 1', 'Um quadrado do mapa (use as setas)']);
  });

  it('does not open for a read-only map', () => {
    const { fixture, el, key } = setup();
    fixture.componentRef.setInput('readOnly', true);
    fixture.detectChanges();
    key('c');
    expect(el.querySelector('[role="listbox"]')).toBeNull();
  });
});

describe('AreaMap, keys and pointers', () => {
  it('places with Enter and Space on the hovered point, and does nothing on Enter without one', () => {
    const { key, pointer, places } = setup();
    key('Enter');
    expect(places).toEqual([]);
    pointer('pointermove', 5, 2);
    key(' ');
    expect(places).toEqual([{ kind: 'point', square: { col: 5, row: 2 } }]);
  });

  it('keeps the point on the map, and starts from the corner when the caster is not on it', () => {
    const { key, places } = setup({ casterOnMap: false });
    key('ArrowLeft');
    key('Enter');
    expect(places).toEqual([{ kind: 'point', square: { col: 0, row: 0 } }]);
  });

  it('turns a direction toward the first one when none is chosen, and ignores other keys', () => {
    const { key, places } = setup({ placement: AreaPlacement.DIRECTION });
    expect(key('q').defaultPrevented).toBe(false);
    key('ArrowRight');
    key('Enter');
    expect(places).toEqual([{ kind: 'direction', direction: expect.anything() }]);
  });

  it('takes a pen like a finger, ignores another pointer, and a cancel clears the finger', () => {
    const { el, pointer, places } = setup();
    pointer('pointerdown', 4, 2, 'pen');
    expect(el.querySelector('.am__finger, [class*="finger"]')).not.toBeNull();
    pointer('pointercancel', 4, 2, 'pen');
    pointer('pointerup', 4, 2, 'pen');
    expect(places).toEqual([]);
    pointer('pointermove', 4, 2, 'touch');
    pointer('pointerleave', 4, 2, 'touch');
    expect(places).toEqual([]);
  });

  it('ignores a secondary button, and the mouse leaving clears the template', () => {
    const { pointer, places, el } = setup();
    pointer('pointerup', 4, 2, 'mouse', 2);
    expect(places).toEqual([]);
    pointer('pointermove', 4, 2);
    expect(cells(el)).toBeGreaterThan(0);
    pointer('pointerleave', 4, 2);
    expect(cells(el)).toBe(0);
  });

  it('moves the placed point with the nudge buttons, and not when a direction is placed', () => {
    const { fixture, el, places } = setup();
    el.querySelector<HTMLButtonElement>('[aria-label="Um quadrado para a direita"]')?.click();
    expect(places).toEqual([]);
    fixture.componentRef.setInput('placed', { kind: 'point', square: { col: 11, row: 0 } });
    fixture.detectChanges();
    el.querySelector<HTMLButtonElement>('[aria-label="Um quadrado para a direita"]')!.click();
    el.querySelector<HTMLButtonElement>('[aria-label="Um quadrado para cima"]')!.click();
    expect(places).toEqual([]);
  });

  it('says out loud, at once, why a tap outside the range placed nothing', async () => {
    vi.useFakeTimers();
    const { fixture, el, pointer, places } = setup({ rangeFt: 15 });
    pointer('pointerup', 11, 7);
    await nextTurn();
    fixture.detectChanges();
    expect(places).toEqual([null]);
    expect(el.querySelector('[role="status"]')?.textContent).toContain('Fora do alcance.');
  });
});

function cells(el: HTMLElement): number {
  return (el.querySelector('.am__fill')?.getAttribute('d') ?? '').split('M').length - 1;
}
