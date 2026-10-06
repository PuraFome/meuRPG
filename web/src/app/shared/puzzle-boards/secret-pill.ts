import { ChangeDetectionStrategy, Component } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * "Só você vê": the pill beside what only the master reads (a lock's solution, the hint rings). The crossed eye and the words
 * say it together; a master who shares his screen sees it before the players do.
 */
@Component({
  selector: 'app-secret-pill',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `<mat-icon aria-hidden="true">visibility_off</mat-icon>Só você vê`,
  styles: `
    :host {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      box-sizing: border-box;
      height: 26px;
      padding: 0 10px 0 6px;
      border: 1px solid var(--mr-control-line);
      border-radius: var(--mr-radius-pill);
      background: var(--mr-surface);
      color: var(--mr-ink-muted);
      font-size: 14px;
      line-height: 1;
      white-space: nowrap;
    }

    mat-icon {
      width: 18px;
      height: 18px;
      font-size: 18px;
    }
  `,
})
export class SecretPill {}
