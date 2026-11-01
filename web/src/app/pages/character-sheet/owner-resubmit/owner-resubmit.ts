import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

/**
 * "Enviar de novo" (the screen's only filled button) and "Editar ficha" for the owner whose sheet came back (RN-15).
 * A refusal from the server is its own alert.
 * Editing never sends by itself.
 */
@Component({
  selector: 'app-owner-resubmit',
  imports: [MatButtonModule, MatIconModule, RouterLink],
  templateUrl: './owner-resubmit.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
  styleUrl: './owner-resubmit.scss',
})
export class OwnerResubmit {
  readonly busy = input(false);
  readonly error = input<string | null>(null);
  /** The editor's route, or `null` when the sheet can't be edited. */
  readonly editLink = input<readonly string[] | null>(null);
  readonly resubmit = output<void>();
}
