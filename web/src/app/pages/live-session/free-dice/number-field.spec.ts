import { TestBed } from '@angular/core/testing';

import { NumberField, numberText, typedNumber } from './number-field';

describe('number-field helpers', () => {
  it('writes the number, with the plus of a modifier and the typographic minus', () => {
    expect(numberText(3, true)).toBe('+3');
    expect(numberText(0, true)).toBe('0');
    expect(numberText(-20, true)).toBe('−20');
    expect(numberText(2, false)).toBe('2');
  });

  it('reads what was typed: signs, spaces and the typographic minus', () => {
    expect(typedNumber('+3')).toBe(3);
    expect(typedNumber(' −20 ')).toBe(-20);
    expect(typedNumber('-4')).toBe(-4);
    expect(typedNumber('7')).toBe(7);
    expect(typedNumber('')).toBeNull();
    expect(typedNumber('abc')).toBeNull();
    expect(typedNumber('1.5')).toBeNull();
    expect(typedNumber('--3')).toBeNull();
  });
});

describe('NumberField', () => {
  function mount(value: number) {
    const fixture = TestBed.createComponent(NumberField);
    const ref = fixture.componentRef;
    ref.setInput('inputId', 'n');
    ref.setInput('value', value);
    ref.setInput('min', 1);
    ref.setInput('max', 20);
    ref.setInput('minusLabel', 'Menos');
    ref.setInput('plusLabel', 'Mais');
    const changes: number[] = [];
    fixture.componentInstance.valueChange.subscribe((n) => changes.push(n));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { fixture, el, changes, box: el.querySelector('input')! };
  }

  it('does nothing at a limit, but keeps the button in the tab order', () => {
    const low = mount(1);
    const minus = low.el.querySelector<HTMLElement>('[aria-label="Menos"]')!;
    expect(minus.getAttribute('aria-disabled')).toBe('true');
    minus.click();
    expect(low.changes).toEqual([]);
    const high = mount(20);
    high.el.querySelector<HTMLElement>('[aria-label="Mais"]')!.click();
    expect(high.changes).toEqual([]);
  });

  it('puts a typed number in range and writes it back', () => {
    const { box, changes } = mount(5);
    box.value = '99';
    box.dispatchEvent(new Event('change'));
    expect(changes).toEqual([20]);
    expect(box.value).toBe('20');
  });

  it('goes back to the last good number when what was typed is no number', () => {
    const { box, changes } = mount(5);
    box.value = 'abc';
    box.dispatchEvent(new Event('change'));
    expect(changes).toEqual([]);
    expect(box.value).toBe('5');
  });

  it('commits on Enter too', () => {
    const { box, changes } = mount(5);
    box.value = '8';
    box.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));
    expect(changes).toEqual([8]);
  });
});
