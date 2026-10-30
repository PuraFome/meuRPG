import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { OptionCard } from './option-card';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [OptionCard],
  template: `
    <app-option-card
      [kind]="kind()"
      name="grp"
      title="Fogo"
      sub="Queima"
      cost="1 uso"
      [why]="why()"
      [checked]="on()"
      (chosen)="count.set(count() + 1)"
    />
  `,
})
class Host {
  readonly kind = signal<'checkbox' | 'radio' | 'fixed'>('checkbox');
  readonly why = signal('');
  readonly on = signal(false);
  readonly count = signal(0);
}

function mount() {
  const f = TestBed.createComponent(Host);
  f.detectChanges();
  const el = f.nativeElement as HTMLElement;
  return { f, el, input: () => el.querySelector('input') };
}

describe('OptionCard', () => {
  it('is a checkbox that is hidden but focusable, inside the whole card as its label', () => {
    const { el, input } = mount();
    expect(input()?.type).toBe('checkbox');
    expect(input()?.classList.contains('mr-visually-hidden')).toBe(true);
    expect(input()?.closest('label')?.classList.contains('opt')).toBe(true);
    expect(el.textContent).toContain('1 uso');
  });

  it('is a radio of its group when asked', () => {
    const { f, input } = mount();
    f.componentInstance.kind.set('radio');
    f.detectChanges();
    expect(input()?.type).toBe('radio');
    expect(input()?.name).toBe('grp');
  });

  it('tells when it is ticked and says it', () => {
    const { f, el } = mount();
    f.componentInstance.on.set(true);
    f.detectChanges();
    expect(el.querySelector('.opt--on')).not.toBeNull();
    expect(el.querySelector('input')?.checked).toBe(true);
  });

  it('emits when chosen', () => {
    const { f, input } = mount();
    input()?.click();
    expect(f.componentInstance.count()).toBe(1);
  });

  it('is off, with the reason written, when it cannot be taken', () => {
    const { f, el, input } = mount();
    f.componentInstance.why.set('Sem usos');
    f.detectChanges();
    expect(input()?.disabled).toBe(true);
    expect(el.querySelector('.opt--off')?.textContent).toContain('Sem usos');
  });

  it('is only told, with no input, when it is fixed', () => {
    const { f, input } = mount();
    f.componentInstance.kind.set('fixed');
    f.detectChanges();
    expect(input()).toBeNull();
  });

  it('points the input at its sub line', () => {
    const { input, el } = mount();
    const id = input()?.getAttribute('aria-describedby') ?? '';
    expect(el.querySelector(`#${id}`)?.textContent).toContain('Queima');
  });
});
