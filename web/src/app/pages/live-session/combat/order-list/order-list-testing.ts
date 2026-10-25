import type { ComponentFixture } from '@angular/core/testing';

/** Opens every row of an order in place (the specs read what a row holds once it is open). */
export function openAllRows(fixture: ComponentFixture<unknown>): void {
  const el = fixture.nativeElement as HTMLElement;
  el.querySelectorAll<HTMLButtonElement>('.row__head').forEach((head) => head.click());
  fixture.detectChanges();
}
