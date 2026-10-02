import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormBuilder } from '@angular/forms';

import { ROLL_DIE, RollDie } from '../../../core/dice/dice';
import { AbilityScores } from './ability-scores';

/** The dice come out in this order: 6 5 4 1 (15), 6 6 5 1 (17), 4 4 3 1 (11), 5 5 5 1 (15), 3 3 2 1 (8), 6 6 6 1 (18). */
const DICE = [6, 5, 4, 1, 6, 6, 5, 1, 4, 4, 3, 1, 5, 5, 5, 1, 3, 3, 2, 1, 6, 6, 6, 1];
// Best first: 18, 17, 15, 15, 11, 8.

function sequence(faces: number[]): RollDie {
  let i = 0;
  return () => faces[i++ % faces.length];
}

function stubPhone(phone: boolean): void {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: phone && query.includes('767'),
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
  }));
}

describe('AbilityScores (E6-20, E6-20b)', () => {
  afterEach(() => vi.unstubAllGlobals());

  function setup(phone = false) {
    stubPhone(phone);
    TestBed.configureTestingModule({
      providers: [{ provide: ROLL_DIE, useValue: sequence(DICE) }],
    });
    const group = new FormBuilder().nonNullable.group({
      str: 10,
      dex: 10,
      con: 10,
      int: 10,
      wis: 10,
      cha: 10,
    });
    const fixture = TestBed.createComponent(AbilityScores);
    fixture.componentRef.setInput('group', group);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { fixture, el, group };
  }

  function choose(fixture: ComponentFixture<AbilityScores>, title: string): void {
    const radios = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLInputElement>(
        'input[type=radio]',
      ),
    );
    const radio = radios.find((r) => r.closest('mat-radio-button')?.textContent?.includes(title));
    radio!.click();
    fixture.detectChanges();
  }

  const selects = (el: HTMLElement) => Array.from(el.querySelectorAll<HTMLSelectElement>('select'));
  const chips = (el: HTMLElement) => Array.from(el.querySelectorAll('app-dice-result .chip'));
  const total = (chip: Element) => chip.querySelector('.chip__total')?.textContent?.trim();

  function pick(fixture: ComponentFixture<AbilityScores>, select: HTMLSelectElement, text: string) {
    const option = Array.from(select.options).find((o) => o.textContent?.trim().startsWith(text));
    select.value = option!.value;
    select.dispatchEvent(new Event('change'));
    fixture.detectChanges();
  }

  it('starts on "Digitar" with today\'s six number fields', () => {
    const { el, fixture } = setup();
    expect(el.querySelectorAll('input[type=number]').length).toBe(6);
    expect(el.querySelector('app-ability-placing')).toBeNull();
    expect(fixture.componentInstance.incomplete()).toBe(false);
  });

  it('offers the three methods as radios', () => {
    const { el } = setup();
    const titles = Array.from(el.querySelectorAll('.method__title')).map((t) => t.textContent);
    expect(titles).toEqual(['Digitar', 'Rolar 4d6', 'Conjunto padrão']);
  });

  it('"Rolar 4d6" rolls six results, best first, with the lowest die struck', () => {
    const { el, fixture } = setup();
    choose(fixture, 'Rolar 4d6');
    expect(chips(el).map(total)).toEqual(['18', '17', '15', '15', '11', '8']);
    const first = chips(el)[0];
    // 6 6 6 1: the 1 is the dashed one.
    expect(first.querySelectorAll('.chip__die--dropped').length).toBe(1);
    expect(first.querySelector('.chip__die--dropped')?.textContent?.trim()).toBe('1');
    expect(first.getAttribute('aria-label')).toBe(
      '18: dados 6, 6, 6 e 1; o 1 foi descartado. Livre.',
    );
    expect(el.textContent).toContain('Rolar de novo');
  });

  it('is incomplete until every result sits on an ability', () => {
    const { el, fixture } = setup();
    choose(fixture, 'Rolar 4d6');
    expect(fixture.componentInstance.incomplete()).toBe(true);
    expect(el.textContent).toContain('Faltam 6 atributos');
  });

  it('places a result through the ability select and writes the number into the form', () => {
    const { el, fixture, group } = setup();
    choose(fixture, 'Rolar 4d6');
    pick(fixture, selects(el)[0], '18');
    expect(group.getRawValue().str).toBe(18);
    expect(chips(el)[0].getAttribute('aria-label')).toBe(
      '18: dados 6, 6, 6 e 1; o 1 foi descartado. Em Força.',
    );
    expect(el.textContent).toContain('Faltam 5 atributos');
  });

  it('places all six, completing the step, and the six values are the six results', () => {
    const { el, fixture, group } = setup();
    choose(fixture, 'Rolar 4d6');
    // Indexes 0..5 in the results list are 18, 17, 15, 15, 11, 8.
    const wanted = ['17', '18', '15', '11', '8', '15'];
    selects(el).forEach((select, i) => {
      // The first option with exactly the wanted total (not one marked "em ...").
      const option = Array.from(select.options).find(
        (o) => o.textContent?.trim() === wanted[i] && !o.selected,
      )!;
      select.value = option.value;
      select.dispatchEvent(new Event('change'));
      fixture.detectChanges();
    });
    const values = Object.values(group.getRawValue()).sort((a, b) => a - b);
    expect(values).toEqual([8, 11, 15, 15, 17, 18]);
    expect(fixture.componentInstance.incomplete()).toBe(false);
    expect(el.textContent).toContain('Os seis resultados estão colocados.');
  });

  it('swaps two abilities when a result placed elsewhere is picked', () => {
    const { el, fixture, group } = setup();
    choose(fixture, 'Rolar 4d6');
    pick(fixture, selects(el)[0], '18'); // Força 18
    pick(fixture, selects(el)[1], '17'); // Destreza 17
    pick(fixture, selects(el)[1], '18 (em Força)'); // Destreza takes Força's 18
    expect(group.getRawValue().dex).toBe(18);
    expect(group.getRawValue().str).toBe(17);
  });

  it('"Rolar de novo" replaces the results and clears the placing', () => {
    const { el, fixture } = setup();
    choose(fixture, 'Rolar 4d6');
    pick(fixture, selects(el)[0], '18');
    const reroll = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Rolar de novo'),
    )!;
    reroll.click();
    fixture.detectChanges();
    // The die sequence repeats, so the same six come out, all free again.
    expect(chips(el).every((c) => c.classList.contains('chip--free'))).toBe(true);
    expect(fixture.componentInstance.incomplete()).toBe(true);
  });

  it('"Conjunto padrão" lists 15, 14, 13, 12, 10 and 8 with no dice and no re-roll', () => {
    const { el, fixture } = setup();
    choose(fixture, 'Conjunto padrão');
    expect(chips(el).map(total)).toEqual(['15', '14', '13', '12', '10', '8']);
    expect(el.querySelectorAll('.chip__die').length).toBe(0);
    expect(el.textContent).not.toContain('Rolar de novo');
    pick(fixture, selects(el)[2], '15');
    expect(fixture.componentInstance.incomplete()).toBe(true);
  });

  it('keeps the rolled results when the person goes back and forth', () => {
    const { el, fixture } = setup();
    choose(fixture, 'Rolar 4d6');
    pick(fixture, selects(el)[0], '18');
    choose(fixture, 'Digitar');
    expect(fixture.componentInstance.incomplete()).toBe(false);
    choose(fixture, 'Rolar 4d6');
    expect(chips(el)[0].classList.contains('chip--placed')).toBe(true);
  });

  describe('on a phone', () => {
    const slots = (el: HTMLElement) => Array.from(el.querySelectorAll<HTMLButtonElement>('.slot'));

    it('puts a result on an ability with two taps, saying so', () => {
      const { el, fixture, group } = setup(true);
      choose(fixture, 'Rolar 4d6');
      expect(el.querySelectorAll('select').length).toBe(0);
      const first = chips(el)[0] as HTMLButtonElement;
      first.click();
      fixture.detectChanges();
      expect(first.getAttribute('aria-pressed')).toBe('true');
      expect(first.textContent).toContain('Escolhido');
      expect(el.querySelector('.status')?.textContent).toContain('18 escolhido');
      expect(slots(el)[0].getAttribute('aria-label')).toBe('Força: colocar o 18');

      slots(el)[0].click();
      fixture.detectChanges();
      expect(group.getRawValue().str).toBe(18);
      expect(el.querySelector('.status')?.textContent).toContain('18 em Força.');
      expect(chips(el)[0].textContent).toContain('em Força');
      expect(chips(el)[0].getAttribute('aria-pressed')).toBe('false');
    });

    it('does not let a free ability be tapped while no result is chosen', () => {
      const { el, fixture } = setup(true);
      choose(fixture, 'Rolar 4d6');
      expect(slots(el).every((s) => s.disabled)).toBe(true);
    });

    it('picks a placed result up by tapping its ability, then swaps on the next', () => {
      const { el, fixture, group } = setup(true);
      choose(fixture, 'Rolar 4d6');
      (chips(el)[0] as HTMLButtonElement).click(); // 18
      fixture.detectChanges();
      slots(el)[0].click(); // Força 18
      fixture.detectChanges();
      (chips(el)[1] as HTMLButtonElement).click(); // 17
      fixture.detectChanges();
      slots(el)[1].click(); // Destreza 17
      fixture.detectChanges();

      slots(el)[0].click(); // pick up Força's 18
      fixture.detectChanges();
      expect(chips(el)[0].getAttribute('aria-pressed')).toBe('true');
      slots(el)[1].click(); // onto Destreza: the two swap
      fixture.detectChanges();
      expect(group.getRawValue().str).toBe(17);
      expect(group.getRawValue().dex).toBe(18);
    });
  });
});
