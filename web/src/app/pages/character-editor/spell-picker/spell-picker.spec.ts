import { TestBed } from '@angular/core/testing';

import { SpellOptionVm } from '../character-editor.types';
import { SpellPicker } from './spell-picker';

const SPELLS: SpellOptionVm[] = [
  { key: 'spell:fire-bolt', namePt: 'Raio de Fogo', level: 0, classKeys: ['class:wizard'], fromTable: false, archived: false, off: false },
  { key: 'spell:light', namePt: 'Luz', level: 0, classKeys: ['class:wizard'], fromTable: false, archived: false, off: false },
];

describe('SpellPicker', () => {
  function render(inputs: { filtered?: SpellOptionVm[]; selected?: string[]; filter?: string }) {
    TestBed.configureTestingModule({ imports: [SpellPicker] });
    const fixture = TestBed.createComponent(SpellPicker);
    fixture.componentRef.setInput('titleId', 'cantrips-label');
    fixture.componentRef.setInput('title', 'Truques');
    fixture.componentRef.setInput('searchLabel', 'Buscar truque');
    fixture.componentRef.setInput('noun', 'truque');
    fixture.componentRef.setInput('options', SPELLS);
    fixture.componentRef.setInput('filtered', inputs.filtered ?? SPELLS);
    fixture.componentRef.setInput('selected', new Set(inputs.selected ?? []));
    fixture.componentRef.setInput('filter', inputs.filter ?? '');
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('says how many are chosen and which, from the whole list', () => {
    // "Luz" is chosen but filtered out of view: it still counts.
    const { el } = render({ filtered: [SPELLS[0]], selected: ['spell:light'], filter: 'raio' });

    expect(el.querySelector('.picker__chosen')?.textContent).toBe('1 truque escolhido: Luz');
    expect(el.querySelector('h3')?.id).toBe('cantrips-label');
    expect(el.querySelector('[role="group"]')?.getAttribute('aria-labelledby')).toBe(
      'cantrips-label',
    );
  });

  it('names the search box by aria-label, with the words inside it and no floating label', () => {
    const { el } = render({});
    const input = el.querySelector<HTMLInputElement>('input[type="search"]')!;

    expect(input.getAttribute('aria-label')).toBe('Buscar truque');
    expect(input.placeholder).toBe('Buscar truque');
    // A floating label is placed by measuring the icon, which can run before
    // the label exists and leave the words over the magnifier (PR #56).
    expect(el.querySelector('mat-label, .mdc-floating-label')).toBeNull();
  });

  it('tells how to get the list back when the search finds nothing', () => {
    const { el } = render({ filtered: [], filter: 'xyz' });

    expect(el.textContent).toContain(
      'Nenhum truque com esse nome. Apague a busca para ver a lista toda.',
    );
  });

  it('says the class has no cantrips when there is no search', () => {
    const { el } = render({ filtered: [] });

    expect(el.textContent).toContain('Nenhum truque encontrado para a classe escolhida.');
    expect(el.textContent).toContain('Nenhum truque escolhido');
  });

  it('has a "?" after each spell that asks for its description', () => {
    const { fixture, el } = render({});
    const asked: string[] = [];
    fixture.componentInstance.describe.subscribe((spell) => asked.push(spell.key));
    const buttons = Array.from(el.querySelectorAll<HTMLButtonElement>('.picker__help'));
    expect(buttons.map((b) => b.getAttribute('aria-label'))).toEqual([
      'Descrição de Raio de Fogo',
      'Descrição de Luz',
    ]);
    buttons[1].click();
    expect(asked).toEqual(['spell:light']);
  });

  it('does not tick the spell when its "?" is pressed', () => {
    const { fixture, el } = render({});
    const ticked: string[] = [];
    fixture.componentInstance.toggle.subscribe((key) => ticked.push(key));
    el.querySelector<HTMLButtonElement>('.picker__help')!.click();
    expect(ticked).toEqual([]);
  });

  it('tags the master\'s own spell "Da mesa", and a retired one with its word', () => {
    const ink: SpellOptionVm = { key: 'spell:ink@mesa', namePt: 'Lâmina de Nanquim', level: 0, classKeys: ['class:wizard'], fromTable: true, archived: true, off: false };
    const { el } = render({ filtered: [SPELLS[0], ink] });
    const rows = Array.from(el.querySelectorAll('.picker__row'));
    expect(rows[0].querySelector('.mr-tag')).toBeNull();
    expect(rows[1].textContent).toContain('Da mesa');
    expect(rows[1].textContent).toContain('Arquivada');
  });

  it('says a spell outside the lists without a class the catalog cannot name: never "é de ." nor a key', () => {
    const { fixture, el } = render({});
    fixture.componentRef.setInput('outside', [{ spell: SPELLS[0], classes: '' }]);
    fixture.detectChanges();
    const text = (el.querySelector('.picker__out')?.textContent ?? '').replace(/\s+/g, ' ');
    expect(text).toContain('Fora da lista das suas classes');
    expect(text).toContain('Você a lê em “Magias”, mas não a escolhe nesta ficha.');
    expect(text).not.toContain('Esta magia é de');
  });
});
