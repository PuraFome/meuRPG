import { NgTemplateOutlet } from '@angular/common';
import { Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

/**
 * The two buttons of an XP screen: the one filled action ("Dar 116 XP a cada
 * um", "Registrar marco") and the way out ("Agora não", "Cancelar"). They are
 * always two real buttons of one size: 44px side by side on a desktop, 48px
 * and the full width, stacked, on a phone; a pair that does not fit the line
 * stacks instead of wrapping a label. `secondaryStyle="text"` is the phone
 * sheet's "Cancelar", a text button under the filled one.
 *
 * While the action cannot go (nobody checked, a field missing) the filled
 * button turns dashed, as in the start-combat dialog, stays reachable by
 * keyboard (`disabledInteractive`) and `reason` says why in words, linked to
 * it with `aria-describedby`. A click on it still reaches the screen
 * (`primary`), which marks the fields and moves focus to the first wrong one.
 */
@Component({
  selector: 'app-xp-actions',
  imports: [MatButtonModule, NgTemplateOutlet],
  template: `
    @if (reason()) {
      <p class="reason" [id]="reasonId">{{ reason() }}</p>
    }
    <div class="pair" [class.pair--end]="align() === 'end'">
      @if (primaryFirst()) {
        <ng-container [ngTemplateOutlet]="main" />
        <ng-container [ngTemplateOutlet]="other" />
      } @else {
        <ng-container [ngTemplateOutlet]="other" />
        <ng-container [ngTemplateOutlet]="main" />
      }
    </div>

    <ng-template #main>
      <button
        mat-flat-button
        type="button"
        class="btn primary"
        [class.primary--off]="blocked()"
        [aria-disabled]="blocked() || busy()"
        [attr.aria-describedby]="reason() ? reasonId : null"
        (click)="primary.emit()"
      >
        {{ primaryLabel() }}
      </button>
    </ng-template>
    <ng-template #other>
      @if (secondaryStyle() === 'text') {
        <button mat-button type="button" class="btn secondary" (click)="secondary.emit()">
          {{ secondaryLabel() }}
        </button>
      } @else {
        <button mat-stroked-button type="button" class="btn secondary" (click)="secondary.emit()">
          {{ secondaryLabel() }}
        </button>
      }
    </ng-template>
  `,
  styleUrl: './xp-actions.scss',
})
export class XpActions {
  readonly primaryLabel = input.required<string>();
  readonly secondaryLabel = input.required<string>();
  /** The action cannot go yet: the button is dashed and `reason` says why. */
  readonly blocked = input(false);
  /** A call is in flight: nothing more to press. */
  readonly busy = input(false);
  readonly reason = input('');
  readonly secondaryStyle = input<'outlined' | 'text'>('outlined');
  /** Where the filled button is: first (a card, "Dar 116 XP" then "Agora não") or last
   * (a dialog's footer, "Cancelar" then the action). On a phone it is always first. */
  readonly primaryFirst = input(false);
  /** `end`: the pair sits at the right (a dialog's footer); `start`: at the left (a card). */
  readonly align = input<'start' | 'end'>('start');

  readonly primary = output<void>();
  readonly secondary = output<void>();

  /** Unique per instance: two blocks may be on the page. */
  protected readonly reasonId = `xp-actions-reason-${nextId++}`;
}

let nextId = 0;
