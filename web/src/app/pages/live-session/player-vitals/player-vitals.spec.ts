import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { pensantusVitals } from '../testing';
import { PlayerVitals } from './player-vitals';

describe('PlayerVitals as a beast (MR-037, E9-11)', () => {
  function render(over: Parameters<typeof pensantusVitals>[0], beastAc: number | null) {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(PlayerVitals);
    fixture.componentRef.setInput(
      'vitals',
      pensantusVitals({ name: 'Sálvia', hitPointsCurrent: 38, hitPointsMax: 38, ...over }),
    );
    fixture.componentRef.setInput('sheet', { armorClass: 12, summary: 'Druida 5', senses: [] });
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('beastAc', beastAc);
    fixture.componentRef.setInput('compact', true);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }
  const flat = (n: Element | null) => n?.textContent?.replace(/\s+/g, ' ').trim();

  it('shows the two reserves where the hit points box is, the beast\'s armor class, and "Sem magias" in place of the slots', () => {
    const el = render(
      {
        wildShape: {
          beastKey: 'monster:wolf',
          beastNamePt: 'Lobo',
          hitPointsCurrent: 11,
          hitPointsMax: 11,
        },
      },
      13,
    );
    expect(Array.from(el.querySelectorAll('app-wild-pools .pool'), flat)).toEqual([
      'PV do Lobo11 de 11',
      'PV da Sálvia38 de 38',
    ]);
    expect(el.querySelector('.hp')).toBeNull();
    // One armor class on the page, the beast's: not the druid's own 12.
    expect(flat(el.querySelector('.shield'))).toContain('13');
    expect(flat(el.querySelector('.shield'))).not.toContain('12');
    expect(flat(el.querySelector('.shield .stats__label'))).toBe('CA do Lobo');
    expect(el.querySelector('.slots')).toBeNull();
    expect(flat(el.querySelector('.nospells'))).toBe('blockSem magias na forma de fera.');
  });

  it('is the ordinary card otherwise', () => {
    const el = render({}, null);
    expect(el.querySelector('app-wild-pools')).toBeNull();
    expect(flat(el.querySelector('.shield'))).toContain('12');
    expect(el.querySelector('.slots')).not.toBeNull();
  });
});

describe('PlayerVitals hit dice by size (decisions batch 2 B, 6)', () => {
  function render(over: Parameters<typeof pensantusVitals>[0]) {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(PlayerVitals);
    fixture.componentRef.setInput('vitals', pensantusVitals(over));
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }
  const flat = (n: Element | null) => n?.textContent?.replace(/\s+/g, ' ').trim();

  it('says the dice by size and what is left of each', () => {
    const el = render({
      hitDice: '5d10 e 1d6',
      hitDiceSizes: [
        { faces: 10, total: 5, used: 2 },
        { faces: 6, total: 1, used: 0 },
      ],
      hitDiceTotal: 6,
      hitDiceUsed: 2,
    });
    const box = Array.from(el.querySelectorAll('.box')).find((b) =>
      flat(b)?.startsWith('Dados de vida'),
    );
    expect(flat(box?.querySelector('.box__value') ?? null)).toBe('5d10 e 1d6');
    expect(flat(box?.querySelector('.box__words') ?? null)).toBe('Restam 3 de 5d10 e 1 de 1d6');
  });

  it('says one size as before: what is left of how many', () => {
    const el = render({});
    expect(flat(el.querySelector('.box__words'))).toBe('Restam 2 de 3d6');
  });
});
