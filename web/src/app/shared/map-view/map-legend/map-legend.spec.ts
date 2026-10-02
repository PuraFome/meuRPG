import { TestBed } from '@angular/core/testing';

import { ViewToken } from '../map-geometry';
import { MapLegend } from './map-legend';

describe('MapLegend', () => {
  function token(over: Partial<ViewToken>): ViewToken {
    return {
      characterId: 'c',
      name: 'Goblin',
      mine: false,
      hidden: false,
      xBp: 0,
      yBp: 0,
      ...over,
    };
  }

  function legendText(tokens: ViewToken[]): string[] {
    const fixture = TestBed.createComponent(MapLegend);
    fixture.componentRef.setInput('tokens', tokens);
    fixture.detectChanges();
    const items = (fixture.nativeElement as HTMLElement).querySelectorAll('li');
    return Array.from(items, (li) => li.textContent?.replace(/\s+/g, ' ').trim() ?? '');
  }

  it('names a hidden token with no stray space before the comma', () => {
    const text = legendText([token({ characterId: 'g', name: 'Goblin', hidden: true })]);
    expect(text.at(-1)).toMatch(/Goblin, escondido$/);
  });

  it('marks the player\'s own token "(você)"', () => {
    const text = legendText([token({ characterId: 'p', name: 'Pensantus', mine: true })]);
    expect(text.at(-1)).toMatch(/Pensantus \(você\)$/);
  });
});
