import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { CheckRow } from './check-row';
import { FieldNote } from './field-note';
import { NumberStepper } from './number-stepper';
import { PickList } from './pick-list';
import { SelectField, type SelectOption } from './select-field';
import { SwitchField } from './switch-field';
import { TextField } from './text-field';

@Component({
  imports: [CheckRow, FieldNote, NumberStepper, PickList, SelectField, SwitchField, TextField],
  template: `
    <app-text-field label="Nome" [value]="name()" path="table_spell.name_pt" [issues]="issues()" hint="Dica." (valueChange)="name.set($event)" />
    <app-select-field label="Escola" [options]="options" [value]="school()" path="table_spell.school_key" (valueChange)="school.set($event)" />
    <app-pick-list label="Idiomas" addLabel="Adicionar idioma" [options]="options" [values]="langs()" path="table_race.languages" (added)="langs.set([...langs(), $event])" (removed)="langs.set(langs().filter((k) => k !== $event))" />
    <app-switch-field label="Ritual" [checked]="on()" (toggled)="on.set($event)" />
    <app-check-row label="V" [checked]="on()" (toggled)="on.set($event)" />
    <app-number-stepper label="Sabedoria" [value]="n()" [min]="-4" [max]="4" (valueChange)="n.set($event)" />
    <app-field-note id="note" [issues]="issues()" />
  `,
})
class Host {
  readonly name = signal('Lâmina');
  readonly school = signal('a');
  readonly langs = signal<string[]>(['a']);
  readonly on = signal(false);
  readonly n = signal(0);
  readonly issues = signal<string[]>([]);
  readonly options: SelectOption[] = [
    { value: 'a', label: 'Alfa' },
    { value: 'b', label: 'Beta' },
    { value: 'c', label: 'Gama' },
  ];
}

describe('form fields', () => {
  let fixture: ReturnType<typeof TestBed.createComponent<Host>>;
  let el: HTMLElement;
  const host = () => fixture.componentInstance;
  const text = (e: Element) => (e.textContent ?? '').replace(/\u00a0/g, ' ').replace(/\s+/g, ' ');

  beforeEach(() => {
    TestBed.resetTestingModule();
    fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    el = fixture.nativeElement as HTMLElement;
  });

  it('text field: carries its path as data-field, says what was typed and shows the hint', () => {
    const input = el.querySelector<HTMLInputElement>('[data-field="table_spell.name_pt"]')!;
    expect(input.value).toBe('Lâmina');
    input.value = 'Sopro';
    input.dispatchEvent(new Event('input'));
    expect(host().name()).toBe('Sopro');
    expect(text(el.querySelector('app-text-field')!)).toContain('Dica.');
  });

  it('text field: a refusal replaces the hint, marks the input invalid and is described by one id for every message', () => {
    host().issues.set(['Primeira.', 'Segunda.']);
    fixture.detectChanges();
    const field = el.querySelector('app-text-field')!;
    const input = field.querySelector('input')!;
    expect(input.getAttribute('aria-invalid')).toBe('true');
    expect(text(field)).toContain('Primeira.');
    expect(text(field)).toContain('Segunda.');
    expect(text(field)).not.toContain('Dica.');
    const group = field.querySelector('app-field-note > div')!;
    expect(group.id).toBe(input.getAttribute('aria-describedby'));
    expect(group.querySelectorAll('p')).toHaveLength(2);
    // The ids of two fields never collide.
    const ids = Array.from(el.querySelectorAll('[id^="fn-"]')).map((e) => e.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('select field: gives back the value of the option, not its text', () => {
    const select = el.querySelector<HTMLSelectElement>('[data-field="table_spell.school_key"]')!;
    select.selectedIndex = 2;
    select.dispatchEvent(new Event('change'));
    expect(host().school()).toBe('c');
  });

  it('pick list: chips with a 44 px remove button, and a select for the values still free', () => {
    const list = el.querySelector('app-pick-list')!;
    expect(text(list)).toContain('Alfa');
    const add = list.querySelector('select')!;
    expect(Array.from(add.options).map((o) => o.text.trim())).toEqual(['', 'Beta', 'Gama']);
    add.selectedIndex = 1;
    add.dispatchEvent(new Event('change'));
    expect(host().langs()).toEqual(['a', 'b']);
    fixture.detectChanges();
    const x = list.querySelector<HTMLButtonElement>('.chip__x')!;
    expect(x.getAttribute('aria-label')).toBe('Tirar Alfa');
    const box = getComputedStyle(x);
    expect(box.width).toBe('44px');
    expect(box.height).toBe('44px');
    x.click();
    expect(host().langs()).toEqual(['b']);
  });

  it('switch: a real switch with the state in a word', () => {
    const sw = el.querySelector<HTMLButtonElement>('button[role="switch"]')!;
    expect(sw.getAttribute('aria-checked')).toBe('false');
    expect(text(el.querySelector('app-switch-field')!)).toContain('Desligado');
    sw.click();
    fixture.detectChanges();
    expect(host().on()).toBe(true);
    expect(text(el.querySelector('app-switch-field')!)).toContain('Ligado');
  });

  it('check row: a checkbox under the words', () => {
    const box = el.querySelector<HTMLInputElement>('app-check-row input')!;
    box.click();
    expect(host().on()).toBe(true);
  });

  it('stepper: minus and plus stop at the limits and the value reads with its sign', () => {
    const stepper = el.querySelector('app-number-stepper')!;
    const [minus, plus] = Array.from(stepper.querySelectorAll<HTMLButtonElement>('button'));
    plus.click();
    expect(host().n()).toBe(1);
    host().n.set(4);
    fixture.detectChanges();
    expect(plus.disabled).toBe(true);
    expect(text(stepper)).toContain('+4');
    host().n.set(-2);
    fixture.detectChanges();
    expect(text(stepper)).toContain('−2');
    minus.click();
    expect(host().n()).toBe(-3);
  });
});
