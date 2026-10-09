import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

/**
 * The count pips of a resource box (PM-07b 7): a filled dot for each use left and a ring for each one spent, in the
 * accent. Decorative only (`aria-hidden`): the box around it says the count in words ("2 de 3").
 */
@Component({
  selector: 'app-count-pips',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @for (left of pips(); track $index) {
      <span class="pip" [class.pip--left]="left"></span>
    }
  `,
  styles: `
    :host {
      display: inline-flex;
      flex-wrap: wrap;
      gap: 6px;
    }

    .pip {
      box-sizing: border-box;
      width: 14px;
      height: 14px;
      border: 2px solid var(--mr-accent);
      border-radius: 50%;
    }

    .pip--left {
      background: var(--mr-accent);
    }
  `,
  host: { 'aria-hidden': 'true' },
})
export class CountPips {
  /** The uses left. */
  readonly left = input.required<number>();
  readonly total = input.required<number>();

  protected readonly pips = computed(() =>
    Array.from({ length: Math.max(0, this.total()) }, (_, i) => i < this.left()),
  );
}
