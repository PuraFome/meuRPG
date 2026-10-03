import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * The state word of a group header (timeline.md, shared decision 1): an open
 * circle and "Disponível", or a filled circle with ✕ and "Usada" (dashed and
 * muted). A pill on a phone, a plain word from 1024px. `icon` replaces the
 * circle for the movement ("Restam 7,5 m").
 */
@Component({
  selector: 'app-group-state',
  imports: [MatIconModule],
  template: `
    @if (icon()) {
      <mat-icon aria-hidden="true">{{ icon() }}</mat-icon>
    } @else {
      <span class="dot" aria-hidden="true">
        @if (used()) {<mat-icon>close</mat-icon>}
      </span>
    }{{ word() }}
  `,
  styleUrl: './group-state.scss',
  host: { '[class.used]': 'used()' },
})
export class GroupState {
  readonly word = input.required<string>();
  readonly used = input(false);
  readonly icon = input('');
}
