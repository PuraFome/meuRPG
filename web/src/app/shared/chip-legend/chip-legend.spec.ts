import { TestBed } from '@angular/core/testing';

import type { MarksVm } from '../../core/characters/gear';
import { ChipLegend } from './chip-legend';

const marks = (m: Partial<MarksVm>): MarksVm => ({
  silvered: false,
  adamantine: false,
  magical: false,
  ...m,
});

function render(
  set: (c: ChipLegend, ref: ReturnType<typeof TestBed.createComponent<ChipLegend>>) => void,
) {
  const fixture = TestBed.createComponent(ChipLegend);
  set(fixture.componentInstance, fixture);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

const words = (el: HTMLElement) =>
  Array.from(el.querySelectorAll('li')).map((li) =>
    (li.textContent ?? '')
      .replace(/\s+/g, ' ')
      .replace(/auto_awesome|dark_mode|diamond|link|\+\d/g, '')
      .trim(),
  );

describe('ChipLegend', () => {
  it('draws nothing when no row has a chip', () => {
    const el = render((_, f) => f.componentRef.setInput('marks', [null, marks({})]));
    expect(el.querySelector('ul')).toBeNull();
  });

  it('lists the marks that appear, once each, in the order of the grid, then the bonus', () => {
    const el = render((_, f) => {
      f.componentRef.setInput('marks', [marks({ magical: true }), marks({ silvered: true }), null]);
      f.componentRef.setInput('bonus', 'counts');
    });
    expect(words(el)).toEqual(['Prateada', 'Mágica', 'Bônus mágico, já somado']);
  });

  it('says the bonus does not count, with a dashed chip, when it is idle', () => {
    const el = render((_, f) => f.componentRef.setInput('bonus', 'idle'));
    expect(words(el)).toEqual(['Bônus mágico, não conta agora']);
    expect(el.querySelector('.bonus-chip--idle')).not.toBeNull();
  });

  it('adds the two attunement chips between the marks and the bonus', () => {
    const el = render((_, f) => {
      f.componentRef.setInput('marks', [marks({ adamantine: true })]);
      f.componentRef.setInput('attunement', true);
      f.componentRef.setInput('bonus', 'counts');
    });
    expect(words(el)).toEqual([
      'Adamantina',
      'Sintonizado',
      'Pede sintonização',
      'Bônus mágico, já somado',
    ]);
  });

  it('is named for a screen reader, and hidden from one when it is decorative', () => {
    const named = render((_, f) => f.componentRef.setInput('bonus', 'counts'));
    expect(named.querySelector('ul')!.getAttribute('aria-label')).toBe('Legenda dos selos');
    expect(named.querySelector('ul')!.hasAttribute('aria-hidden')).toBe(false);
    const hidden = render((_, f) => {
      f.componentRef.setInput('bonus', 'counts');
      f.componentRef.setInput('decorative', true);
    });
    expect(hidden.querySelector('ul')!.getAttribute('aria-hidden')).toBe('true');
    expect(hidden.querySelector('ul')!.hasAttribute('aria-label')).toBe(false);
  });
});
