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

  describe('what the map has', () => {
    function render(inputs: Record<string, unknown>): HTMLElement {
      const fixture = TestBed.createComponent(MapLegend);
      for (const [key, value] of Object.entries(inputs)) {
        fixture.componentRef.setInput(key, value);
      }
      fixture.detectChanges();
      return fixture.nativeElement as HTMLElement;
    }
    const point = (kind: number, revealed = true) => ({ id: `p${kind}${revealed}`, name: 'P', kind, xBp: 0, yBp: 0, revealed });
    const words = (el: HTMLElement) => (el.textContent ?? '').replace(/\s+/g, ' ');

    it('lists only the kinds and the states of the points it is given', () => {
      const el = render({ states: true, points: [point(3, true)] });
      expect(words(el)).toContain('Cena de RP');
      expect(words(el)).toContain('Revelado');
      for (const missing of ['Batalha', 'Submapa', 'Escondido']) {
        expect(words(el)).not.toContain(missing);
      }
    });

    it('lists a hidden state only when a point is hidden, and a trap or a light adds no plain kind', () => {
      const el = render({ states: true, points: [point(1, false), point(4, true), point(6, true)] });
      expect(words(el)).toContain('Batalha');
      expect(words(el)).toContain('Escondido');
      expect(words(el)).not.toContain('Revelado');
      expect(words(el)).not.toContain('Cena de RP');
    });

    it('without the points it keeps its old behaviour: every kind (the session screens)', () => {
      const el = render({ states: true });
      for (const kind of ['Batalha', 'Submapa', 'Cena de RP', 'Revelado', 'Escondido']) {
        expect(words(el)).toContain(kind);
      }
    });

    it('draws a token as the map does: the same initial, an NPC as the white rounded square', () => {
      const goblin: ViewToken = { characterId: 'g', name: 'Goblin 2', mine: false, hidden: false, xBp: 0, yBp: 0, kind: 2 };
      const el = render({ tokens: [goblin], kindShapes: true, initialOf: () => 'G2' });
      const disc = el.querySelector('.lg__disc')!;
      expect(disc.textContent?.trim()).toBe('G2');
      expect(disc.classList).toContain('lg__disc--npc');
    });

    it('draws a creature as the maps do: round and hollow, with a dashed outline', () => {
      const raven: ViewToken = { characterId: 'p', creatureId: 'r', name: 'Nanquim', mine: false, hidden: false, xBp: 0, yBp: 0 };
      const el = render({ tokens: [raven], kindShapes: true });
      expect(el.querySelector('.lg__disc')?.classList).toContain('lg__disc--creature');
    });

    it('keeps "Tokens" above the names where asked, even without the states', () => {
      const t = token({ characterId: 'p', name: 'Pensantus' });
      expect(words(render({ states: false, tokens: [t] }))).not.toContain('Tokens');
      expect(words(render({ states: false, tokens: [t], tokensTitle: true }))).toContain('Tokens');
    });
  });
});
