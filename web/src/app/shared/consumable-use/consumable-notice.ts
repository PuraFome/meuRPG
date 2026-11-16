import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { ConsumableNotice } from '../../core/characters/consumables';

/**
 * What a use of a consumable said (565-03): the card with the heal or the resistance and, for an item the app carries out
 * nothing for, the item's own text in a dashed box under it. The page that shows it keeps the live region (`role="status"`)
 * around it, so the words are announced when they come.
 */
@Component({
  selector: 'app-consumable-notice',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <div class="consumable__result" [class.consumable__result--good]="notice().tone === 'good'">
      <mat-icon aria-hidden="true">{{ notice().icon }}</mat-icon>
      <p>
        <strong>{{ notice().headline }}</strong>
        @if (notice().detail) {
          <span class="consumable__detail">{{ notice().detail }}</span>
        }
      </p>
    </div>
    @for (line of notice().text; track $index) {
      <p class="consumable__text">{{ line }}</p>
    }
  `,
  styleUrl: './consumable-notice.scss',
})
export class ConsumableNoticeCard {
  readonly notice = input.required<ConsumableNotice>();
}
