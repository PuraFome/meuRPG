import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';
import { MAT_BOTTOM_SHEET_DATA, MatBottomSheetRef } from '@angular/material/bottom-sheet';

import { ListSpellsResponseSchema, SpellSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import { SpellsState } from '../../core/spells/spells-state';
import { SpellFilterSheet, type SpellFilterSheetData } from './spell-filter-sheet';

const flat = (n: Element | null) => n?.textContent?.replace(/\s+/g, ' ').trim() ?? '';

describe('SpellFilterSheet (E10-11, state 3)', () => {
  const requests: Record<string, unknown>[] = [];
  const dismiss = vi.fn();

  async function open() {
    requests.length = 0;
    dismiss.mockReset();
    const state = new SpellsState(
      {
        list: async (req) => {
          requests.push(req as Record<string, unknown>);
          const n = (req as { classKey?: string }).classKey ? 2 : 5;
          return create(ListSpellsResponseSchema, { spells: Array.from({ length: n }, (_, i) => create(SpellSchema, { key: `s${i}` })), total: n });
        },
      },
      'camp-1',
      () => 'char-1',
    );
    await state.change({ query: 'maos' });
    const data: SpellFilterSheetData = {
      state,
      classes: [{ key: 'class:wizard', namePt: 'Mago', archived: false }],
      mine: { id: 'char-1', name: 'Pensantus', label: 'Pensantus, Mago 4' },
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_BOTTOM_SHEET_DATA, useValue: data },
        { provide: MatBottomSheetRef, useValue: { dismiss } },
      ],
    });
    const fixture = TestBed.createComponent(SpellFilterSheet);
    fixture.detectChanges();
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        fixture.detectChanges();
      }
    };
    return { el: fixture.nativeElement as HTMLElement, state, settle };
  }

  it('has the class, the circles, the school and the switch, and the count follows the server', async () => {
    const { el, state, settle } = await open();
    expect(flat(el.querySelector('h2'))).toBe('Filtros');
    expect(el.querySelectorAll('.chip')).toHaveLength(11);
    expect(el.querySelector('[role=switch]')).not.toBeNull();
    expect(flat(el.querySelector('.foot button:last-child'))).toBe('Ver 5 magias');
    const select = el.querySelector<HTMLSelectElement>('select[name=class]')!;
    select.value = 'class:wizard';
    select.dispatchEvent(new Event('change'));
    await settle();
    await state.search();
    await settle();
    expect(requests.at(-1)).toMatchObject({ classKey: 'class:wizard', query: 'maos' });
    expect(flat(el.querySelector('.foot button:last-child'))).toBe('Ver 2 magias');
  });

  it('"Limpar" clears the filters but not the name on the page behind, and "Ver" closes', async () => {
    const { el, state, settle } = await open();
    await state.change({ classKey: 'class:wizard', levels: [1] });
    await settle();
    Array.from(el.querySelectorAll<HTMLButtonElement>('.foot button')).find((b) => flat(b) === 'Limpar')!.click();
    await settle();
    expect(state.filter()).toMatchObject({ query: 'maos', classKey: '', levels: [] });
    el.querySelector<HTMLButtonElement>('.foot button:last-child')!.click();
    expect(dismiss).toHaveBeenCalled();
  });

  it('toggles a circle chip with aria-pressed', async () => {
    const { el, state, settle } = await open();
    const second = el.querySelectorAll<HTMLButtonElement>('.chip')[3];
    second.click();
    await settle();
    expect(state.filter().levels).toEqual([2]);
    expect(el.querySelectorAll('.chip')[3].getAttribute('aria-pressed')).toBe('true');
    expect(el.querySelectorAll('.chip')[0].getAttribute('aria-pressed')).toBe('false');
  });
});
