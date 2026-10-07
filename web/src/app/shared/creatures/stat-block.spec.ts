import { TestBed } from '@angular/core/testing';

import { raven, flat } from '../../core/creatures/creatures-testing';
import { StatBlock } from './stat-block';

describe('StatBlock (E9-10, quadro 3)', () => {
  function render(hp: { current: number; max: number } | null = { current: 1, max: 1 }) {
    const fixture = TestBed.createComponent(StatBlock);
    fixture.componentRef.setInput('creature', raven());
    fixture.componentRef.setInput('hp', hp);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { el, flat };
  }

  it("draws what the server sent: CA, the creature's own PV, the speeds in metres, one tile each", () => {
    const { el, flat } = render();
    expect(Array.from(el.querySelectorAll('.tile')).map((t) => flat(t))).toEqual([
      'CA 12',
      'PV 1 de 1',
      'Deslocamento 3 m',
      'Voo 15 m',
    ]);
  });

  it("without a creature's own PV (the book's), shows the average and the dice", () => {
    const { el, flat } = render(null);
    expect(flat(el.querySelectorAll('.tile')[1])).toBe('PV 1 (1d4-1)');
  });

  it('the six abilities with the true minus sign, the modifier and the score', () => {
    const { el, flat } = render();
    const items = Array.from(el.querySelectorAll('.ability')).map((a) => flat(a));
    expect(items).toEqual([
      'Força −4 valor 2',
      'Destreza +2 valor 14',
      'Constituição −1 valor 8',
      'Inteligência −4 valor 2',
      'Sabedoria +1 valor 12',
      'Carisma −2 valor 6',
    ]);
  });

  it('the lines carry Portuguese labels; the book\'s text is English, marked once and tagged lang="en"', () => {
    const { el, flat } = render();
    const lines = Array.from(el.querySelectorAll('.line')).map((l) => flat(l));
    expect(lines).toEqual([
      'Perícias Percepção +3',
      'Sentidos Percepção passiva 13',
      'Idiomas —',
      'Desafio 0',
    ]);
    expect(flat(el.querySelector('.srd'))).toBe(
      'Os textos abaixo são do livro de regras (SRD 5.1), em inglês.',
    );
    const entries = Array.from(el.querySelectorAll('.entry'));
    expect(entries.map((e) => e.getAttribute('lang'))).toEqual(['en', 'en']);
    expect(flat(entries[0].querySelector('h3'))).toBe('Mimicry');
    // The traits get a heading like the actions, and the headings go in order (h2, then h3 for each entry).
    expect(Array.from(el.querySelectorAll('h2')).map((h) => flat(h))).toEqual([
      'Características',
      'Ações',
    ]);
  });

  it('has no "Atacar" button: it is a page to read', () => {
    const { el } = render();
    expect(el.querySelector('button')).toBeNull();
  });
});
