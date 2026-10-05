import { ChangeDetectionStrategy, Component, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * "Ajustar a escolha" (E9-05): four arrows, 44 px each, that move the chosen
 * square one over. A square is 30 px on a phone, too small to hit with a thumb
 * every time, so the arrows finish what the tap started (the map also takes
 * the arrow keys).
 */
@Component({
  selector: 'app-move-adjust',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="adj" role="group" aria-labelledby="adjust-cap">
      <span class="adj__cap" id="adjust-cap">Ajustar a escolha</span>
      <span class="adj__btns">
        @for (a of arrows; track a.key) {
          <button type="button" class="adj__btn" [attr.aria-label]="a.label" (click)="nudge.emit(a.key)">
            <mat-icon aria-hidden="true">{{ a.icon }}</mat-icon>
          </button>
        }
      </span>
    </div>
  `,
  styleUrl: './move-adjust.scss',
})
export class MoveAdjust {
  /** The arrow key that stands for the button pressed. */
  readonly nudge = output<'ArrowLeft' | 'ArrowUp' | 'ArrowDown' | 'ArrowRight'>();
  protected readonly arrows = [
    { key: 'ArrowLeft', label: 'Um quadrado para a esquerda', icon: 'arrow_back' },
    { key: 'ArrowUp', label: 'Um quadrado para cima', icon: 'arrow_upward' },
    { key: 'ArrowDown', label: 'Um quadrado para baixo', icon: 'arrow_downward' },
    { key: 'ArrowRight', label: 'Um quadrado para a direita', icon: 'arrow_forward' },
  ] as const;
}
