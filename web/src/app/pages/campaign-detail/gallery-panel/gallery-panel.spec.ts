import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { GalleryClient } from '../../../core/images/gallery-client';
import {
  FakeGalleryClient,
  galleryImage,
  galleryUsage,
  mirathelImages,
  plain,
} from '../../../core/images/gallery-testing';
import { GalleryPanel } from './gallery-panel';
import { nextTurn } from '../../../../test-settle';

describe('GalleryPanel', () => {
  let gallery: FakeGalleryClient;

  beforeEach(() => {
    TestBed.configureTestingModule({
      imports: [GalleryPanel],
      providers: [provideRouter([]), { provide: GalleryClient, useClass: FakeGalleryClient }],
    });
    gallery = TestBed.inject(GalleryClient) as unknown as FakeGalleryClient;
  });

  async function render(): Promise<HTMLElement> {
    const fixture = TestBed.createComponent(GalleryPanel);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.detectChanges();
    await nextTurn();
    await fixture.whenStable();
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('shows the six newest thumbnails, the count and "Ver todas as N imagens" (board C6-02)', async () => {
    const images = [galleryImage('img-ruinas', 'Ruínas élficas'), ...mirathelImages()];
    gallery.listResult = Promise.resolve({ images, usage: galleryUsage(images) });
    const el = await render();

    const thumbs = Array.from(el.querySelectorAll('img'));
    expect(thumbs.map((i) => i.getAttribute('alt'))).toEqual([
      'Ruínas élficas',
      'Covil dos goblins',
      'Taverna do Javali',
      'Capitão Goblin',
      'Planta da torre',
      'Mapa de Mirathel',
    ]);
    expect(thumbs[0].getAttribute('src')).toBe('/images/img-ruinas/thumb');
    expect(plain(el.querySelector('app-panel-head')?.textContent)).toContain('6 de 300 imagens');
    const links = Array.from(el.querySelectorAll('a'));
    expect(links.map((a) => plain(a.textContent).replace(/^\w+ /, '').trim())).toEqual([
      'Enviar imagem',
      'Ver todas as 6 imagens',
    ]);
    expect(links[1].getAttribute('href')).toBe('/campaigns/camp-1/gallery');
  });

  it('invites the first upload when the gallery is empty', async () => {
    const el = await render();
    expect(el.textContent).toContain('Nenhuma imagem ainda.');
    expect(el.querySelector('img')).toBeNull();
    expect(el.textContent).toContain('Enviar imagem');
    expect(el.textContent).not.toContain('Ver todas');
  });
});
