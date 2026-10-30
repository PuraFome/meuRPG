import { TestBed } from '@angular/core/testing';
import { vi } from 'vitest';

import type { DarkCharacter } from '../../core/maps/in-the-dark';
import { settle } from '../../../test-settle';
import { DarkWarning } from './dark-warning';

const brisa: DarkCharacter = { characterId: 'c-brisa', name: 'Brisa', col: 3, row: 4 };
const jabuti: DarkCharacter = { characterId: 'c-jabuti', name: 'Jabuti', col: 9, row: 4 };

const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim();

describe('DarkWarning', () => {
  async function render(
    characters: readonly DarkCharacter[],
    inputs: { announcement?: string; compact?: boolean; canLight?: boolean } = {},
  ) {
    const fixture = TestBed.createComponent(DarkWarning);
    fixture.componentRef.setInput('characters', characters);
    fixture.componentRef.setInput('announcement', inputs.announcement ?? '');
    fixture.componentRef.setInput('compact', inputs.compact ?? false);
    fixture.componentRef.setInput('canLight', inputs.canLight ?? true);
    const shown = vi.fn();
    const lit = vi.fn();
    fixture.componentInstance.show.subscribe(shown);
    fixture.componentInstance.light.subscribe(lit);
    await settle(fixture);
    const el = fixture.nativeElement as HTMLElement;
    const button = (name: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).filter((b) =>
        flat(b)?.includes(name),
      );
    return { fixture, el, shown, lit, button };
  }

  it('draws nothing for nobody: no empty notice, no seal', async () => {
    const { el } = await render([]);
    expect(el.querySelector('.dark')).toBeNull();
    expect(el.querySelector('button')).toBeNull();
  });

  it('one character: the amber notice with its two buttons', async () => {
    const { el, shown, lit, button } = await render([brisa]);
    expect(flat(el.querySelector('.dark'))).toContain(
      'No escuro: Brisa não enxerga onde está (sem luz e sem visão no escuro).',
    );
    button('Ver no mapa')[0].click();
    button('Acender luz')[0].click();
    expect(shown).toHaveBeenCalledWith('c-brisa');
    expect(lit).toHaveBeenCalledWith('c-brisa');
    expect(el.querySelectorAll('li')).toHaveLength(0);
  });

  it('several: the count, the cause once, and a pair of buttons per character', async () => {
    const { el, shown, lit, button } = await render([brisa, jabuti]);
    expect(flat(el.querySelector('.dark__title'))).toBe(
      'No escuro: 2 personagens não enxergam onde estão.',
    );
    expect(flat(el.querySelector('.dark__cause'))).toBe('Sem luz e sem visão no escuro.');
    const rows = Array.from(el.querySelectorAll('li'));
    expect(rows.map((r) => flat(r.querySelector('.dark__name')))).toEqual(['Brisa', 'Jabuti']);
    button('Ver no mapa')[1].click();
    button('Acender luz')[1].click();
    expect(shown).toHaveBeenCalledWith('c-jabuti');
    expect(lit).toHaveBeenCalledWith('c-jabuti');
  });

  it('names whom each button is for, to a screen reader', async () => {
    const { el } = await render([brisa, jabuti]);
    const labels = Array.from(el.querySelectorAll('button')).map((b) =>
      b.getAttribute('aria-label'),
    );
    expect(labels).toEqual([
      'Ver no mapa, Brisa',
      'Acender luz de Brisa',
      'Ver no mapa, Jabuti',
      'Acender luz de Jabuti',
    ]);
  });

  it('without a quick light control, only "Ver no mapa"', async () => {
    const { button } = await render([brisa], { canLight: false });
    expect(button('Ver no mapa')).toHaveLength(1);
    expect(button('Acender luz')).toHaveLength(0);
  });

  it('the one-line form for the painting mode: names and one "Ver no mapa"', async () => {
    const { el, shown, button } = await render([brisa, jabuti], { compact: true });
    expect(flat(el.querySelector('.dark'))).toContain('No escuro: Brisa, Jabuti.');
    expect(el.querySelectorAll('li')).toHaveLength(0);
    expect(button('Acender luz')).toHaveLength(0);
    button('Ver no mapa')[0].click();
    expect(shown).toHaveBeenCalledWith('c-brisa');
  });

  it('says the live message in a polite status region that is there before the message', async () => {
    const { fixture, el } = await render([]);
    const region = el.querySelector('[role="status"]');
    expect(region?.getAttribute('aria-live')).toBe('polite');
    expect(flat(region)).toBe('');
    fixture.componentRef.setInput(
      'announcement',
      'A tocha de bilbo apagou. A luz de bilbo ficou “Nenhuma”.',
    );
    await settle(fixture);
    expect(flat(el.querySelector('[role="status"] span'))).toBe(
      'A tocha de bilbo apagou. A luz de bilbo ficou “Nenhuma”.',
    );
  });

  it('keeps the message beside the notice when someone is still in the dark', async () => {
    const { el } = await render([brisa], { announcement: 'A tocha de bilbo apagou.' });
    expect(flat(el.querySelector('[role="status"] span'))).toBe('A tocha de bilbo apagou.');
    expect(el.querySelector('.dark')).not.toBeNull();
  });
});
