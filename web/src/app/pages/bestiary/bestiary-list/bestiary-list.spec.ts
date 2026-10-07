import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { of } from 'rxjs';
import { provideRouter, Router } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { Code, ConnectError } from '@connectrpc/connect';

import { CreatureSize } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { type BestiaryAccess, BestiaryAccessCheck } from '../../../core/creatures/bestiary-access';
import { CreaturesClient } from '../../../core/creatures/creatures-client';
import { FakeCreaturesClient, flat, summary } from '../../../core/creatures/creatures-testing';
import { BestiaryList } from './bestiary-list';

@Component({ template: 'stat block' })
class Stub {}

describe('BestiaryList (MR-042, E10-08 states 1, 2 and 7)', () => {
  let api: FakeCreaturesClient;
  let access: BestiaryAccess;
  let dialogResult: unknown;
  let opened: unknown[];

  const catalog = () => [
    summary('monster:wolf', 'Lobo', { name: 'Wolf', sizePt: 'Médio', size: 'Medium', challengeRating: '1/4', armorClass: 13, hitPoints: 11 }),
    summary('monster:giant-wolf-spider', 'Aranha-lobo gigante', { name: 'Giant wolf spider', sizePt: 'Médio', size: 'Medium', challengeRating: '1/4', armorClass: 13, hitPoints: 11 }),
    summary('monster:dire-wolf', 'Lobo atroz', { name: 'Dire wolf', sizePt: 'Grande', size: 'Large', challengeRating: '1', armorClass: 14, hitPoints: 37 }),
    summary('monster:winter-wolf', 'Lobo do inverno', { name: 'Winter wolf', type: 'monstrosity', typePt: 'monstruosidade', sizePt: 'Grande', size: 'Large', challengeRating: '3', armorClass: 13, hitPoints: 75 }),
    summary('monster:ogre', 'Ogro', { name: 'Ogre', type: 'giant', typePt: 'gigante', sizePt: 'Grande', size: 'Large', challengeRating: '2', armorClass: 11, hitPoints: 59 }),
  ];

  async function open(url = '/campanhas/camp-1/bestiario', prep: (a: FakeCreaturesClient) => void = () => undefined) {
    api = new FakeCreaturesClient();
    api.catalog = catalog();
    prep(api);
    TestBed.configureTestingModule({
      providers: [
        provideRouter([
          { path: 'campanhas/:id/bestiario', component: BestiaryList },
          { path: 'campanhas/:id/bestiario/:slug', component: Stub },
        ]),
        { provide: CreaturesClient, useValue: api },
        { provide: BestiaryAccessCheck, useValue: { check: async () => access } },
        {
          provide: MatDialog,
          useValue: {
            open: (_c: unknown, config: unknown) => {
              opened.push(config);
              return { afterClosed: () => of(dialogResult) };
            },
          },
        },
        { provide: MatBottomSheet, useValue: {} },
      ],
    });
    const harness = await RouterTestingHarness.create();
    const component = await harness.navigateByUrl(url, BestiaryList);
    // The clock is the test's: no real waiting, the typing pause is advanced by `debounced`.
    const settle = async () => {
      for (let i = 0; i < 4; i++) {
        harness.detectChanges();
        await harness.fixture.whenStable();
        await vi.advanceTimersByTimeAsync(0);
      }
      harness.detectChanges();
    };
    const debounced = async () => {
      await vi.advanceTimersByTimeAsync(300);
      await settle();
    };
    await settle();
    const el = harness.routeNativeElement as HTMLElement;
    return { el, settle, debounced, component, harness };
  }

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    access = { status: 'master', campaignName: 'Mirathel' };
    dialogResult = undefined;
    opened = [];
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('lists the creatures with the Portuguese name, the SRD\'s in small type, the type and size, the ND, and CA and PV', async () => {
    const { el } = await open();
    expect(flat(el.querySelector('h1'))).toBe('Bestiário');
    expect(flat(el.querySelector('.mr-page-lead'))).toBe('5 criaturas do SRD 5.1 · Mirathel');
    const rows = Array.from(el.querySelectorAll('.row'));
    expect(rows).toHaveLength(5);
    expect(flat(rows[0].querySelector('.row__pt'))).toBe('Lobo');
    expect(flat(rows[0].querySelector('.row__en'))).toBe('Wolf · SRD');
    expect(flat(rows[0].querySelector('.row__kind'))).toBe('Fera · Médio');
    expect(flat(rows[0].querySelector('.row__nd'))).toBe('ND 1/4');
    expect(flat(rows[0].querySelector('.row__stats'))?.replace(/ /g, ' ')).toBe('CA 13 · PV 11');
    expect(rows[0].querySelector('.row__link')?.getAttribute('href')).toBe('/campanhas/camp-1/bestiario/wolf');
    expect(flat(el.querySelector('.list__n'))).toBe('5 de 5 criaturas');
    // The SRD's name is English: marked for a screen reader.
    expect(rows[0].querySelector('.row__en [lang=en]')?.textContent).toBe('Wolf');
  });

  it('searches while typing, asking the server once after a pause, and says what the search matches', async () => {
    const { el, debounced, settle } = await open();
    const q = el.querySelector<HTMLInputElement>('input[type=search]')!;
    q.value = 'lobo';
    q.dispatchEvent(new Event('input'));
    q.value = 'lobo ';
    q.dispatchEvent(new Event('input'));
    await debounced();
    expect(api.searches.map((s) => s.query)).toEqual(['', 'lobo']);
    expect(api.searches[1].pageSize).toBe(400);
    expect(Array.from(el.querySelectorAll('.row__pt')).map((n) => n.textContent)).toEqual(['Lobo', 'Aranha-lobo gigante', 'Lobo atroz', 'Lobo do inverno']);
    expect(flat(el.querySelector('.list__n'))).toBe('4 de 5 criaturas');
    expect(flat(el.querySelector('.list__note'))).toContain('A busca vale para o nome em português e para o nome do SRD, em inglês.');
    expect(el.textContent).toContain('Limpar filtros');
  });

  it('a row opened before the typing pause ends is not pulled back to the list when the pause ends', async () => {
    const { el, settle } = await open();
    const q = el.querySelector<HTMLInputElement>('input[type=search]')!;
    q.value = 'ogro';
    q.dispatchEvent(new Event('input'));
    await settle();
    // The tap: the navigation starts (the list stays on screen until it is done), then the pause ends.
    const navigation = TestBed.inject(Router).navigateByUrl(el.querySelector('.row__link')!.getAttribute('href')!);
    await vi.advanceTimersByTimeAsync(300);
    await navigation;
    await settle();
    expect(TestBed.inject(Router).url).toBe('/campanhas/camp-1/bestiario/wolf?q=ogro');
    // No search was sent for the typed word: the list was already leaving.
    expect(api.searches.map((s) => s.query)).toEqual(['']);
  });

  it('the type, size and ND filters go to the server (an exact ND is both ends, a range its two)', async () => {
    const { el, settle } = await open();
    const select = (name: string, value: string) => {
      const s = el.querySelector<HTMLSelectElement>(`select[name=${name}]`)!;
      s.value = value;
      s.dispatchEvent(new Event('change'));
    };
    select('type', 'giant');
    await settle();
    select('size', 'large');
    await settle();
    select('cr', '1-4');
    await settle();
    const last = api.searches[api.searches.length - 1];
    expect(last).toMatchObject({ type: 'giant', size: CreatureSize.LARGE, minCr: '1', maxCr: '4' });
    expect(Array.from(el.querySelectorAll('.row__pt')).map((n) => n.textContent)).toEqual(['Ogro']);
    select('cr', '1/4');
    await settle();
    expect(api.searches[api.searches.length - 1]).toMatchObject({ minCr: '1/4', maxCr: '1/4' });
    // The filters offer the six sizes and the ND as ranges, then one by one.
    expect(Array.from(el.querySelectorAll('select[name=size] option')).map((o) => o.textContent)).toEqual(['Todos', 'Miúdo', 'Pequeno', 'Médio', 'Grande', 'Enorme', 'Imenso']);
    expect(el.querySelectorAll('select[name=cr] optgroup')).toHaveLength(2);
  });

  it('keeps the search in the link, and a link with a search opens already searched', async () => {
    const { el } = await open('/campanhas/camp-1/bestiario?q=wolf&tipo=beast');
    expect(el.querySelector<HTMLInputElement>('input[type=search]')!.value).toBe('wolf');
    expect(api.searches[0]).toMatchObject({ query: 'wolf', type: 'beast' });
    // The book's size is asked once more, since the search itself does not say it.
    expect(api.searches.some((s) => s.pageSize === 1)).toBe(true);
  });

  it('an empty search says so with the typed word, and "Limpar a busca" brings the whole bestiary back', async () => {
    const { el, debounced, settle } = await open();
    const q = el.querySelector<HTMLInputElement>('input[type=search]')!;
    q.value = 'wyrm';
    q.dispatchEvent(new Event('input'));
    await debounced();
    expect(flat(el.querySelector('.empty__t'))).toBe('Nenhuma criatura com “wyrm”.');
    expect(flat(el.querySelector('.empty__s'))).toBe('Confira a grafia ou procure pelo nome em inglês do SRD. Os filtros de tipo, tamanho e ND também contam.');
    expect(flat(el.querySelector('.list__n'))).toBe('0 de 5 criaturas');
    expect(el.querySelector('.list__n')?.getAttribute('role')).toBe('status');
    const clear = Array.from(el.querySelectorAll('button')).find((b) => flat(b) === 'Limpar a busca')!;
    clear.click();
    await settle();
    expect(el.querySelectorAll('.row')).toHaveLength(5);
    expect(q.value).toBe('');
    // The button is gone: the focus is on the search field.
    expect(document.activeElement).toBe(q);
  });

  it('empty by the filters alone names the filters, not a word', async () => {
    const { el, debounced, settle } = await open();
    const s = el.querySelector<HTMLSelectElement>('select[name=type]')!;
    s.value = 'undead';
    s.dispatchEvent(new Event('change'));
    await settle();
    expect(flat(el.querySelector('.empty__t'))).toBe('Nenhuma criatura passa nesses filtros.');
    expect(flat(el.querySelector('.empty button'))).toBe('Limpar filtros');
  });

  it('while the first answer has not come it says it is loading, and the count waits', async () => {
    api = new FakeCreaturesClient();
    let release!: () => void;
    const held = new Promise<void>((r) => (release = r));
    TestBed.configureTestingModule({
      providers: [
        provideRouter([{ path: 'campanhas/:id/bestiario', component: BestiaryList }]),
        { provide: CreaturesClient, useValue: { search: async () => { await held; return { creatures: [], total: 0 }; } } },
        { provide: BestiaryAccessCheck, useValue: { check: async () => access } },
      ],
    });
    const harness = await RouterTestingHarness.create();
    await harness.navigateByUrl('/campanhas/camp-1/bestiario', BestiaryList);
    await vi.advanceTimersByTimeAsync(0);
    harness.detectChanges();
    const el = harness.routeNativeElement as HTMLElement;
    expect(flat(el.querySelector('.list__wait'))).toBe('Buscando as criaturas...');
    expect(flat(el.querySelector('.list__n'))).toBe('');
    release();
  });

  it('a failed search says what to do, by code, and "Tentar de novo" asks again', async () => {
    const { el, debounced, settle } = await open();
    api.searchFail = new ConnectError('down', Code.Unavailable);
    const q = el.querySelector<HTMLInputElement>('input[type=search]')!;
    q.value = 'lobo';
    q.dispatchEvent(new Event('input'));
    await debounced();
    expect(el.querySelector('[role=alert]')?.textContent).toContain('Não deu para abrir o bestiário: o servidor não respondeu.');
    api.searchFail = null;
    Array.from(el.querySelectorAll('button')).find((b) => flat(b) === 'Tentar de novo')!.click();
    await settle();
    expect(el.querySelectorAll('.row')).toHaveLength(4);
  });

  it('a player gets no bestiary: the page says only the master uses it and draws no list (the first search ran in parallel with the role)', async () => {
    access = { status: 'forbidden' };
    const { el } = await open();
    expect(flat(el.querySelector('h1'))).toBe('Bestiário');
    expect(flat(el.querySelector('.mr-notice'))).toBe('Só o mestre usa o bestiário da campanha.');
    expect(el.querySelector('input')).toBeNull();
    expect(el.querySelector('.row')).toBeNull();
  });

  it('the first search does not wait for the role check', async () => {
    api = new FakeCreaturesClient();
    api.catalog = catalog();
    let grant!: (a: BestiaryAccess) => void;
    const role = new Promise<BestiaryAccess>((r) => (grant = r));
    TestBed.configureTestingModule({
      providers: [
        provideRouter([{ path: 'campanhas/:id/bestiario', component: BestiaryList }]),
        { provide: CreaturesClient, useValue: api },
        { provide: BestiaryAccessCheck, useValue: { check: () => role } },
      ],
    });
    const harness = await RouterTestingHarness.create();
    await harness.navigateByUrl('/campanhas/camp-1/bestiario', BestiaryList);
    await vi.advanceTimersByTimeAsync(0);
    expect(api.searches).toHaveLength(1);
    grant({ status: 'master', campaignName: 'Mirathel' });
    await vi.advanceTimersByTimeAsync(0);
    harness.detectChanges();
    expect(harness.routeNativeElement?.querySelectorAll('.row')).toHaveLength(5);
  });

  it('the rows\' links carry the typed search at once, before the pause ends, so a quick click keeps it', async () => {
    const { el, settle } = await open();
    const q = el.querySelector<HTMLInputElement>('input[type=search]')!;
    q.value = 'ogro';
    q.dispatchEvent(new Event('input'));
    await settle();
    expect(el.querySelector('.row__link')?.getAttribute('href')).toBe('/campanhas/camp-1/bestiario/wolf?q=ogro');
  });

  it('with a search in the link the count waits for the book\'s size: nothing, then "N de M"; and says only "N criaturas" if the size cannot be had', async () => {
    const ok = await open('/campanhas/camp-1/bestiario?q=lobo');
    expect(flat(ok.el.querySelector('.list__n'))).toBe('4 de 5 criaturas');
    TestBed.resetTestingModule();
    const failing = await open('/campanhas/camp-1/bestiario?q=lobo', (a) => {
      const real = a.search.getMockImplementation()!;
      a.search.mockImplementation(async (c, f) => {
        if (f.pageSize === 1) {
          throw new Error('down');
        }
        return real(c, f);
      });
    });
    expect(flat(failing.el.querySelector('.list__n'))).toBe('4 criaturas');
  });

  it('a campaign that is not there says so, like the other pages', async () => {
    access = { status: 'not-found' };
    const { el } = await open();
    expect(flat(el.querySelector('h1'))).toBe('Campanha não encontrada');
  });

  it('every row has "Pôr no combate", named by its creature, beside the link that opens the stat block (E10-08 states 1 and 7)', async () => {
    const { el } = await open();
    const buttons = Array.from(el.querySelectorAll<HTMLButtonElement>('.row__put'));
    expect(buttons).toHaveLength(5);
    expect(flat(buttons[0])).toBe('Pôr no combate');
    expect(buttons[0].getAttribute('aria-label')).toBe('Pôr no combate: Lobo');
    // The button is not inside the link: the two are never nested.
    expect(buttons.every((b) => b.closest('a') === null)).toBe(true);
    expect(el.querySelectorAll('a.row__link')).toHaveLength(5);
  });

  it('"Pôr no combate" opens the sheet for that row\'s creature, and what went in is announced above the list', async () => {
    dialogResult = { count: 3, names: 'Lobo 1, Lobo 2 e Lobo 3', combatName: 'Emboscada na ponte', started: false, hidden: true, encounterId: 'enc-1' };
    const { el, settle } = await open();
    el.querySelectorAll<HTMLButtonElement>('.row__put')[0].click();
    await settle();
    expect(opened).toHaveLength(1);
    expect((opened[0] as { data: { campaignId: string; creature: { key: string } } }).data).toMatchObject({ campaignId: 'camp-1', creature: { key: 'monster:wolf' } });
    const done = el.querySelector('.put-done')!;
    expect(done.getAttribute('role')).toBe('status');
    expect(flat(done.querySelector('p'))).toBe('Entraram no combate: Lobo 1, Lobo 2 e Lobo 3. Combate “Emboscada na ponte”. Estão escondidos: só você os vê até revelar.');
    expect(done.querySelector('a')?.getAttribute('href')).toBe('/campanhas/camp-1/sessao');
  });

  it('a sheet closed with nothing put in announces nothing', async () => {
    const { el, settle } = await open();
    el.querySelectorAll<HTMLButtonElement>('.row__put')[1].click();
    await settle();
    expect(opened).toHaveLength(1);
    expect(el.querySelector('.put-done')).toBeNull();
  });

  it('the back link goes to the campaign', async () => {
    const { el } = await open();
    expect(el.querySelector('.back')?.getAttribute('href')).toBe('/campanhas/camp-1');
  });
});
