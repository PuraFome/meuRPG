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

describe('PlayerVitals under Escudo Arcano and Ajuda (PM-03a)', () => {
  const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim();

  function mount(over: Parameters<typeof pensantusVitals>[0], bonus: number | null) {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(PlayerVitals);
    fixture.componentRef.setInput('vitals', pensantusVitals(over));
    fixture.componentRef.setInput('sheet', { armorClass: 13, summary: 'Mago 5', senses: [] });
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('compact', true);
    fixture.componentRef.setInput('armorClassBonus', bonus);
    fixture.detectChanges();
    return fixture;
  }

  it('shows the armor class already summed, the small sum under it and the label under the cards', () => {
    const el = mount({}, 5).nativeElement as HTMLElement;
    expect(flat(el.querySelector('.shield__number'))).toBe('18');
    expect(flat(el.querySelector('.shield__sum'))).toBe('13 + 5');
    const label = el.querySelector('.effects[role="status"] .effect--shield');
    expect(flat(label)).toContain('Escudo Arcano +5 até a sua vez');
  });

  it('is the plain shield, with no sum and no label, without the spell', () => {
    const el = mount({}, 0).nativeElement as HTMLElement;
    expect(flat(el.querySelector('.shield__number'))).toBe('13');
    expect(el.querySelector('.shield__sum')).toBeNull();
    expect(el.querySelector('.effect')).toBeNull();
  });

  it('says once, politely, that the shield ended with the base class, and stops after six seconds', () => {
    vi.useFakeTimers();
    try {
      const fixture = mount({}, 5);
      const el = fixture.nativeElement as HTMLElement;
      expect(el.querySelector('.ended')).toBeNull();
      fixture.componentRef.setInput('armorClassBonus', 0);
      fixture.detectChanges();
      expect(flat(el.querySelector('.ended-live[role="status"] .ended'))).toContain(
        'O Escudo Arcano acabou. A sua CA voltou a 13.',
      );
      expect(flat(el.querySelector('.shield__number'))).toBe('13');
      vi.advanceTimersByTime(5999);
      fixture.detectChanges();
      expect(el.querySelector('.ended')).not.toBeNull();
      vi.advanceTimersByTime(1);
      fixture.detectChanges();
      expect(el.querySelector('.ended')).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it('does not say the shield ended when the combat or the combatant went away', () => {
    const fixture = mount({}, 5);
    fixture.componentRef.setInput('armorClassBonus', null);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).querySelector('.ended')).toBeNull();
  });

  it('hangs "+5 de Ajuda" off the maximum, stripes the bar and reads the number for a screen reader', () => {
    const el = mount({ hitPointsCurrent: 43, hitPointsMax: 43, hitPointsMaxBonus: 5 }, 0)
      .nativeElement as HTMLElement;
    expect(flat(el.querySelector('.hp__max'))).toBe('de 43');
    expect(flat(el.querySelector('.hp__top .hp__aid'))).toBe('+5 de Ajuda');
    expect(flat(el.querySelector('.effect--aid'))).toContain(
      'Ajuda: +5 nos PV até o mestre encerrar ou um descanso longo',
    );
    expect(flat(el.querySelector('.hp .mr-visually-hidden'))).toBe(
      'Pontos de vida: 43 de 43, 5 de Ajuda',
    );
    expect(el.querySelector('.hp__bar')?.getAttribute('aria-label')).toBe(
      '43 de 43 pontos de vida',
    );
    const [solid, striped] = Array.from(el.querySelectorAll<HTMLElement>('.hp__fill'));
    expect(parseFloat(solid.style.width)).toBeCloseTo((38 / 43) * 100);
    expect(parseFloat(striped.style.width)).toBeCloseTo((5 / 43) * 100);
  });

  it("leaves the striped part empty when the hit points are below the sheet's maximum", () => {
    const el = mount({ hitPointsCurrent: 31, hitPointsMax: 43, hitPointsMaxBonus: 5 }, 0)
      .nativeElement as HTMLElement;
    const [solid, striped] = Array.from(el.querySelectorAll<HTMLElement>('.hp__fill'));
    expect(parseFloat(solid.style.width)).toBeCloseTo((31 / 43) * 100);
    expect(parseFloat(striped.style.width)).toBe(0);
  });

  it('writes +10 for a third-level Ajuda', () => {
    const el = mount({ hitPointsCurrent: 48, hitPointsMax: 48, hitPointsMaxBonus: 10 }, 0)
      .nativeElement as HTMLElement;
    expect(flat(el.querySelector('.hp__aid'))).toBe('+10 de Ajuda');
    expect(flat(el.querySelector('.effect--aid'))).toContain('Ajuda: +10 nos PV');
  });

  it('writes nothing about Ajuda without it', () => {
    const plain = mount({}, 0).nativeElement as HTMLElement;
    expect(plain.querySelector('.hp__aid')).toBeNull();
    expect(plain.querySelectorAll('.hp__fill')).toHaveLength(1);
  });
});
