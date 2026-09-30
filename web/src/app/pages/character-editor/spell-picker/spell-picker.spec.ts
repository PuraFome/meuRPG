import { TestBed } from '@angular/core/testing';

import { SpellOptionVm } from '../character-editor.types';
import { SpellPicker } from './spell-picker';

const SPELLS: SpellOptionVm[] = [
  { key: 'spell:fire-bolt', namePt: 'Raio de Fogo', level: 0, classKeys: ['class:wizard'] },
  { key: 'spell:light', namePt: 'Luz', level: 0, classKeys: ['class:wizard'] },
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
});
