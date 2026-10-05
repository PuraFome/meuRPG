import { TestBed } from '@angular/core/testing';
import { textOf } from '../../core/format/text-testing';
import { beforeEach, describe, expect, it } from 'vitest';

import { ViewAsList, type ViewAsPerson } from './view-as-list';

const plain = textOf;
const people: ViewAsPerson[] = [
  { id: 'p', name: 'Pensantus', sub: 'Vinicius' },
  { id: 't', name: 'Toren', sub: 'Caio' },
  { id: 'b', name: 'Brisa', sub: 'Lia' },
];

describe('ViewAsList', () => {
  beforeEach(() => TestBed.configureTestingModule({ imports: [ViewAsList] }));

  function create(inputs: Record<string, unknown> = {}) {
    const fixture = TestBed.createComponent(ViewAsList);
    fixture.componentRef.setInput('people', people);
    for (const [k, v] of Object.entries(inputs)) {
      fixture.componentRef.setInput(k, v);
    }
    const chosen: (string | null)[] = [];
    fixture.componentInstance.choose.subscribe((c) => chosen.push(c));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { fixture, el, chosen, radios: () => Array.from(el.querySelectorAll<HTMLElement>('[role="radio"]')) };
  }

  it('lists "Todos" and one row for each character, with how many squares they see, counted from the states', () => {
    const { radios } = create({ counts: new Map([['p', 76], ['t', 22], ['b', 1]]), total: 384 });
    expect(radios().map((r) => plain(r))).toEqual([
      'Todos Sem névoa: o seu mapa de mestre',
      'Pensantus Vinicius 76 quadrados vistos',
      'Toren Caio 22 quadrados vistos',
      'Brisa Lia 1 quadrado visto',
    ]);
  });

  it('says a count could not be read, and shows no count while it is being read', () => {
    const { radios } = create({ counts: new Map<string, number | null>([['t', null]]) });
    expect(plain(radios()[1])).toBe('Pensantus Vinicius');
    expect(plain(radios()[2])).toBe('Toren Caio Não deu para ler');
  });

  it('is a radio group named "Ver como", with "Todos" checked at first and one stop in the tab order', () => {
    const { el, radios } = create();
    expect(el.querySelector('[role="radiogroup"]')?.getAttribute('aria-labelledby')).toBe('view-as-title');
    expect(radios().map((r) => r.getAttribute('aria-checked'))).toEqual(['true', 'false', 'false', 'false']);
    expect(radios().map((r) => r.getAttribute('tabindex'))).toEqual(['0', '-1', '-1', '-1']);
  });

  it('moves between the rows with the arrows (wrapping, Home and End) and chooses with Enter', () => {
    const { fixture, radios, chosen } = create({ selected: null });
    radios()[0].focus();
    radios()[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
    fixture.detectChanges();
    expect(document.activeElement).toBe(radios()[1]);
    expect(radios().map((r) => r.getAttribute('tabindex'))).toEqual(['-1', '0', '-1', '-1']);
    // Moving never chooses.
    expect(chosen).toEqual([]);
    radios()[1].dispatchEvent(new KeyboardEvent('keydown', { key: 'End', bubbles: true }));
    expect(document.activeElement).toBe(radios()[3]);
    radios()[3].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
    expect(document.activeElement).toBe(radios()[0]);
    radios()[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true }));
    expect(document.activeElement).toBe(radios()[3]);
    // A native button: Enter and Space click it.
    radios()[3].click();
    expect(chosen).toEqual(['b']);
  });

  it('asks only for a change: choosing the row already chosen says nothing', () => {
    const { radios, chosen } = create({ selected: 't' });
    radios()[2].click();
    expect(chosen).toEqual([]);
    radios()[0].click();
    expect(chosen).toEqual([null]);
  });

  it('says "Visão do grupo" in words, on or off, and a line about the chosen view', () => {
    const off = create();
    expect(plain(off.el.querySelector('.va__group'))).toBe('Visão do grupo: desligada neste mapa. Muda no editor do mapa.');
    const on = create({ groupVision: true, note: 'Toren vê 22 quadrados de 384 e nenhum inimigo.' });
    expect(plain(on.el.querySelector('.va__group'))).toContain('ligada neste mapa');
    expect(plain(on.el.querySelector('.va__note'))).toContain('Toren vê 22 quadrados');
  });
});
