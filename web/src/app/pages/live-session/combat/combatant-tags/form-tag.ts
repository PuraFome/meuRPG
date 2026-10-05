import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * "Na forma de Lobo" under a druid's name in the orders (MR-037, E9-11): the beast's name goes to everyone who sees the druid
 * (the server sends `wild_shape_beast_name_pt` to the party), a paw and words, never only a colour. The beast's hit points are
 * not here: only the master and the druid's player get them (RN-20), and they have their own place in the row.
 */
@Component({
  selector: 'app-form-tag',
  imports: [MatIconModule],
  template: `<mat-icon aria-hidden="true">pets</mat-icon>Na forma de {{ beast() }}`,
  styles: `
    :host {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      font-size: 13px;
      font-weight: 700;
      line-height: 18px;
      color: var(--mr-ink);
    }

    .mat-icon {
      flex: none;
      width: 15px;
      height: 15px;
      font-size: 15px;
    }
  `,
})
export class FormTag {
  /** The beast's Portuguese name ("Lobo"). */
  readonly beast = input.required<string>();
}

/** The beast's pool in the master's list: "Lobo 11 de 11", above the druid's own, in the same column (the master and the druid's player only). */
@Component({
  selector: 'app-beast-pool',
  template: `<span class="who">{{ beast() }}</span> {{ current() }}<span class="of"> de {{ max() }}</span>`,
  styles: `
    :host {
      display: block;
      margin-bottom: 3px;
      font-size: 15px;
      white-space: nowrap;
    }

    .who {
      font: 500 13px var(--mr-font-sans);
      color: var(--mr-ink-muted);
    }

    .of {
      font: 400 14px var(--mr-font-sans);
      color: var(--mr-ink-muted);
    }
  `,
})
export class BeastPool {
  readonly beast = input.required<string>();
  readonly current = input.required<number>();
  readonly max = input.required<number>();
}
