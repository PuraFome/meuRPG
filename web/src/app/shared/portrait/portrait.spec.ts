import { TestBed } from '@angular/core/testing';

import { Portrait } from './portrait';

describe('Portrait', () => {
  function setup(src: string, name: string) {
    const fixture = TestBed.createComponent(Portrait);
    fixture.componentRef.setInput('src', src);
    fixture.componentRef.setInput('name', name);
    fixture.componentRef.setInput('size', 64);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('shows the image, named for a screen reader', () => {
    const { el } = setup('/images/a/thumb', 'Mira');
    expect(el.querySelector('img')?.getAttribute('src')).toBe('/images/a/thumb');
    expect(el.getAttribute('role')).toBe('img');
    expect(el.getAttribute('aria-label')).toBe('Retrato de Mira');
  });

  it('shows the initials with no image, and when the image fails (a 404 off the stage)', () => {
    const { el, fixture } = setup('', 'Capitão Goblin');
    expect(el.querySelector('.pt__initials')?.textContent).toBe('CG');
    fixture.componentRef.setInput('src', '/images/a');
    fixture.detectChanges();
    el.querySelector('img')!.dispatchEvent(new Event('error'));
    fixture.detectChanges();
    expect(el.querySelector('img')).toBeNull();
    expect(el.querySelector('.pt__initials')?.textContent).toBe('CG');
    // A new URL tries again.
    fixture.componentRef.setInput('src', '/images/b');
    fixture.detectChanges();
    expect(el.querySelector('img')).not.toBeNull();
  });
});
