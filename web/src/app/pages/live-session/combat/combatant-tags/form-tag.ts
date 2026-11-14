import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * "Na forma de Lobo" under a druid's name in the orders (MR-037, E9-11): the beast's name goes to everyone who sees the druid
 * (the server sends `wild_shape_beast_name_pt` to the party), a paw and words, never only a colour. The beast's hit points are
 * not here: only the master and the druid's player get them (RN-20), and they have their own place in the row.
 */
@Component({
  selector: 'app-form-tag',
  imports: [MatIconModule],
  template: `<mat-icon aria-hidden="true">pets</mat-icon>{{ named() ? beast() : 'Na forma de ' + beast() }}@if (short(); as t) { · <mat-icon class="hour" aria-hidden="true">hourglass_empty</mat-icon>{{ t }} }`,
  styles: `
    :host {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      font-size: var(--mr-font-13);
      font-weight: 700;
      line-height: 18px;
      color: var(--mr-ink);
    }

    .mat-icon {
      flex: none;
      width: 15px;
      height: 15px;
      font-size: var(--mr-font-15);
    }
  `,
})
export class FormTag {
  /** The beast's Portuguese name ("Lobo"). */
  readonly beast = input.required<string>();
  /** The time left as the server writes it ("52 min restantes"): only the master and the druid's player have it. */
  readonly time = input('');
  /** The opened row of the master, which already says it is a form: the beast's name and the time in full ("Lobo · 52 min restantes"). */
  readonly named = input(false, { transform: (v: boolean | string) => v === '' || v === true });

  protected readonly short = computed(() =>
    this.named()
      ? this.time()
      : this.time()
          .replace(/ restantes$/, '')
          .replace(/^Restam /, ''),
  );
}
