import { TestBed } from '@angular/core/testing';

import type { EffectCardView } from '../../../../core/effects/effects';
import { EffectCards } from './effect-cards';

const plain = (text: string | null | undefined) =>
  (text ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

const hold: EffectCardView = {
  key: 'e1',
  name: 'Imobilizar Pessoa',
  origin: 'De alguém que você não vê',
  tags: [],
  clock: 'Resta o tempo da magia: acaba no turno de quem a conjurou, na rodada 12.',
  save: 'No fim de cada turno seu: teste de resistência de Sabedoria.',
  changes: [
    'Você não age nem se move, e não fala.',
    'Falha em testes de resistência de Força e de Destreza.',
  ],
};

const bless: EffectCardView = {
  key: 'e2',
  name: 'Bênção',
  origin: 'De Tavo',
  tags: ['+1d4 em ataques e resistências'],
  clock: 'Restam 8 rodadas: acaba no turno de Tavo, na rodada 11.',
  save: '',
  changes: [],
};

describe('EffectCards', () => {
  function setup(cards: readonly EffectCardView[], exhaustion = 0) {
    const fixture = TestBed.createComponent(EffectCards);
    fixture.componentRef.setInput('cards', cards);
    fixture.componentRef.setInput('exhaustion', exhaustion);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('draws nothing without effects and without exhaustion', () => {
    const { el } = setup([]);
    expect(el.querySelector('section')).toBeNull();
  });

  it.each([
    ['390 px (a phone, one column)', 390],
    ['1280 px (a desktop, the same cards)', 1280],
  ])('draws a card for each effect, named and grouped, at %s', (_name, width) => {
    const { el } = setup([hold, bless]);
    el.style.width = `${width}px`;
    expect(plain(el.querySelector('h3')?.textContent)).toBe('Seus efeitos');
    const groups = Array.from(el.querySelectorAll<HTMLElement>('[role="group"].fx__card'));
    expect(groups).toHaveLength(2);
    // Each card is a group named by its own title.
    for (const [i, g] of groups.entries()) {
      const titleId = g.getAttribute('aria-labelledby')!;
      expect(plain(el.querySelector(`#${titleId}`)?.textContent)).toBe([hold, bless][i].name);
    }
    // The cards stack in one list that does not scroll sideways: every box wraps its words.
    expect(el.querySelector('ul.fx__list')).not.toBeNull();
  });

  it('writes whose the effect is, its labels, its clock and the save it asks, never a DC', () => {
    const { el } = setup([hold, bless]);
    const text = plain(el.textContent);
    expect(text).toContain('De alguém que você não vê');
    expect(text).toContain('De Tavo');
    expect(text).toContain('+1d4 em ataques e resistências');
    expect(text).toContain('Restam 8 rodadas: acaba no turno de Tavo, na rodada 11.');
    expect(text).toContain('No fim de cada turno seu: teste de resistência de Sabedoria.');
    expect(text).not.toMatch(/\bCD\b/);
  });

  it('lists what an effect changes in a box of its own under the card', () => {
    const { el } = setup([hold]);
    const box = el.querySelector<HTMLElement>('.fx__changes')!;
    expect(box.getAttribute('role')).toBe('group');
    expect(plain(box.textContent)).toContain('O que isso muda');
    expect(Array.from(box.querySelectorAll('li')).map((l) => plain(l.textContent))).toEqual(
      hold.changes,
    );
    expect(el.querySelector('.fx__item .fx__card')?.nextElementSibling).toBe(box);
  });

  it('has the exhaustion card with the level, who sets it and the lines up to the level', () => {
    const { el } = setup([], 4);
    expect(plain(el.querySelector('h3')?.textContent)).toBe('Exaustão');
    const text = plain(el.textContent);
    expect(text).toContain('Nível 4');
    expect(text).toContain('Definida pelo mestre. Cada nível soma aos de baixo.');
    expect(
      Array.from(el.querySelectorAll('.fx__lines li')).map((l) => plain(l.textContent)),
    ).toEqual([
      'Desvantagem em testes de habilidade',
      'Deslocamento pela metade',
      'Desvantagem em ataques e testes de resistência',
      'PV máximos pela metade',
    ]);
  });

  it('says in a polite live region what came and what went', () => {
    const { fixture, el } = setup([bless]);
    const live = () => plain(el.querySelector('[role="status"]')?.textContent);
    expect(live()).toBe('');
    fixture.componentRef.setInput('cards', [bless, hold]);
    fixture.detectChanges();
    expect(live()).toBe('Novo efeito: Imobilizar Pessoa.');
    fixture.componentRef.setInput('cards', [hold]);
    fixture.detectChanges();
    expect(live()).toBe('Bênção acabou.');
    fixture.componentRef.setInput('exhaustion', 2);
    fixture.detectChanges();
    expect(live()).toBe('Exaustão nível 2.');
    expect(el.querySelector('[role="status"]')?.getAttribute('aria-live')).toBe('polite');
  });
});
