import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { ToggleChips, type ToggleChip } from './toggle-chips';

const CHIPS: ToggleChip[] = [
  { value: 'a', label: 'Acuidade' },
  { value: 'b', label: 'Leve' },
  { value: 'c', label: 'Alcance', locked: true },
];

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ToggleChips],
  template: `<app-toggle-chips label="Propriedades" [chips]="chips" [values]="values()" lockNote="Alcance: só corpo a corpo." path="p.props" (valuesChange)="values.set($event)" />`,
})
class Host {
  readonly chips = CHIPS;
  readonly values = signal<string[]>(['b']);
}

describe('ToggleChips (the weapon properties, board 580-11)', () => {
  function setup() {
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const buttons = () => Array.from(el.querySelectorAll<HTMLButtonElement>('button.tchip'));
    return { fixture, el, buttons, host: fixture.componentInstance };
  }

  it('is a group with a button per chip, pressed when chosen', () => {
    const { el, buttons } = setup();
    expect(el.querySelector('[role=group]')?.getAttribute('aria-label')).toBe('Propriedades');
    expect(buttons().map((b) => b.getAttribute('aria-pressed'))).toEqual(['false', 'true', 'false']);
  });

  it('adds and takes off a chip in the order chosen', () => {
    const { fixture, buttons, host } = setup();
    buttons()[0].click();
    fixture.detectChanges();
    expect(host.values()).toEqual(['b', 'a']);
    buttons()[1].click();
    fixture.detectChanges();
    expect(host.values()).toEqual(['a']);
  });

  it('keeps a locked chip on screen, shut, and gives a screen reader the reason', () => {
    const { fixture, el, buttons, host } = setup();
    const locked = buttons()[2];
    expect(locked.getAttribute('aria-disabled')).toBe('true');
    const note = el.querySelector('.tchips__note');
    expect(note?.textContent).toContain('Alcance: só corpo a corpo.');
    expect(locked.getAttribute('aria-describedby')).toBe(note?.id);
    locked.click();
    fixture.detectChanges();
    expect(host.values()).toEqual(['b']);
  });
});
