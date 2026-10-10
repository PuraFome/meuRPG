import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { DescriptionLanguage } from '../../core/text/description-language';
import { SpellDetailsVm } from './spell-details.types';
import { SpellDetails, SpellDetailsData } from './spell-details';

const KNOCK: SpellDetailsVm = {
  key: 'spell:knock',
  namePt: 'Arrombar',
  nameEn: 'Knock',
  level: 2,
  schoolNamePt: 'Transmutação',
  ritual: false,
  concentration: false,
  castingTime: { amount: 1, unit: 'action', trigger: '', raw: '1 action' },
  range: { kind: 'ranged', distanceFt: 60, raw: '60 feet' },
  components: { verbal: true, somatic: false, material: false, materialText: '' },
  duration: {
    kind: 'instantaneous',
    amount: 0,
    unit: '',
    upTo: false,
    concentration: false,
    raw: 'Instantaneous',
  },
  description: ['Choose an object that you can see within range.', 'A second paragraph.'],
  higherLevel: [],
};

describe('SpellDetails (E6-22, E6-23)', () => {
  const close = vi.fn();

  function setup(load: () => Promise<SpellDetailsVm>) {
    close.mockReset();
    const data: SpellDetailsData = { namePt: 'Arrombar', load };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close } },
      ],
    });
    const fixture = TestBed.createComponent(SpellDetails);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  const settle = async (fixture: { detectChanges(): void; whenStable(): Promise<unknown> }) => {
    await fixture.whenStable();
    fixture.detectChanges();
  };
  const text = (el: Element) => el.textContent?.replace(/\s+/g, ' ').trim() ?? '';

  it('shows the name and a loading line while the details load', () => {
    const { el } = setup(() => new Promise(() => undefined));
    expect(el.querySelector('h2')?.textContent).toBe('Arrombar');
    expect(el.querySelector('[role=status]')?.textContent).toContain('Carregando');
  });

  it('shows the four fields in Portuguese and, with no translation yet, the SRD text in English with a note', async () => {
    const { fixture, el } = setup(() => Promise.resolve({ ...KNOCK, textPtMissing: true }));
    await settle(fixture);
    expect(text(el)).toContain('Nome no SRD: Knock');
    expect(text(el)).toContain('2º nível · Transmutação');
    const facts = Array.from(el.querySelectorAll('.spell__fact')).map((f) => text(f));
    expect(facts).toEqual([
      'Tempo de conjuração 1 ação',
      'Alcance 18 m',
      'Componentes V',
      'Duração Instantânea',
    ]);
    expect(text(el)).toContain('Texto do SRD 5.1 (em inglês)');
    const prose = el.querySelector('.spell__prose .srd')!;
    expect(prose.getAttribute('lang')).toBe('en');
    expect(prose.querySelectorAll('p').length).toBe(2);
    expect(text(el)).not.toContain('Em níveis superiores');
    expect(el.querySelector('app-description-lang-button')).toBeNull();
  });

  describe('the Portuguese text and the language toggle', () => {
    const TRANSLATED: SpellDetailsVm = {
      ...KNOCK,
      components: {
        verbal: true,
        somatic: true,
        material: true,
        materialText: 'A pinch of salt.',
        materialTextPt: 'Uma pitada de sal.',
      },
      descriptionPt: ['Escolha um objeto que você veja.', 'Um segundo parágrafo.'],
      higherLevel: ['The spell grows.'],
      higherLevelPt: ['A magia cresce.'],
    };

    afterEach(() => {
      if (TestBed.inject(DescriptionLanguage).english()) {
        TestBed.inject(DescriptionLanguage).toggle();
      }
    });

    it('reads Portuguese first, material and higher levels included, with no lang="en"', async () => {
      const { fixture, el } = setup(() => Promise.resolve(TRANSLATED));
      await settle(fixture);
      const paragraphs = Array.from(el.querySelectorAll('.spell__prose .srd')).map((p) => text(p));
      expect(paragraphs).toEqual([
        'Escolha um objeto que você veja.Um segundo parágrafo.',
        'A magia cresce.',
      ]);
      expect(el.querySelector('.spell__prose .srd[lang]')).toBeNull();
      expect(text(el)).toContain('Componentes V, S, M (Uma pitada de sal.)');
      expect(el.querySelector('[data-testid=lang-note]')).toBeNull();
      expect(text(el.querySelector('app-description-lang-button')!)).toBe('Ver em inglês');
    });

    it('"Ver em inglês" flips description, higher levels and material, and back', async () => {
      const { fixture, el } = setup(() => Promise.resolve(TRANSLATED));
      await settle(fixture);
      el.querySelector<HTMLButtonElement>('app-description-lang-button button')!.click();
      await settle(fixture);
      const prose = Array.from(el.querySelectorAll('.spell__prose .srd'));
      expect(prose.map((p) => p.getAttribute('lang'))).toEqual(['en', 'en']);
      expect(text(el)).toContain('Choose an object that you can see within range.');
      expect(text(el)).toContain('The spell grows.');
      expect(text(el)).toContain('(A pinch of salt.)');
      expect(text(el)).toContain('(em inglês)');
      const back = el.querySelector<HTMLButtonElement>('app-description-lang-button button')!;
      expect(text(back)).toBe('Ver em português');
      back.click();
      await settle(fixture);
      expect(text(el)).toContain('Escolha um objeto que você veja.');
    });

    it('a table spell is Portuguese only: no label, no button', async () => {
      const { fixture, el } = setup(() =>
        Promise.resolve({
          ...KNOCK,
          table: true,
          textPtOnly: true,
          descriptionPt: KNOCK.description,
        }),
      );
      await settle(fixture);
      expect(el.querySelector('app-description-lang-button')).toBeNull();
      expect(el.querySelector('#spell-srd-label')).toBeNull();
    });
  });

  it('tags a ritual and a concentration spell, and shows the higher-level text', async () => {
    const { fixture, el } = setup(() =>
      Promise.resolve({
        ...KNOCK,
        ritual: true,
        concentration: true,
        higherLevel: ['The spell grows.'],
      }),
    );
    await settle(fixture);
    const tags = Array.from(el.querySelectorAll('.mr-tag')).map((t) => t.textContent);
    expect(tags).toEqual(['Ritual', 'Concentração']);
    expect(text(el)).toContain('Em níveis superiores');
    expect(text(el)).toContain('The spell grows.');
  });

  it('marks a value that does not map as the SRD text', async () => {
    const { fixture, el } = setup(() =>
      Promise.resolve({ ...KNOCK, range: { kind: 'special', distanceFt: 0, raw: 'Special' } }),
    );
    await settle(fixture);
    const range = el.querySelectorAll('.spell__fact')[1];
    expect(range.querySelector('[lang=en]')?.textContent).toBe('Special');
    expect(text(range)).toContain('(texto do SRD)');
  });

  it('says so when the details cannot load, and tries again', async () => {
    let fail = true;
    const { fixture, el } = setup(() =>
      fail ? Promise.reject(new Error('x')) : Promise.resolve(KNOCK),
    );
    await settle(fixture);
    expect(el.querySelector('[role=alert]')?.textContent).toContain('Não deu para carregar');
    fail = false;
    Array.from(el.querySelectorAll('button'))
      .find((b) => b.textContent?.includes('Tentar de novo'))!
      .click();
    await settle(fixture);
    expect(text(el)).toContain('Alcance 18 m');
    expect(el.querySelector('[role=alert]')).toBeNull();
  });

  it('closes with "Fechar" and with the X', async () => {
    const { fixture, el } = setup(() => Promise.resolve(KNOCK));
    await settle(fixture);
    el.querySelector<HTMLButtonElement>('.spell__close')!.click();
    Array.from(el.querySelectorAll('button'))
      .find((b) => b.textContent?.trim() === 'Fechar')!
      .click();
    expect(close).toHaveBeenCalledTimes(2);
  });

  it('shows "Alvo" after the range when the server names a target (the editor and the combat open this too)', async () => {
    const { fixture, el } = setup(() => Promise.resolve({ ...KNOCK, targetLabel: 'Uma criatura' }));
    await settle(fixture);
    const facts = Array.from(el.querySelectorAll('.spell__fact')).map((f) => text(f));
    expect(facts).toEqual([
      'Tempo de conjuração 1 ação',
      'Alcance 18 m',
      'Alvo Uma criatura',
      'Componentes V',
      'Duração Instantânea',
    ]);
    // Five facts in a two by two grid: the last one takes the whole row (the style that does it is `:last-child:nth-child(odd)`).
    const last = el.querySelector('.spell__fact:last-child')!;
    expect(last.matches(':nth-child(odd)')).toBe(true);
    expect(el.querySelector('.spell__facts--rows')).toBeNull();
  });
});
