import { TestBed } from '@angular/core/testing';

import { type ExtraDieField, dieFields } from '../../../../core/effects/effects';
import { ExtraDice } from './extra-dice';

const bless = dieFields([{ name: 'Bênção', faces: 4, sign: 1 }]);
const bane = dieFields([{ name: 'Perdição', faces: 4, sign: -1 }]);

describe('ExtraDice', () => {
  function setup(fields: readonly ExtraDieField[]) {
    const fixture = TestBed.createComponent(ExtraDice);
    fixture.componentRef.setInput('fields', fields);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }
  const input = (el: HTMLElement, at = 0) => el.querySelectorAll<HTMLInputElement>('input')[at];
  const fill = (fixture: { detectChanges(): void }, el: HTMLElement, value: string, at = 0) => {
    const field = input(el, at);
    field.value = value;
    field.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  };

  it('draws nothing when no effect adds a die', () => {
    const { el } = setup([]);
    expect(el.querySelector('fieldset')).toBeNull();
  });

  it('labels the field, says what it is for and its range, and names the hint to the field', () => {
    const { el } = setup(bless);
    const field = input(el);
    expect(el.querySelector('label')?.textContent).toBe('Resultado do d4 (Bênção)');
    expect(el.querySelector('label')?.getAttribute('for')).toBe(field.id);
    const hint = el.querySelector(`#${field.getAttribute('aria-describedby')}`)!;
    expect(hint.textContent?.replace(/\s+/g, ' ').trim()).toBe(
      'Bênção soma 1d4 a esta jogada. Role um d4 além do d20. De 1 a 4.',
    );
    expect(el.querySelector('legend.mr-visually-hidden')).not.toBeNull();
  });

  it('says it takes the die away for Perdição', () => {
    const { el } = setup(bane);
    expect(el.textContent).toContain('Perdição subtrai 1d4 desta jogada.');
  });

  it('hands the faces typed back, and null for an empty or out-of-range field', () => {
    const { fixture, el } = setup([...bless, ...bane]);
    const cmp = fixture.componentInstance;
    expect(cmp.faces()).toEqual([null, null]);
    fill(fixture, el, '3', 0);
    expect(cmp.faces()).toEqual([3, null]);
    fill(fixture, el, '5', 1);
    expect(cmp.faces()).toEqual([3, null]);
    expect(input(el, 1).getAttribute('aria-invalid')).toBe('true');
    fill(fixture, el, '2', 1);
    expect(cmp.faces()).toEqual([3, 2]);
    expect(input(el, 1).getAttribute('aria-invalid')).toBeNull();
    fill(fixture, el, 'abc', 0);
    expect(cmp.faces()).toEqual([null, 2]);
  });
});
