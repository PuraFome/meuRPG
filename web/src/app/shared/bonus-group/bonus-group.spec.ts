import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { BonusGroup } from './bonus-group';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [BonusGroup],
  template: `<app-bonus-group [value]="value()" [disabled]="disabled()" (valueChange)="got.push($event)" />`,
})
class Host {
  readonly value = signal(1);
  readonly disabled = signal(false);
  readonly got: number[] = [];
}

function setup() {
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const radios = () => Array.from(el.querySelectorAll<HTMLInputElement>('input[type=radio]'));
  return { fixture, el, radios, host: fixture.componentInstance };
}

describe('BonusGroup', () => {
  it('is a group named "Bônus mágico" with four radios', () => {
    const { el, radios } = setup();
    expect(el.querySelector('fieldset legend')!.textContent).toBe('Bônus mágico');
    expect(
      Array.from(el.querySelectorAll('label')).map((l) =>
        l.textContent!.replace(/\s+/g, ' ').trim(),
      ),
    ).toEqual(['Nenhum', 'check+1', '+2', '+3']);
    expect(radios().map((r) => r.checked)).toEqual([false, true, false, false]);
    expect(new Set(radios().map((r) => r.name)).size).toBe(1);
  });

  it('marks the chosen one with a check and follows the value', () => {
    const { fixture, el, host } = setup();
    expect(el.querySelectorAll('mat-icon')).toHaveLength(1);
    host.value.set(0);
    fixture.detectChanges();
    expect(el.querySelector('.bg__opt--on')!.textContent).toContain('Nenhum');
    expect(el.querySelectorAll('mat-icon')).toHaveLength(1);
  });

  it('says the value of the radio that changes', () => {
    const { radios, host } = setup();
    radios()[3].dispatchEvent(new Event('change'));
    radios()[0].dispatchEvent(new Event('change'));
    expect(host.got).toEqual([3, 0]);
  });

  it('is inert while disabled', () => {
    const { fixture, el, host } = setup();
    host.disabled.set(true);
    fixture.detectChanges();
    expect(el.querySelector('fieldset')!.disabled).toBe(true);
  });
});
