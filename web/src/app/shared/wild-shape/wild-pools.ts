import { Component, computed, input } from '@angular/core';

/** One pool of hit points: its label, the number and the bar. */
export interface Pool {
  readonly label: string;
  readonly current: number;
  readonly max: number;
}

/**
 * The two reserves of hit points of a druid in a beast form (MR-037, E9-11 states 3 and 4): the beast's and
 * the druid's, side by side ("11 de 11" and "38 de 38", each with its label and a bar); one over the other on
 * a phone narrower than 360 px. Only the druid's player and the master get these numbers (RN-20).
 */
@Component({
  selector: 'app-wild-pools',
  template: `
    @for (p of pools(); track p.label) {
      <div class="pool" role="group" [attr.aria-label]="p.label + ': ' + p.current + ' de ' + p.max">
        <span class="pool__label">{{ p.label }}</span>
        <span class="pool__num">{{ p.current }} <small>de {{ p.max }}</small></span>
        <span class="pool__bar" aria-hidden="true"><span class="pool__fill" [style.width.%]="percent(p)"></span></span>
      </div>
    }
  `,
  styles: `
    :host {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 10px;
      // Two pools never stretch across a wide column: about 220 px each.
      max-width: 456px;
      margin: 0;

      @media (max-width: 359.98px) {
        grid-template-columns: minmax(0, 1fr);
      }
    }

    .pool {
      display: flex;
      flex-direction: column;
      gap: 4px;
      box-sizing: border-box;
      padding: 10px 12px;
      border: 1px solid var(--mr-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-surface);
    }

    .pool__label {
      font-size: 13px;
      font-weight: 700;
      line-height: 17px;
      color: var(--mr-ink-muted);
    }

    .pool__num {
      font-family: var(--mr-font-display);
      font-size: 26px;
      font-weight: 800;
      line-height: 30px;

      small {
        font-family: var(--mr-font-sans);
        font-size: 14px;
        font-weight: 500;
        color: var(--mr-ink-muted);
      }
    }

    .pool__bar {
      display: block;
      height: 6px;
      border-radius: var(--mr-radius-pill);
      background: var(--mr-line);
      overflow: hidden;
    }

    .pool__fill {
      display: block;
      height: 100%;
      border-radius: var(--mr-radius-pill);
      background: var(--mr-ink);
    }
  `,
})
export class WildPools {
  /** The beast's pool, then the character's. */
  readonly beast = input.required<Pool>();
  readonly character = input.required<Pool>();
  protected readonly pools = computed(() => [this.beast(), this.character()]);

  protected percent(p: Pool): number {
    return p.max > 0 ? Math.max(0, Math.min(100, (p.current / p.max) * 100)) : 0;
  }
}
