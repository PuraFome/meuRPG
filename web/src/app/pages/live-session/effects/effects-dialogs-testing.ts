import { type Type } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { vi } from 'vitest';

import { EffectsClient } from '../../../core/effects/effects-client';

export const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim();

/** A button the app turned off: a Material button that stays focusable says it with `aria-disabled`. */
export const isOff = (b: HTMLElement | null | undefined) =>
  !b || (b as HTMLButtonElement).disabled || b.getAttribute('aria-disabled') === 'true';

/** Opens a dialog of the effects the way the sheet does: its data, a reference that records what it closes with. */
export function openDialog<C, D>(component: Type<C>, data: D, api: Partial<EffectsClient>) {
  const close = vi.fn();
  TestBed.configureTestingModule({
    providers: [
      { provide: EffectsClient, useValue: api },
      { provide: MatDialogRef, useValue: { close } },
      { provide: MAT_DIALOG_DATA, useValue: data },
    ],
  });
  const fixture = TestBed.createComponent(component);
  document.body.appendChild(fixture.nativeElement);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const settle = async () => {
    await fixture.whenStable();
    fixture.detectChanges();
    await fixture.whenStable();
  };
  const button = (name: string) =>
    Array.from(el.querySelectorAll('button')).find((b) => flat(b)?.endsWith(name)) as
      HTMLButtonElement | undefined;
  /** Picks the option with this label in the select named by its `data-field`. */
  const pick = async (field: string, label: string) => {
    const select = el.querySelector(`select[data-field="${field}"]`) as HTMLSelectElement;
    select.selectedIndex = Array.from(select.options).findIndex((o) => flat(o) === label);
    select.dispatchEvent(new Event('change'));
    await settle();
  };
  /** Types in the input named by its `data-field`. */
  const type = async (field: string, value: string) => {
    const input = el.querySelector(`input[data-field="${field}"]`) as HTMLInputElement;
    input.value = value;
    input.dispatchEvent(new Event('input'));
    await settle();
  };
  const radio = async (label: string) => {
    const choice = Array.from(el.querySelectorAll('label.choice')).find((l) =>
      flat(l)?.startsWith(label),
    );
    (choice?.querySelector('input') as HTMLInputElement).click();
    await settle();
  };
  return {
    fixture,
    component: fixture.componentInstance,
    el,
    close,
    settle,
    button,
    pick,
    type,
    radio,
  };
}
