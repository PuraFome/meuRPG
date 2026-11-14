import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * A line of a sheet with an icon before its words: the copies a weapon has, the reason a bonus does not count.
 * The words are the content; `muted` quiets them.
 */
@Component({
  selector: 'app-sheet-note',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `<mat-icon aria-hidden="true">{{ icon() }}</mat-icon>
    <span><ng-content /></span>`,
  styleUrl: './sheet-note.scss',
  host: { '[class.sheet-note--muted]': 'muted()' },
})
export class SheetNote {
  /** The name of the icon in the icon font. */
  readonly icon = input.required<string>();
  readonly muted = input(true);
}
