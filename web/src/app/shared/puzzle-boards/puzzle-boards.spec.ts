import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { PuzzleKind, PuzzleRunSchema } from '../../../gen/meurpg/play/v1/puzzles_pb';
import { GLYPHS, RUNES, pillarFaces } from '../../core/puzzles/puzzle-symbols';
import { LightsBoard, type LightPress } from './lights-board';
import { LockBoard } from './lock-board';
import { PillarsBoard } from './pillars-board';
import { PuzzleHost } from './puzzle-host';
import type { Turn } from './symbol-columns';

const flat = (n: number, on: readonly number[]) => Array.from({ length: n * n }, (_, i) => on.includes(i));

function lights(inputs: Record<string, unknown>) {
  const fixture = TestBed.createComponent(LightsBoard);
  for (const [k, v] of Object.entries(inputs)) {
    fixture.componentRef.setInput(k, v);
  }
  const presses: LightPress[] = [];
  fixture.componentInstance.press.subscribe((p) => presses.push(p));
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, presses };
}

describe('LightsBoard (E10-06 states 6 to 9)', () => {
  it('names every light by its row, column and state, so no light is only a color', () => {
    const { el } = lights({ size: 3, lit: flat(3, [4]), mode: 'play' });
    const labels = Array.from(el.querySelectorAll('button')).map((b) => b.getAttribute('aria-label'));
    expect(labels).toHaveLength(9);
    expect(labels[0]).toBe('Luz na linha 1, coluna 1, apagada');
    expect(labels[4]).toBe('Luz na linha 2, coluna 2, acesa');
    expect(el.querySelector('[role="group"]')?.getAttribute('aria-label')).toBe('Painel de luzes, 3 por 3');
  });

  it('draws a lit light as a sun and an off one as a ring', () => {
    const { el } = lights({ size: 3, lit: flat(3, [0]), mode: 'view' });
    const cells = Array.from(el.querySelectorAll('.cell'));
    expect(cells[0].classList).toContain('cell--lit');
    expect(cells[0].querySelectorAll('path').length).toBe(1); // the rays
    expect(cells[1].classList).not.toContain('cell--lit');
    expect(cells[1].querySelector('circle')?.getAttribute('fill')).toBe('none');
  });

  it('emits the row and column of a pressed light (counted from 0)', () => {
    const { el, presses } = lights({ size: 5, lit: flat(5, []), mode: 'play' });
    (el.querySelector('[data-i="13"]') as HTMLElement).click();
    expect(presses).toEqual([{ row: 2, col: 3 }]);
  });

  it('is a static board for the master: nothing in it is a button', () => {
    const { el, presses } = lights({ size: 3, lit: flat(3, [1]), mode: 'view' });
    expect(el.querySelectorAll('button')).toHaveLength(0);
    expect(el.querySelectorAll('[role="img"]')).toHaveLength(9);
    (el.querySelector('.cell') as HTMLElement).click();
    expect(presses).toEqual([]);
  });

  it('does nothing once it is solved or stopped, and says so with aria-disabled', () => {
    const { el, presses } = lights({ size: 3, lit: flat(3, []), mode: 'play', disabled: true });
    const first = el.querySelector('button') as HTMLButtonElement;
    expect(first.getAttribute('aria-disabled')).toBe('true');
    first.click();
    expect(presses).toEqual([]);
  });

  it('has one tab stop, and the arrow keys walk the squares', () => {
    const { el, fixture } = lights({ size: 3, lit: flat(3, []), mode: 'play' });
    document.body.append(el);
    const stops = () => Array.from(el.querySelectorAll('button')).filter((b) => b.tabIndex === 0);
    expect(stops()).toHaveLength(1);
    const first = el.querySelector('[data-i="0"]') as HTMLElement;
    first.focus();
    first.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    expect(document.activeElement).toBe(el.querySelector('[data-i="1"]'));
    (document.activeElement as HTMLElement).dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
    expect(document.activeElement).toBe(el.querySelector('[data-i="4"]'));
    (document.activeElement as HTMLElement).dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }));
    (document.activeElement as HTMLElement).dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }));
    expect(document.activeElement).toBe(el.querySelector('[data-i="3"]')); // the edge holds
    fixture.detectChanges();
    expect(stops()).toHaveLength(1);
    el.remove();
  });

  it('marks what just changed (dashed) and the master\'s hints (solid garnet)', () => {
    const { el } = lights({ size: 3, lit: flat(3, []), mode: 'view', changed: [1, 2], hints: [8] });
    const cells = Array.from(el.querySelectorAll('.cell'));
    expect(cells[1].classList).toContain('cell--changed');
    expect(cells[2].classList).toContain('cell--changed');
    expect(cells[8].classList).toContain('cell--hint');
    expect(cells[0].classList).not.toContain('cell--changed');
  });
});

