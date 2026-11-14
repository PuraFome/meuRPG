import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

/**
 * The foot of a sheet that saves one thing: "Salvar", with "Cancelar" beside it in a dialog, and the line saying who can change it.
 * On a phone "Salvar" is alone and full width; in a dialog the two are the same size.
 * While `busy` the button is a muted "Salvando…" with a turning ring and "Cancelar" is off.
 */
@Component({
  selector: 'app-sheet-save-foot',
  imports: [MatButtonModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './sheet-save-foot.html',
  styleUrl: './sheet-save-foot.scss',
})
export class SheetSaveFoot {
  /** The sheet is a bottom sheet on a phone: no "Cancelar", its ✕ closes it. */
  readonly inSheet = input(false);
  readonly busy = input(false);
  /** The line with the lock: who changes this, and until when. */
  readonly lock = input.required<string>();
  readonly saved = output<void>();
  readonly cancelled = output<void>();
}
