import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { PanelAction } from './panel-action';

describe('PanelAction', () => {
  beforeEach(() => TestBed.configureTestingModule({ providers: [provideRouter([])] }));

  function setup(inputs: Record<string, unknown>) {
    const fixture = TestBed.createComponent(PanelAction);
    for (const [name, value] of Object.entries(inputs)) {
      fixture.componentRef.setInput(name, value);
    }
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('is a link with its icon before the words when it has a place to go', () => {
    const { el } = setup({ label: 'Novo mapa', icon: 'add', link: ['/campaigns', 'c1', 'maps'] });
    const link = el.querySelector('a')!;
    expect(link.getAttribute('href')).toBe('/campaigns/c1/maps');
    expect(link.querySelector('mat-icon')?.getAttribute('aria-hidden')).toBe('true');
    expect(link.textContent?.replace('add', '').trim()).toBe('Novo mapa');
    expect(el.querySelector('button')).toBeNull();
  });

  it('is a button that says it was pressed when it has no place to go, with no icon if none is given', () => {
    const { fixture, el } = setup({ label: 'Ver todos os 9 NPCs', underline: true });
    let pressed = 0;
    fixture.componentInstance.pressed.subscribe(() => pressed++);
    const button = el.querySelector('button')!;
    expect(button.querySelector('mat-icon')).toBeNull();
    expect(button.classList).toContain('act--underline');
    button.click();
    expect(pressed).toBe(1);
  });
});
