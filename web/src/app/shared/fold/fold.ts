import { ChangeDetectionStrategy, Component, input, model } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

let nextId = 0;

const DEFAULT_LEVEL = 3;

/**
 * The one disclosure of the app: a bordered `.mr-panel--fold` whose header is a 48 px button (`aria-expanded`, the words
 * left, the chevron right) with the body under it. The words stay beside the chevron when folded; a small note
 * (`foldNote`, "Só você vê") sits in the header too. The open state lives here, in memory, never on the device.
 * With `foldable` off the header is a plain title and the body always shows (a wide screen where the same words
 * are a section).
 */
@Component({
  selector: 'app-fold',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    class: 'mr-panel mr-panel--fold',
    '[class.mr-panel--fold-row]': "appearance() === 'row'",
    '[class.mr-panel--fold-flush]': "appearance() === 'flush'",
    '[class.mr-fold--muted]': 'muted()',
    '[attr.role]': "region() ? 'region' : null",
    '[attr.aria-label]': 'region() ? title() : null',
  },
  template: `
    <div class="mr-fold__head" role="heading" [attr.aria-level]="level()">
      @if (foldable()) {
        <button
          type="button"
          class="mr-fold__button"
          [id]="id + '-button'"
          [attr.aria-expanded]="open()"
          [attr.aria-controls]="id + '-body'"
          (click)="open.set(!open())"
        >
          @if (icon()) {
            <mat-icon class="mr-fold__icon" aria-hidden="true">{{ icon() }}</mat-icon>
          }
          <span class="mr-fold__title">{{ title() }}</span>
          <span class="mr-fold__note"><ng-content select="[foldNote]" /></span>
          <mat-icon aria-hidden="true">{{ open() ? 'expand_less' : 'expand_more' }}</mat-icon>
        </button>
      } @else {
        <span class="mr-fold__title">{{ title() }}</span>
        <span class="mr-fold__note"><ng-content select="[foldNote]" /></span>
      }
    </div>
    @if (open() || !foldable()) {
      <div class="mr-fold__body" [id]="id + '-body'">
        <ng-content />
      </div>
    }
  `,
})
export class Fold {
  protected readonly id = `fold-${nextId++}`;
  /** The words of the header. */
  readonly title = input.required<string>();
  /** The heading level of the header (the page's outline), 3 by default. */
  readonly level = input(DEFAULT_LEVEL);
  readonly foldable = input(true);
  /** `section`: a titled block of the page (display type); `row`: a quieter group row (sans type, an icon at the left, a rule under the header when open); `flush`: a fold inside a card, framed by the card (a rule above it). */
  readonly appearance = input<'section' | 'row' | 'flush'>('section');
  /** The row's icon, at the left of its words. */
  readonly icon = input('');
  /** The words in the muted ink: a group that is spent ("Ação bônus · Usada"). */
  readonly muted = input(false);
  /** The panel is a landmark named by its title (a section the person looks for by name). */
  readonly region = input(false);
  /** Open or folded; two-way, so the caller may open it (a notice that points into it). */
  readonly open = model(false);
}
