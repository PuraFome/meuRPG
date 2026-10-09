import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

/**
 * What the owning player of a pending character sees while the master's request
 * for changes is open (RN-15): "O mestre pediu ajustes." as an alert, the reason
 * in quotes and when it was asked, then "Enviar de novo" (the screen's only
 * filled button) and "Editar ficha". It stands in for the "Esperando a aprovação
 * do mestre" strip: two strips would be noise. Editing never sends by itself.
 */
@Component({
  selector: 'app-owner-changes',
  imports: [MatButtonModule, MatIconModule, RouterLink],
  templateUrl: './owner-changes.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
  styleUrl: './owner-changes.scss',
})
export class OwnerChanges {
  readonly reason = input.required<string>();
  /** When the master asked, already written ("8 de out., 21h10"). */
  readonly when = input('');
  readonly busy = input(false);
  readonly error = input<string | null>(null);
  /** The editor's route, or `null` when the sheet can't be edited. */
  readonly editLink = input<readonly string[] | null>(null);
  readonly resubmit = output<void>();
}
