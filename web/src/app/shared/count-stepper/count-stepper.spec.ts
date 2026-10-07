import { TestBed } from '@angular/core/testing';

import { CountStepper } from './count-stepper';

describe('CountStepper', () => {
  function setup(value: number, min = 1, max = 10) {
    const fixture = TestBed.createComponent(CountStepper);
    fixture.componentRef.setInput('value', value);
    fixture.componentRef.setInput('min', min);
    fixture.componentRef.setInput('max', max);
    fixture.componentRef.setInput('noun', 'Bandido');
    const emitted: number[] = [];
    fixture.componentInstance.valueChange.subscribe((v) => emitted.push(v));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const buttons = () => Array.from(el.querySelectorAll<HTMLButtonElement>('button'));
    return { fixture, el, buttons, emitted };
  }

  it('names its buttons by the noun and says the count', () => {
    const { el, buttons } = setup(3);
    expect(buttons().map((b) => b.getAttribute('aria-label'))).toEqual([
      'Menos um Bandido',
      'Mais um Bandido',
    ]);
    expect(el.querySelector('output')?.textContent).toBe('3');
    expect(el.querySelector('output')?.getAttribute('aria-label')).toBe('3 Bandido');
  });

  it('two quick taps add two even before the parent draws again', () => {
    const { buttons, emitted } = setup(1);
    buttons()[1].click();
    buttons()[1].click();
    expect(emitted).toEqual([2, 3]);
  });

  it('stops at its limits: the button is aria-disabled and does nothing', () => {
    const { buttons, emitted, fixture } = setup(1, 1, 2);
    expect(buttons()[0].getAttribute('aria-disabled')).toBe('true');
    buttons()[0].click();
    buttons()[1].click();
    fixture.detectChanges();
    expect(buttons()[1].getAttribute('aria-disabled')).toBe('true');
    buttons()[1].click();
    expect(emitted).toEqual([2]);
  });

  it('at the lowest value the minus button can mean "take it out", with its own name', () => {
    const { fixture, buttons, emitted } = setup(1, 0, 40);
    fixture.componentRef.setInput('minLabel', 'Tirar Ogro do encontro');
    fixture.detectChanges();
    expect(buttons()[0].getAttribute('aria-label')).toBe('Tirar Ogro do encontro');
    buttons()[0].click();
    expect(emitted).toEqual([0]);
  });

  it("takes its own wording for a noun that is not masculine, with the count in the number's label", () => {
    const { fixture, el, buttons } = setup(3);
    fixture.componentRef.setInput('minusLabel', 'Menos uma peça');
    fixture.componentRef.setInput('plusLabel', 'Mais uma peça');
    fixture.componentRef.setInput('valueLabel', '{n} peças');
    fixture.detectChanges();
    expect(buttons().map((b) => b.getAttribute('aria-label'))).toEqual([
      'Menos uma peça',
      'Mais uma peça',
    ]);
    expect(el.querySelector('output')?.getAttribute('aria-label')).toBe('3 peças');
  });
});
