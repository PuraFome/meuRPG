import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * The notice the owning player of a pending character reads while the master's
 * request for changes is open (RN-15): "O mestre pediu ajustes.", the reason in quotes and when it was asked.
 * It has no live role: the screen puts it in a status region that is already on the page.
 * The buttons are in `app-owner-resubmit`, outside that region.
 */
@Component({
  selector: 'app-owner-changes',
  imports: [MatIconModule],
  templateUrl: './owner-changes.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
  styleUrl: './owner-changes.scss',
})
export class OwnerChanges {
  readonly reason = input.required<string>();
  /** When the master asked, already written ("8 de out., 21h10"). */
  readonly when = input('');
}
