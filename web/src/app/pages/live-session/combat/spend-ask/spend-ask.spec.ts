import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import { ActionOptionSchema } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { SpendAsk } from './spend-ask';

/** An invented table feature: spends 1 to 3 points of a pool and heals 1d6 per point. */
const option = (usesLeft = 5) =>
  create(ActionOptionSchema, {
    action: {
      key: 'feature:folego-da-mesa',
      namePt: 'Fôlego da Mesa',
      resourceKey: 'reserva_de_teste',
      cost: 0,
      costMax: 3,
      heal: { count: 1, sides: 6 },
      healPerPoint: { count: 1, sides: 6 },
    },
    usesLeft,
    enabled: true,
  });

describe('SpendAsk: "quantos?"', () => {
  function setup(usesLeft = 5) {
    const fixture = TestBed.createComponent(SpendAsk);
    fixture.componentRef.setInput('option', option(usesLeft));
    const confirmed: number[] = [];
    let cancelled = 0;
    fixture.componentInstance.confirmed.subscribe((n) => confirmed.push(n));
    fixture.componentInstance.cancelled.subscribe(() => cancelled++);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const button = (label: string) =>
      [...el.querySelectorAll<HTMLButtonElement>('button')].find(
        (b) => (b.getAttribute('aria-label') ?? b.textContent ?? '').trim() === label,
      )!;
    const click = (label: string) => {
      button(label).click();
      fixture.detectChanges();
    };
    const status = () => el.querySelector('[role="status"]')!.textContent!.trim();
    return { el, fixture, confirmed, cancelled: () => cancelled, click, button, status };
  }

  it('is a named group that asks "Quantos?" with the range from the least to what is left', () => {
    const { el } = setup();
    const group = el.querySelector('[role="group"]')!;
    expect(group.getAttribute('aria-label')).toBe('Quantos? Fôlego da Mesa');
    expect(el.textContent).toContain('Quantos?');
    expect(el.textContent).toContain('de 1 a 3');
  });

  it('stops the range at what is left of the pool', () => {
    const { el } = setup(2);
    expect(el.textContent).toContain('de 1 a 2');
  });

  it('starts at the least and says in a live region what it will cost and heal', () => {
    const { status } = setup();
    expect(status()).toBe('Gasta 1 · cura 1d6');
  });

  it('follows the stepper: three points read "Gasta 3 · cura 3d6", and the bounds hold', () => {
    const { click, status, el } = setup();
    click('Gastar um a mais');
    click('Gastar um a mais');
    expect(status()).toBe('Gasta 3 · cura 3d6');
    click('Gastar um a mais');
    expect(status()).toBe('Gasta 3 · cura 3d6');
    expect(el.querySelector('output')!.textContent!.trim()).toBe('3');
  });

  it('every button has a name', () => {
    const { el } = setup();
    for (const b of el.querySelectorAll('button')) {
      expect(b.getAttribute('aria-label') ?? b.textContent?.trim()).toBeTruthy();
    }
  });

  it('confirms the number picked, once, and cancels without one', () => {
    const { click, confirmed, cancelled } = setup();
    click('Gastar um a mais');
    click('Gastar um a mais');
    click('Usar Fôlego da Mesa gastando 3');
    expect(confirmed).toEqual([3]);
    click('Cancelar');
    expect(cancelled()).toBe(1);
    expect(confirmed).toEqual([3]);
  });

  it('cancels with Escape', () => {
    const { fixture, cancelled } = setup();
    (fixture.nativeElement as HTMLElement).dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape' }),
    );
    expect(cancelled()).toBe(1);
  });
});
