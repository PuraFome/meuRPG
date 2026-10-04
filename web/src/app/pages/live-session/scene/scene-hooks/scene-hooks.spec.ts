import { TestBed } from '@angular/core/testing';

import { SceneHooks } from './scene-hooks';

describe('SceneHooks', () => {
  function setup(hooks: string, phone: boolean) {
    const fixture = TestBed.createComponent(SceneHooks);
    fixture.componentRef.setInput('hooks', hooks);
    fixture.componentRef.setInput('phone', phone);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { fixture, el, toggle: () => el.querySelector<HTMLButtonElement>('.sh__toggle')! };
  }

  it('is open on a computer, shows the text as Markdown and always shows the lock and "Só você vê"', () => {
    const { el, toggle } = setup('O mercador foi levado.\n\nMira está escondida.', false);
    expect(toggle().getAttribute('aria-expanded')).toBe('true');
    expect(el.querySelectorAll('.sh__text p')).toHaveLength(2);
    expect(el.querySelector('.sh__lock')?.textContent).toContain('Só você vê');
    expect(el.querySelector('.sh__lock mat-icon')?.textContent).toBe('lock');
  });

  it('is folded on a phone with the lock still on screen, and opens and closes with a 48px named button', () => {
    const { fixture, el, toggle } = setup('Segredo', true);
    expect(toggle().getAttribute('aria-expanded')).toBe('false');
    expect(el.querySelector('.sh__text')).toBeNull();
    expect(el.querySelector('.sh__lock')?.textContent).toContain('Só você vê');
    expect(toggle().getAttribute('aria-label')).toBe('Abrir os ganchos e anotações');
    toggle().click();
    fixture.detectChanges();
    expect(el.querySelector('.sh__text')?.textContent).toContain('Segredo');
    expect(toggle().getAttribute('aria-label')).toBe('Recolher os ganchos e anotações');
    expect(toggle().getAttribute('aria-controls')).toBe(el.querySelector('.sh__body')?.id);
  });

  it('says there are no hooks when the scene has none', () => {
    const { el } = setup('  ', false);
    expect(el.querySelector('.sh__none')?.textContent).toContain('Esta cena não tem ganchos.');
  });
});
