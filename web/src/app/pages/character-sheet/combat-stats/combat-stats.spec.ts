import { TestBed } from '@angular/core/testing';

import { CombatStats } from './combat-stats';

describe('CombatStats under Ajuda (PM-03a)', () => {
  const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim();

  function mount(over: Record<string, unknown>) {
    const fixture = TestBed.createComponent(CombatStats);
    fixture.componentRef.setInput('armorClass', 14);
    fixture.componentRef.setInput('initiative', 2);
    fixture.componentRef.setInput('speedFt', 30);
    fixture.componentRef.setInput('hitPointsMax', 38);
    fixture.componentRef.setInput('hitDice', '5d8');
    for (const [name, value] of Object.entries(over)) {
      fixture.componentRef.setInput(name, value);
    }
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('draws "43 de 43", the sheet maximum, "+5 de Ajuda" and the banner', () => {
    const el = mount({ hitPointsCurrent: 43, aid: 5 });
    expect(flat(el.querySelector('.hp__value'))).toBe('43 de 43');
    expect(flat(el.querySelector('.hp__note'))).toBe('máximo 38 da ficha');
    expect(flat(el.querySelector('.hp__tag'))).toBe('+5 de Ajuda');
    expect(flat(el.querySelector('.aid-banner'))).toContain(
      'Ajuda: +5 nos PV até o mestre encerrar ou um descanso longo',
    );
    expect(el.querySelector('.aid-banner')?.getAttribute('role')).toBe('status');
  });

  it('keeps the old text without the live numbers or without Ajuda', () => {
    for (const el of [mount({}), mount({ hitPointsCurrent: 30, aid: 0 }), mount({ aid: 5 })]) {
      expect(flat(el.querySelector('.hp__note'))).toBe('Os atuais aparecem na sessão');
      expect(el.querySelector('.aid-banner')).toBeNull();
    }
  });
});
