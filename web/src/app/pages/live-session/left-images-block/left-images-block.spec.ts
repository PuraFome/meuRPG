import { TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { ShownImageVm } from '../../../core/play/live-session.types';
import { LeftImagesBlock } from './left-images-block';

const tower: ShownImageVm = {
  id: 'img-2',
  name: 'Planta da torre',
  width: 800,
  height: 600,
  url: '/images/img-2',
};
const hall: ShownImageVm = { ...tower, id: 'img-3', name: 'O porão', url: '/images/img-3' };

describe('LeftImagesBlock (C5-02, C6-08)', () => {
  const asPhone = (matches: boolean) => {
    window.matchMedia = (() => ({
      matches,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
    })) as unknown as typeof window.matchMedia;
  };
  afterEach(() => {
    delete (window as unknown as Record<string, unknown>)['matchMedia'];
  });

  function mount(images: readonly ShownImageVm[], phone: boolean) {
    asPhone(phone);
    const open = vi
      .fn()
      .mockReturnValue({ afterClosed: () => ({ subscribe: () => undefined }), close: vi.fn() });
    TestBed.configureTestingModule({ providers: [{ provide: MatDialog, useValue: { open } }] });
    const fixture = TestBed.createComponent(LeftImagesBlock);
    fixture.componentRef.setInput('images', images);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, open };
  }

  it('draws nothing while the list is empty', () => {
    expect(mount([], true).el.querySelector('app-fold')).toBeNull();
  });

  it('is a fold of one row on a phone, "Imagens que o mestre deixou (2)", closed until opened', () => {
    const { fixture, el } = mount([tower, hall], true);
    const button = el.querySelector('button.mr-fold__button') as HTMLButtonElement;
    expect(button.textContent).toContain('Imagens que o mestre deixou (2)');
    expect(button.getAttribute('aria-expanded')).toBe('false');
    expect(el.querySelector('.thumb')).toBeNull();
    button.click();
    fixture.detectChanges();
    expect(el.querySelectorAll('.thumb').length).toBe(2);
  });

  it('shows the thumbnails at once on a computer, with no button to fold it', () => {
    const { el } = mount([tower, hall], false);
    expect(el.querySelector('button.mr-fold__button')).toBeNull();
    expect(el.querySelectorAll('.thumb').length).toBe(2);
    expect(el.querySelector('.thumb__img')?.getAttribute('src')).toBe('/images/img-2/thumb');
  });

  it('names each thumbnail button after its image and opens the view', () => {
    const { el, open } = mount([tower], false);
    const thumb = el.querySelector('button.thumb') as HTMLButtonElement;
    expect(thumb.getAttribute('aria-label')).toBe('Ver Planta da torre em tela cheia');
    expect(thumb.textContent).toContain('Planta da torre');
    thumb.click();
    expect(open).toHaveBeenCalledTimes(1);
  });
});