describe('LockBoard and PillarsBoard (E10-06 states 4 and 7)', () => {
  it('shows each wheel\'s face with its name written, and turns it a face at a time', () => {
    const fixture = TestBed.createComponent(LockBoard);
    fixture.componentRef.setInput('wheels', [0, 0, 2, 5]);
    fixture.componentRef.setInput('faces', RUNES);
    fixture.componentRef.setInput('mode', 'play');
    const turns: Turn[] = [];
    fixture.componentInstance.turn.subscribe((t) => turns.push(t));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(Array.from(el.querySelectorAll('.name')).map((n) => n.textContent?.trim())).toEqual(['Lua', 'Lua', 'Onda', 'Estrela']);
    expect(Array.from(el.querySelectorAll('[role="group"][aria-label^="Roda "]')).map((g) => g.getAttribute('aria-label'))).toEqual(['Roda 1: Lua', 'Roda 2: Lua', 'Roda 3: Onda', 'Roda 4: Estrela']);
    (el.querySelector('[aria-label="Próximo símbolo: Roda 3"]') as HTMLElement).click();
    (el.querySelector('[aria-label="Símbolo anterior: Roda 1"]') as HTMLElement).click();
    expect(turns).toEqual([
      { index: 2, delta: 1 },
      { index: 0, delta: -1 },
    ]);
  });

  it('draws a digit as the character, with no second name beside it', () => {
    const fixture = TestBed.createComponent(LockBoard);
    fixture.componentRef.setInput('wheels', [3]);
    fixture.componentRef.setInput('faces', [{ key: '0', namePt: '0' }, { key: '1', namePt: '1' }, { key: '2', namePt: '2' }, { key: '3', namePt: '3' }]);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('.ch')?.textContent).toBe('3');
    expect(el.querySelector('.name')).toBeNull();
  });

  it('has no control in the master\'s static view', () => {
    const fixture = TestBed.createComponent(LockBoard);
    fixture.componentRef.setInput('wheels', [1, 2]);
    fixture.componentRef.setInput('faces', RUNES);
    fixture.componentRef.setInput('changed', [1]);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelectorAll('button')).toHaveLength(0);
    expect(el.querySelectorAll('.col--changed')).toHaveLength(1);
  });

  it('turns a pillar with "Girar", forward only, and names the pillar', () => {
    const fixture = TestBed.createComponent(PillarsBoard);
    fixture.componentRef.setInput('pillars', [3, 0, 2, 1]);
    fixture.componentRef.setInput('faces', pillarFaces(4));
    fixture.componentRef.setInput('mode', 'play');
    const turns: Turn[] = [];
    fixture.componentInstance.turn.subscribe((t) => turns.push(t));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(Array.from(el.querySelectorAll('.head')).map((h) => h.textContent?.trim())).toEqual(['Pilar 1', 'Pilar 2', 'Pilar 3', 'Pilar 4']);
    expect(Array.from(el.querySelectorAll('.name')).map((n) => n.textContent?.trim())).toEqual(['Coruja', 'Corvo', 'Serpente', 'Lobo']);
    const girar = Array.from(el.querySelectorAll<HTMLButtonElement>('.turn'));
    expect(girar).toHaveLength(4);
    expect(girar[0].getAttribute('aria-label')).toBe('Girar o pilar 1');
    girar[1].click();
    expect(turns).toEqual([{ index: 1, delta: 1 }]);
    expect(el.querySelectorAll('.step')).toHaveLength(0);
  });

  it('gives the master arrows to choose the mural', () => {
    const fixture = TestBed.createComponent(PillarsBoard);
    fixture.componentRef.setInput('pillars', [0, 1, 2]);
    fixture.componentRef.setInput('faces', GLYPHS.slice(0, 3));
    fixture.componentRef.setInput('mode', 'edit');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelectorAll('.step')).toHaveLength(6);
    expect(el.querySelectorAll('.turn')).toHaveLength(0);
  });
});

describe('PuzzleHost (the one place that knows the boards)', () => {
  function host(run: ReturnType<typeof create<typeof PuzzleRunSchema>>) {
    const fixture = TestBed.createComponent(PuzzleHost);
    fixture.componentRef.setInput('run', run);
    fixture.componentRef.setInput('mode', 'play');
    const moves: unknown[] = [];
    fixture.componentInstance.move.subscribe((m) => moves.push(m));
    fixture.detectChanges();
    return { el: fixture.nativeElement as HTMLElement, moves };
  }

  it('turns a press on the lights into the server\'s move', () => {
    const { el, moves } = host(create(PuzzleRunSchema, { kind: PuzzleKind.LIGHTS, config: { kind: { case: 'lights', value: { size: 3 } } }, state: { kind: { case: 'lights', value: { lit: flat(3, [0]) } } } }));
    (el.querySelector('[data-i="5"]') as HTMLElement).click();
    expect(moves).toEqual([{ kind: { case: 'lights', value: { row: 1, col: 2 } } }]);
  });

  it('turns a wheel and a pillar into the server\'s moves', () => {
    const lock = host(create(PuzzleRunSchema, { kind: PuzzleKind.LOCK, symbols: RUNES.map((s) => ({ key: s.key, namePt: s.namePt })), state: { kind: { case: 'lock', value: { wheels: [0, 1] } } } }));
    (lock.el.querySelector('[aria-label="Símbolo anterior: Roda 2"]') as HTMLElement).click();
    expect(lock.moves).toEqual([{ kind: { case: 'lock', value: { wheel: 1, delta: -1 } } }]);
    const pillars = host(create(PuzzleRunSchema, { kind: PuzzleKind.PILLARS, symbols: GLYPHS.slice(0, 3).map((s) => ({ key: s.key, namePt: s.namePt })), state: { kind: { case: 'pillars', value: { pillars: [0, 1, 2] } } } }));
    (pillars.el.querySelector('[aria-label="Girar o pilar 3"]') as HTMLElement).click();
    expect(pillars.moves).toEqual([{ kind: { case: 'pillars', value: { pillar: 2, delta: 1 } } }]);
  });

  it('says so, instead of drawing nothing, for a kind the app does not draw yet', () => {
    const { el } = host(create(PuzzleRunSchema, { kind: PuzzleKind.RIDDLE, state: { kind: { case: 'riddle', value: {} } } }));
    expect(el.textContent).toContain('ainda não abre aqui');
  });
});
