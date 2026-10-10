import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { create } from '@bufbuild/protobuf';

import { MapPointKind, MapRefSchema } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { GalleryClient } from '../../../core/images/gallery-client';
import {
  FakeGalleryClient,
  galleryImage,
  galleryUsage,
} from '../../../core/images/gallery-testing';
import { MapsClient } from '../../../core/maps/maps-client';
import { FakeMapsClient, mapPoint } from '../../../core/maps/maps-testing';
import { PointPanel } from './point-panel';

describe('PointPanel', () => {
  let fixture: ComponentFixture<PointPanel>;
  let el: HTMLElement;
  let dirty: boolean[];

  beforeEach(() => {
    fixture = TestBed.createComponent(PointPanel);
    fixture.componentRef.setInput(
      'point',
      mapPoint('p1', 'Taverna do Javali', {
        description: 'Onde a Velha Odra conta o que sabe.',
        revealed: false,
      }),
    );
    fixture.componentRef.setInput('maps', [{ id: 'm2', name: 'Torre de Mirathel' }]);
    dirty = [];
    fixture.componentInstance.dirtyChange.subscribe((d) => dirty.push(d));
    fixture.detectChanges();
    el = fixture.nativeElement;
  });

  function type(input: HTMLInputElement | HTMLTextAreaElement, value: string): void {
    input.value = value;
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  }

  it('shows the selected point: its name, kind, text and state', () => {
    expect(el.querySelector('.pp__title')?.textContent).toBe('Taverna do Javali');
    expect((el.querySelector('input') as HTMLInputElement).value).toBe('Taverna do Javali');
    expect(el.querySelector('textarea')?.value).toBe('Onde a Velha Odra conta o que sabe.');
    const kinds = Array.from(el.querySelectorAll('[role="radio"]'));
    expect(kinds.map((k) => k.getAttribute('aria-checked'))).toEqual(['false', 'false', 'true']);
    expect(el.querySelector('[role="switch"]')?.getAttribute('aria-checked')).toBe('false');
  });

  it('keeps "Salvar ponto" off until something changes, then sends only that', () => {
    const save = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Salvar ponto'),
    ) as HTMLButtonElement;
    expect(save.disabled).toBe(true);
    expect(fixture.componentInstance.changes()).toBeNull();
    type(el.querySelector('input') as HTMLInputElement, 'Taverna nova');
    expect(save.disabled).toBe(false);
    expect(dirty.at(-1)).toBe(true);
    expect(fixture.componentInstance.changes()).toEqual({ name: 'Taverna nova' });
  });

  it('shows "Leva para" only for a Submapa, and offers the other maps', () => {
    expect(el.querySelector('select')).toBeNull();
    (el.querySelectorAll('[role="radio"]')[1] as HTMLElement).click();
    fixture.detectChanges();
    const select = el.querySelector('select') as HTMLSelectElement;
    expect(Array.from(select.options).map((o) => o.textContent?.trim())).toEqual([
      'Nenhum mapa',
      'Torre de Mirathel',
    ]);
    expect(fixture.componentInstance.changes()).toEqual({ kind: MapPointKind.SUBMAP });
  });

  it('asks for a name when it is empty, and sends nothing', () => {
    type(el.querySelector('input') as HTMLInputElement, '  ');
    expect(fixture.componentInstance.changes()).toBeNull();
    fixture.detectChanges();
    expect(el.querySelector('mat-error')?.textContent).toContain('Dê um nome ao ponto.');
  });

  it('confirms a delete in place, naming the point', () => {
    (
      Array.from(el.querySelectorAll('button')).find((b) =>
        b.textContent?.includes('Apagar ponto'),
      ) as HTMLButtonElement
    ).click();
    fixture.detectChanges();
    expect(el.querySelector('.pp__confirm')?.textContent).toContain(
      'Apagar Taverna do Javali? Não dá para desfazer.',
    );
    const removed: number[] = [];
    fixture.componentInstance.removeConfirmed.subscribe(() => removed.push(1));
    (el.querySelector('.pp__confirm button') as HTMLButtonElement).click();
    expect(removed).toHaveLength(1);
  });

  it('discards the draft', () => {
    type(el.querySelector('input') as HTMLInputElement, 'Outra');
    fixture.componentInstance.discard();
    fixture.detectChanges();
    expect((el.querySelector('input') as HTMLInputElement).value).toBe('Taverna do Javali');
  });

  describe('a saved Cena (MR-029)', () => {
    let api: FakeMapsClient;
    let cena: ComponentFixture<PointPanel>;
    let cenaEl: HTMLElement;

    beforeEach(() => {
      api = new FakeMapsClient();
      TestBed.resetTestingModule();
      TestBed.configureTestingModule({ providers: [{ provide: MapsClient, useValue: api }] });
      cena = TestBed.createComponent(PointPanel);
      cena.componentRef.setInput(
        'point',
        mapPoint('p1', 'A carroça tombada', { hooks: 'O mercador Aldo foi levado.' }),
      );
      cena.componentRef.setInput('campaignId', 'c1');
      cena.detectChanges();
      cenaEl = cena.nativeElement;
    });

    it('has "Pistas" and "Ganchos e anotações" after the actions, with the lock and "Só você vê" first', () => {
      const titles = Array.from(cenaEl.querySelectorAll('h3'), (h) => h.textContent?.trim());
      expect(titles).toEqual(['Ações da cena', 'Pistas', 'Ganchos e anotações', 'Imagens da cena']);
      expect(cenaEl.querySelector('.hf__lock')?.textContent).toContain(
        'Só você vê. Nunca aparece para os jogadores.',
      );
      expect(cenaEl.querySelector('.hf textarea')?.getAttribute('aria-labelledby')).toBe(
        'hf-title',
      );
    });

    it('saves "Mostrar a CD aos jogadores" at once with show_dc alone, and the unsaved name survives', async () => {
      const name = cenaEl.querySelector('input') as HTMLInputElement;
      name.value = 'Carroça nova';
      name.dispatchEvent(new Event('input'));
      cena.detectChanges();
      const saved: boolean[] = [];
      cena.componentInstance.showDcSaved.subscribe((on) => saved.push(on));
      (cenaEl.querySelector('.sa__dcswitch [role="switch"]') as HTMLElement).click();
      for (let i = 0; i < 3; i++) {
        await cena.whenStable();
        cena.detectChanges();
      }
      expect(api.calls).toEqual(['updatePoint map-1 p1 {"showDc":true}']);
      expect(saved).toEqual([true]);
      // The page puts the saved flag on the point: the draft is not reset, and "Salvar ponto" sends only the name.
      cena.componentRef.setInput(
        'point',
        mapPoint('p1', 'A carroça tombada', { hooks: 'O mercador Aldo foi levado.', showDc: true }),
      );
      cena.detectChanges();
      expect((cenaEl.querySelector('input') as HTMLInputElement).value).toBe('Carroça nova');
      expect(cena.componentInstance.changes()).toEqual({ name: 'Carroça nova' });
      expect(
        cenaEl.querySelector('.sa__dcswitch [role="switch"]')?.getAttribute('aria-checked'),
      ).toBe('true');
    });

    it('carries the "É ficção" notice once', () => {
      expect(cenaEl.querySelectorAll('app-fiction-notice')).toHaveLength(1);
    });

    it('waits for "Salvar ponto" for the hooks, and sends only them', () => {
      const field = cenaEl.querySelector('.hf textarea') as HTMLTextAreaElement;
      expect(field.value).toBe('O mercador Aldo foi levado.');
      expect(cena.componentInstance.changes()).toBeNull();
      field.value = 'Mira está escondida debaixo da carroça.';
      field.dispatchEvent(new Event('input'));
      cena.detectChanges();
      expect(cena.componentInstance.changes()).toEqual({
        hooks: 'Mira está escondida debaixo da carroça.',
      });
      expect(
        cenaEl.querySelector('.hf mat-hint[align="end"], .hf .mat-mdc-form-field-hint-wrapper')
          ?.textContent,
      ).toContain('39 de 4.000');
      expect(api.calls).toEqual([]);
    });

    it('says a text over 4.000 characters under the field and keeps it', () => {
      const field = cenaEl.querySelector('.hf textarea') as HTMLTextAreaElement;
      field.value = 'x'.repeat(4001);
      field.dispatchEvent(new Event('input'));
      cena.detectChanges();
      expect(cena.componentInstance.changes()).toBeNull();
      cena.detectChanges();
      expect(cenaEl.querySelector('.hf mat-error')?.textContent).toContain(
        'Use até 4.000 caracteres.',
      );
    });
  });
});

