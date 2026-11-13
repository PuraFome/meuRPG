import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import type { EntryRead as Read } from '../../../core/content/content-read-parts';
import { EntryRead } from './entry-read';

const withSide: Read = {
  rows: [],
  sections: [],
  text: ['Escolha um estilo.'],
  options: [
    { title: 'Guarda Alta', text: 'Você mantém a lâmina erguida.', side: 'Classe de Armadura +1' },
    {
      title: 'Mão Firme',
      text: 'Seu golpe cai no mesmo ponto.',
      side: 'Dano de arma corpo a corpo +1',
    },
  ],
};

function render(read: Read, plain = false): HTMLElement {
  const fixture = TestBed.createComponent(EntryRead);
  fixture.componentRef.setInput('read', read);
  fixture.componentRef.setInput('plain', plain);
  fixture.detectChanges();
  return fixture.nativeElement;
}

describe('EntryRead options', () => {
  it('draws one row for each option, the name and what it does, under an "Opções" heading inside the text', () => {
    const el = render(withSide);
    expect(el.querySelector('section[aria-label="Texto"] h2')?.textContent).toBe('Opções');
    const rows = Array.from(el.querySelectorAll('.opts li')).map((li) => [
      li.querySelector('strong')?.textContent,
      li.querySelector('.opts__side')?.textContent,
    ]);
    expect(rows).toEqual([
      ['Guarda Alta', 'Classe de Armadura +1'],
      ['Mão Firme', 'Dano de arma corpo a corpo +1'],
    ]);
    expect(el.textContent).not.toContain('Você mantém a lâmina erguida.');
  });

  it('keeps the full texts in a preview, and for a reader with no words for them (a player)', () => {
    for (const el of [
      render(withSide, true),
      render({ ...withSide, options: withSide.options!.map((o) => ({ ...o, side: undefined })) }),
    ]) {
      expect(el.querySelector('.opts')).toBeNull();
      expect(el.querySelector('ul[aria-label="Opções"]')?.textContent).toContain(
        'Você mantém a lâmina erguida.',
      );
    }
  });
});

describe('EntryRead next-hit effect (#577)', () => {
  const hit: Read = {
    rows: [],
    sections: [],
    text: ['O frio sobe.'],
    hit: {
      title: 'Efeito no próximo golpe',
      where: 'Em quem conjura: o próximo golpe corpo a corpo, por até 1 minuto.',
      lead: 'No próximo golpe corpo a corpo:',
      damage: '+2d8 frio',
      more: '+1d8 por nível acima do 2º, até 5d8',
      save: [
        { text: 'O alvo faz um teste de resistência de Força; se falhar, fica ' },
        { text: 'Impedido', strong: true },
        { text: ' até o fim do próximo turno dele.' },
      ],
      visibility: 'Só quem conjura e você',
    },
  };

  it('draws the section with chips, the sentence and who sees it', () => {
    const el = render(hit);
    expect(el.querySelector('section[aria-labelledby="hit-title"] h2')?.textContent).toBe(
      'Efeito no próximo golpe',
    );
    expect(Array.from(el.querySelectorAll('.hit__chips .chip')).map((c) => c.textContent)).toEqual([
      '+2d8 frio',
      '+1d8 por nível acima do 2º, até 5d8',
    ]);
    expect(el.querySelector('.hit .text strong')?.textContent).toBe('Impedido');
    expect(el.querySelector('.hit__who')?.textContent).toContain('Quem vê o efeito');
    expect(el.querySelector('.hit__who dd')?.textContent).toBe('Só quem conjura e você');
  });

  it('draws one line in a preview, with the damage as a chip and no audience row', () => {
    const el = render(hit, true);
    expect(el.querySelector('.hit-line')?.textContent?.replace(/\s+/g, ' ').trim()).toBe(
      'No próximo golpe corpo a corpo: +2d8 frio, +1d8 por nível acima do 2º, até 5d8. O alvo faz um teste de resistência de Força; se falhar, fica Impedido até o fim do próximo turno dele.',
    );
    expect(el.querySelector('.hit-line .chip')?.textContent).toBe('+2d8 frio');
    expect(el.querySelector('.hit__who')).toBeNull();
  });
});
