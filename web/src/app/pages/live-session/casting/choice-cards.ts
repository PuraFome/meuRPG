import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

let nextId = 0;

/** One card of a choice list: what it is, a line under it and, when it cannot be picked, why. */
export interface ChoiceRow {
  readonly id: string;
  readonly title: string;
  /** The line under the title ("1º nível · 1 ação · toque", "a 1,5 m"). */
  readonly detail: string;
  readonly enabled: boolean;
  /** Why a disabled card cannot be picked ("Fora do alcance do toque (1,5 m)."). */
  readonly reason: string;
}

/**
 * A list of cards that is a radio group (one choice) or a group of check boxes (up to a number), as the casting boards
 * draw them: the chosen card has the accent frame and a check, one that cannot be picked is dashed with its reason on its
 * own line. Native inputs under the cards, so the arrows, the space bar and a screen reader work as in any group.
 */
@Component({
  selector: 'app-choice-cards',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    @if (label()) {
      <span class="cap" [id]="id + '-l'">{{ label() }}</span>
    }
    <div class="rows" [attr.role]="mode() === 'radio' ? 'radiogroup' : 'group'" [attr.aria-labelledby]="label() ? id + '-l' : null">
      @for (r of rows(); track r.id) {
        <label class="row" [class.row--off]="!r.enabled">
          <input
            class="mr-visually-hidden"
            [type]="mode() === 'radio' ? 'radio' : 'checkbox'"
            [name]="id"
            [checked]="chosen().includes(r.id)"
            [disabled]="!r.enabled"
            (change)="choose.emit(r.id)"
          />
          <span class="row__text">
            <b class="row__title">{{ r.title }}</b>
            @if (r.detail) {
              <span class="row__detail">{{ r.detail }}</span>
            }
            @if (!r.enabled && r.reason) {
              <span class="row__why"><mat-icon aria-hidden="true">block</mat-icon>{{ r.reason }}</span>
            }
          </span>
          @if (chosen().includes(r.id)) {
            <mat-icon class="row__mark row__mark--on" aria-hidden="true">{{ mode() === 'radio' ? 'check_circle' : 'check_box' }}</mat-icon>
          } @else if (r.enabled) {
            <mat-icon class="row__mark" aria-hidden="true">{{ mode() === 'radio' ? 'radio_button_unchecked' : 'check_box_outline_blank' }}</mat-icon>
          }
        </label>
      }
    </div>
  `,
  styleUrl: './choice-cards.scss',
})
export class ChoiceCards {
  readonly rows = input.required<readonly ChoiceRow[]>();
  readonly chosen = input<readonly string[]>([]);
  readonly mode = input<'radio' | 'check'>('radio');
  readonly label = input('');
  readonly choose = output<string>();

  protected readonly id = `choice-cards-${nextId++}`;
}
