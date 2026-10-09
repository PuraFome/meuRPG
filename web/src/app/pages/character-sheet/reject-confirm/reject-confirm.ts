import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

/**
 * "Recusar Lyra?": the question under the approval notice before the master deletes a pending
 * character for good. "Confirmar recusa" is outlined in the danger colour (the screen keeps its one
 * filled button) and the parent puts the focus on "Cancelar".
 */
@Component({
  selector: 'app-reject-confirm',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './reject-confirm.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
  styleUrl: './reject-confirm.scss',
})
export class RejectConfirm {
  readonly characterName = input.required<string>();
  readonly playerName = input.required<string>();
  readonly busy = input(false);
  readonly confirmed = output<void>();
  readonly cancelled = output<void>();
}
