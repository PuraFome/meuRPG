import { ComponentFixture, TestBed } from '@angular/core/testing';

import type { GalleryImage } from '../../../../gen/meurpg/maps/v1/gallery_pb';
import { galleryImage, mirathelImages } from '../../../core/images/gallery-testing';
import { GalleryLightbox } from './gallery-lightbox';

describe('GalleryLightbox', () => {
  let fixture: ComponentFixture<GalleryLightbox>;
  let el: HTMLElement;
  let turns: number[];
  let closed: number;
  let renames: GalleryImage[];

  beforeEach(() => {
    TestBed.configureTestingModule({ imports: [GalleryLightbox] });
    fixture = TestBed.createComponent(GalleryLightbox);
    fixture.componentRef.setInput('images', mirathelImages());
    turns = [];
    closed = 0;
    renames = [];
    fixture.componentInstance.indexChange.subscribe((i) => turns.push(i));
    fixture.componentInstance.closed.subscribe(() => closed++);
    fixture.componentInstance.rename.subscribe((i) => renames.push(i));
    el = fixture.nativeElement as HTMLElement;
  });

  function open(index: number): HTMLDialogElement {
    fixture.componentRef.setInput('index', index);
    fixture.detectChanges();
    return el.querySelector('dialog') as HTMLDialogElement;
  }

  function key(dialog: HTMLDialogElement, k: string): void {
    dialog.dispatchEvent(new KeyboardEvent('keydown', { key: k }));
  }

  it('stays closed without an index', () => {
    fixture.detectChanges();
    expect(el.querySelector('dialog')?.hasAttribute('open')).toBe(false);
  });

  it('shows the full image with its name, its size and "N de M", and focuses "Fechar"', () => {
    const dialog = open(1);
    expect(dialog.hasAttribute('open')).toBe(true);
    expect(dialog.getAttribute('aria-labelledby')).toBe('gallery-lightbox-title');
    expect(dialog.querySelector('h2')?.textContent).toBe('Taverna do Javali');
    const img = dialog.querySelector('.lightbox__full') as HTMLImageElement;
    expect(img.getAttribute('src')).toBe('/images/img-taverna');
    expect(img.getAttribute('alt')).toBe('Taverna do Javali');
    // Until the full file loads, the cached thumbnail holds its place,
    // hidden from screen readers.
    const preview = dialog.querySelector('.lightbox__preview') as HTMLImageElement;
    expect(preview.getAttribute('src')).toBe('/images/img-taverna/thumb');
    expect(preview.getAttribute('alt')).toBe('');
    img.dispatchEvent(new Event('load'));
    fixture.detectChanges();
    expect(dialog.querySelector('.lightbox__preview')).toBeNull();
    expect(dialog.textContent).toContain('2 de 5');
    expect(document.activeElement?.getAttribute('aria-label')).toBe('Fechar');
  });

  it('turns with the arrows and wraps at both ends', () => {
    const dialog = open(0);
    key(dialog, 'ArrowRight');
    key(dialog, 'ArrowLeft');
    fixture.componentRef.setInput('index', 4);
    fixture.detectChanges();
    key(dialog, 'ArrowRight');
    expect(turns).toEqual([1, 4, 0]);
  });

  it('hides the page buttons when there is one image', () => {
    fixture.componentRef.setInput('images', mirathelImages().slice(0, 1));
    const dialog = open(0);
    expect(dialog.textContent).not.toContain('Próxima imagem');
  });

  it('hands "Renomear" to the page and closes on "Fechar"', () => {
    const dialog = open(2);
    (
      Array.from(dialog.querySelectorAll('button')).find(
        (b) => b.textContent?.trim() === 'Renomear',
      ) as HTMLButtonElement
    ).click();
    expect(renames.map((i) => i.name)).toEqual(['Capitão Goblin']);

    (dialog.querySelector('[aria-label="Fechar"]') as HTMLButtonElement).click();
    expect(closed).toBe(1);
  });
  it('offers "Pedir um ajuste" only on an image the app generated, and hands it to the page', () => {
    const adjusts: GalleryImage[] = [];
    fixture.componentInstance.adjust.subscribe((i) => adjusts.push(i));
    const plain = open(1);
    expect(plain.textContent).not.toContain('Pedir um ajuste');
    expect(plain.textContent).not.toContain('Gerada por IA');
    fixture.componentRef.setInput('images', [...mirathelImages(), galleryImage('img-gen', 'Imagem 1', { generated: true })]);
    const dialog = open(5);
    expect(dialog.textContent).toContain('Gerada por IA');
    const button = Array.from(dialog.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Pedir um ajuste')!;
    button.click();
    expect(adjusts.map((i) => i.id)).toEqual(['img-gen']);
  });
});
