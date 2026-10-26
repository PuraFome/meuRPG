import { TestBed } from '@angular/core/testing';

import { MoveOverlay } from './move-overlay';

describe('MoveOverlay', () => {
  function mount(inputs: Record<string, unknown>) {
    const fixture = TestBed.createComponent(MoveOverlay);
    for (const [k, v] of Object.entries(inputs)) {
      fixture.componentRef.setInput(k, v);
    }
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('is drawing only: hidden from the screen reader, and it draws nothing with nothing to say', () => {
    const el = mount({});
    expect(el.getAttribute('aria-hidden')).toBe('true');
    expect(el.querySelectorAll('.ring, .x, .tag, line')).toHaveLength(0);
  });

  it('rings the chosen tokens, and the plan is a dashed line to a ring', () => {
    const el = mount({
      columns: 20,
      rings: [{ xBp: 2500, yBp: 2500 }],
      plan: { from: { xBp: 2500, yBp: 2500 }, to: { xBp: 7500, yBp: 2500 } },
    });
    expect(el.querySelectorAll('.ring')).toHaveLength(2);
    expect(el.querySelector('.ring--dest')?.getAttribute('style')).toContain('left: 75%');
    expect(el.querySelector('.ring')?.getAttribute('style')).toContain('width: 7.5%');
    expect(el.querySelector('line.line--plan')).not.toBeNull();
  });

  it('after a move that stopped, draws the line walked, an X where it was meant to go and the label', () => {
    const el = mount({
      columns: 20,
      trail: {
        from: { xBp: 1000, yBp: 5000 },
        to: { xBp: 4000, yBp: 5000 },
        stop: { label: '7,5 m · parou antes', at: { xBp: 8000, yBp: 5000 } },
      },
    });
    expect(el.querySelector('line.line--done')).not.toBeNull();
    expect(el.querySelector('line.line--plan')).not.toBeNull();
    expect(el.querySelector('.x')?.getAttribute('style')).toContain('left: 80%');
    expect(el.querySelector('.tag')?.textContent?.trim()).toBe('7,5 m · parou antes');
    expect(el.querySelector('.tag')?.getAttribute('style')).toContain('left: 60%');
  });
});
