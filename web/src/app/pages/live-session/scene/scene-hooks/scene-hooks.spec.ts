import { TestBed } from '@angular/core/testing';

import { SceneHooks } from './scene-hooks';

describe('SceneHooks', () => {
  function setup(hooks: string, phone: boolean) {
    const fixture = TestBed.createComponent(SceneHooks);
    fixture.componentRef.setInput('hooks', hooks);
    fixture.componentRef.setInput('phone', phone);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { fixture, el, toggle: () => el.querySelector<HTMLButtonElement>('.mr-fold__button')! };
  }

  it('is folded on a computer too, shows the text as Markdown once opened and always shows "Só você vê" and the lock', () => {
    const { fixture, el, toggle } = setup('O mercador foi levado.\n\nMira está escondida.', false);
    expect(toggle().getAttribute('aria-expanded')).toBe('false');
    expect(el.querySelector('.sh__lock')?.textContent).toContain('Só você vê');
    expect(el.querySelector('.mr-fold__icon')?.textContent).toBe('lock');
    toggle().click();
    fixture.detectChanges();
    expect(el.querySelectorAll('.sh__text p')).toHaveLength(2);
  });

  it('is folded on a phone with the lock still on screen, and opens and closes with a named button', () => {
    const { fixture, el, toggle } = setup('Segredo', true);
    expect(toggle().getAttribute('aria-expanded')).toBe('false');
    expect(el.querySelector('.sh__text')).toBeNull();
    expect(el.querySelector('.sh__lock')?.textContent).toContain('Só você vê');
    expect(toggle().textContent).toContain('Ganchos e anotações');
    toggle().click();
    fixture.detectChanges();
    expect(el.querySelector('.sh__text')?.textContent).toContain('Segredo');
    expect(toggle().getAttribute('aria-expanded')).toBe('true');
    expect(toggle().getAttribute('aria-controls')).toBe(el.querySelector('.mr-fold__body')?.id);
    expect(el.querySelector('[role="region"]')?.getAttribute('aria-label')).toBe(
      'Ganchos e anotações',
    );
  });

  it('says there are no hooks when the scene has none', () => {
    const { fixture, el, toggle } = setup('  ', false);
    toggle().click();
    fixture.detectChanges();
    expect(el.querySelector('.sh__none')?.textContent).toContain('Esta cena não tem ganchos.');
  });
});
