import { TestBed } from '@angular/core/testing';

import { mapToken } from '../../core/maps/map-builders-testing';
import { settle } from '../../../test-settle';
import { DarkMarks } from './dark-marks';

describe('DarkMarks', () => {
  const tokens = [
    mapToken('c-brisa', 'Brisa', { xBp: 2000, yBp: 3000 }),
    mapToken('c-jabuti', 'Jabuti', { xBp: 8000, yBp: 3000 }),
    mapToken('c-bilbo', 'bilbo', { xBp: 5000, yBp: 5000 }),
  ];

  async function render(chosen: string | null) {
    const fixture = TestBed.createComponent(DarkMarks);
    fixture.componentRef.setInput('tokens', tokens);
    fixture.componentRef.setInput('dark', new Set(['c-brisa', 'c-jabuti']));
    fixture.componentRef.setInput('chosen', chosen);
    await settle(fixture);
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('rings everyone in the dark and nobody else, at their squares', async () => {
    const { el } = await render(null);
    const rings = Array.from(el.querySelectorAll<HTMLElement>('.ring'));
    expect(rings.map((r) => r.dataset['of'])).toEqual(['c-brisa', 'c-jabuti']);
    expect(rings[0].style.left).toBe('20%');
    expect(rings[0].style.top).toBe('30%');
  });

  it('puts the label “<Nome> · no escuro” on the chosen one only', async () => {
    const { el } = await render('c-jabuti');
    const labels = Array.from(el.querySelectorAll('.label'));
    expect(labels).toHaveLength(1);
    expect(labels[0].textContent?.trim()).toBe('Jabuti · no escuro');
  });

  it('has no label until someone is chosen, and is hidden from assistive tech', async () => {
    const { el } = await render(null);
    expect(el.querySelector('.label')).toBeNull();
    expect(el.getAttribute('aria-hidden')).toBe('true');
  });

  it('ignores a chosen character who left the dark', async () => {
    const { el } = await render('c-bilbo');
    expect(el.querySelector('.label')).toBeNull();
  });
});
