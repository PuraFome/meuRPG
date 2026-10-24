import { ChangeDetectionStrategy, Component, input, model } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * The head of a panel on the campaign page (boards C2-01, C2-02, C6-02, C6-03 and C6-04): a Material
 * Symbol in the muted ink, the panel's title (the page's `h2`), then, at the end, a quiet count
 * ("2 personagens") and whatever is projected (the panel's actions: `app-panel-action`, "Dar XP").
 * The icon is decoration: the title says the same in words.
 *
 * On a phone a panel may fold to this one row (`foldable`): the title and its count become the
 * button that opens the panel in place (`aria-expanded`), the projected content (a pill, an
 * action) stays beside it, and the whole row is the touch target.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-panel-head',
  imports: [MatIconModule],
  template: `
    <mat-icon class="head__icon" [class.head__icon--folded]="foldable()" aria-hidden="true">{{
      icon()
    }}</mat-icon>
    <h2 class="mr-panel__title head__title" [id]="titleId()">
      @if (foldable()) {
        <button
          type="button"
          class="head__fold"
          [attr.aria-expanded]="open()"
          (click)="open.set(!open())"
        >
          {{ title() }}@if (count()) {<span class="head__sum"> · {{ count() }}</span>}
        </button>
      } @else {
        {{ title() }}
      }
    </h2>
    @if (count() && !foldable()) {
      <span class="head__count">{{ count() }}</span>
    }
    <ng-content />
    @if (foldable() && chevron()) {
      <mat-icon class="head__chevron" aria-hidden="true">{{
        open() ? 'expand_less' : 'chevron_right'
      }}</mat-icon>
    }
  `,
  styleUrl: './panel-head.scss',
})
export class PanelHead {
  /** The Material Symbols name of the icon. */
  readonly icon = input.required<string>();
  readonly title = input.required<string>();
  /** The `id` of the `h2`, for the panel's `aria-labelledby`. */
  readonly titleId = input.required<string>();
  /** "2 personagens": empty draws nothing. */
  readonly count = input('');
  /** The head is a button that folds the panel (a phone's rows). */
  readonly foldable = input(false);
  /** Whether a folded row ends with its chevron (a row with its own action says its way without one). */
  readonly chevron = input(true);
  /** Whether a foldable panel is open. */
  readonly open = model(true);
}