describe('PointPanel: a battle point and its encounter (MR-043)', () => {
  function panel(kind: MapPointKind, over: { campaignId?: string; mapId?: string } = {}) {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(PointPanel);
    fixture.componentRef.setInput('point', mapPoint('pt-1', 'Emboscada na ponte', { kind }));
    fixture.componentRef.setInput('campaignId', over.campaignId ?? 'camp-1');
    fixture.componentRef.setInput('mapId', over.mapId ?? 'map-1');
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('a battle point offers "Montar o encontro deste ponto", which opens the builder on that map and point', () => {
    const el = panel(MapPointKind.BATTLE);
    const link = el.querySelector<HTMLAnchorElement>('.pp__enc')!;
    expect(link.textContent?.replace('swords', '').trim()).toBe('Montar o encontro deste ponto');
    expect(link.getAttribute('href')).toBe('/campaigns/camp-1/encounters?map=map-1&point=pt-1');
  });

  it('another kind of point has no such link', () => {
    expect(panel(MapPointKind.SCENE).querySelector('.pp__enc')).toBeNull();
  });
});

describe('PointPanel: a Submapa and its map (open and create)', () => {
  let api: FakeMapsClient;

  function panel(over: { target?: string; maps?: { id: string; name: string }[] } = {}) {
    api = new FakeMapsClient();
    const gallery = new FakeGalleryClient();
    gallery.listResult = Promise.resolve({
      images: [galleryImage('img-torre', 'Planta da torre')],
      usage: galleryUsage(),
    });
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: MapsClient, useValue: api },
        { provide: GalleryClient, useValue: gallery },
      ],
    });
    const fixture = TestBed.createComponent(PointPanel);
    fixture.componentRef.setInput(
      'point',
      mapPoint('pt-1', 'Torre de Mirathel', {
        kind: MapPointKind.SUBMAP,
        targetMap: over.target ? create(MapRefSchema, { id: over.target, name: 'x' }) : undefined,
      }),
    );
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('maps', over.maps ?? [{ id: 'm2', name: 'Interior da torre' }]);
    fixture.detectChanges();
    return fixture;
  }

  const select = (el: HTMLElement) => el.querySelector('select') as HTMLSelectElement;
  const button = (el: HTMLElement, text: string) =>
    Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes(text),
    ) as HTMLButtonElement;

  function choose(fixture: ComponentFixture<PointPanel>, value: string) {
    const s = select(fixture.nativeElement);
    s.value = value;
    s.dispatchEvent(new Event('change'));
    fixture.detectChanges();
  }

  it('offers no "Abrir" until a map is chosen, then links to that map\'s editor', () => {
    const fixture = panel();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('.pp__open')).toBeNull();
    choose(fixture, 'm2');
    const link = el.querySelector<HTMLAnchorElement>('.pp__open')!;
    expect(link.textContent).toContain('Abrir Interior da torre');
    expect(link.getAttribute('href')).toBe('/campaigns/camp-1/maps/m2');
  });

  it('lists "Criar mapa novo…" last and opens the form named after the point', () => {
    const fixture = panel();
    const el = fixture.nativeElement as HTMLElement;
    const labels = Array.from(select(el).options).map((o) => o.textContent?.trim());
    expect(labels.at(-1)).toBe('Criar mapa novo…');
    choose(fixture, '__new__');
    expect(select(el).value).toBe('');
    expect((el.querySelector('.pp__new input') as HTMLInputElement).value).toBe(
      'Torre de Mirathel',
    );
  });

  it('asks for an image before creating', async () => {
    const fixture = panel();
    const el = fixture.nativeElement as HTMLElement;
    choose(fixture, '__new__');
    button(el, 'Criar mapa e usar').click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.textContent).toContain('Escolha uma imagem para o mapa.');
    expect(api.calls.some((c) => c.startsWith('create'))).toBe(false);
  });

  it('creates the map, points the draft at it and asks the page to save', async () => {
    const fixture = panel();
    const el = fixture.nativeElement as HTMLElement;
    const events: string[] = [];
    fixture.componentInstance.saveRequested.subscribe(() => events.push('save'));
    fixture.componentInstance.mapCreated.subscribe((m) => events.push(`made:${m.id}`));
    choose(fixture, '__new__');
    await fixture.whenStable();
    fixture.detectChanges();
    (el.querySelector('.pp__new [role="radio"]') as HTMLElement).click();
    fixture.detectChanges();
    button(el, 'Criar mapa e usar').click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(api.calls).toContain('create Torre de Mirathel img-torre');
    expect(events).toEqual(['made:new-map', 'save']);
    expect(fixture.componentInstance.changes()).toEqual({ targetMapId: 'new-map' });
    expect(el.querySelector('.pp__new')).toBeNull();
    expect(el.querySelector('.pp__open')?.textContent).toContain('Abrir Torre de Mirathel');
  });

  it('says why a create failed and keeps the form', async () => {
    const fixture = panel();
    const el = fixture.nativeElement as HTMLElement;
    api.create = async () => {
      const { ConnectError, Code } = await import('@connectrpc/connect');
      throw new ConnectError('too many', Code.ResourceExhausted);
    };
    choose(fixture, '__new__');
    await fixture.whenStable();
    fixture.detectChanges();
    (el.querySelector('.pp__new [role="radio"]') as HTMLElement).click();
    fixture.detectChanges();
    button(el, 'Criar mapa e usar').click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('.pp__new [role="alert"]')?.textContent).toContain(
      'limite de 200 mapas',
    );
    expect(el.querySelector('.pp__new')).not.toBeNull();
  });
});
