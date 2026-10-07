import { Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import type { EncounterLine } from '../../../../gen/meurpg/play/v1/encounters_pb';
import type { CreatureSummary } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { formatInt } from '../../../core/format/text';

/**
 * "Trocar criatura" (E10-09 states 4 and 5): the creatures with the same XP as the line's (the server's list, without the ones the
 * encounter already has), one to pick, and "Cancelar" and "Trocar por Orc" the same size. In a dialog it sits under the lines
 * with its own buttons; on a phone it takes the whole sheet and its buttons are the sheet's footer (`footer` false here).
 */
@Component({
  selector: 'app-swap-panel',
  imports: [MatButtonModule],
  template: `
    <section class="swap" aria-labelledby="swap-h">
      <h4 class="swap__t" id="swap-h" tabindex="-1">Trocar {{ line().count }} × {{ line().creature?.namePt }}</h4>
      <p class="swap__s">Só aparecem criaturas com o mesmo XP ({{ format(line().creature?.xp ?? 0) }} cada), então o total não muda. A quantidade fica.</p>
      @if (error()) {
        <p class="swap__err" role="alert">{{ error() }}</p>
      }
      @if (options() === null) {
        <p class="mr-muted" role="status">Procurando as trocas...</p>
      } @else if (options()!.length === 0) {
        <p class="mr-muted">Nenhuma outra criatura tem esse XP{{ typeChosen() ? ' neste tipo' : '' }}. As outras do encontro já estão nele.</p>
      } @else {
        <div class="swap__list" role="radiogroup" aria-labelledby="swap-h">
          @for (c of options(); track c.key) {
            <label class="swap__opt" [class.swap__opt--on]="pick() === c.key">
              <input type="radio" name="swap" [value]="c.key" [checked]="pick() === c.key" (change)="picked.emit(c.key)" />
              <span class="swap__n">{{ c.namePt }}</span>
              <span class="swap__nd">ND {{ c.challengeRating }} · {{ format(c.xp) }} XP cada</span>
            </label>
          }
        </div>
      }
      @if (footer()) {
        <div class="swap__actions">
          <button type="button" mat-stroked-button class="no" (click)="cancel.emit()">Cancelar</button>
          <button type="button" mat-flat-button class="go" [class.mr-button--off]="!pick() || busy()" [disabled]="!pick() || busy()" disabledInteractive (click)="confirm.emit()">
            {{ pickName() ? 'Trocar por ' + pickName() : 'Trocar' }}
          </button>
        </div>
      }
    </section>
  `,
  styleUrl: './swap-panel.scss',
})
export class SwapPanel {
  readonly line = input.required<EncounterLine>();
  /** The creatures it can become; `null` while the server answers. */
  readonly options = input.required<readonly CreatureSummary[] | null>();
  readonly pick = input('');
  readonly pickName = input('');
  readonly error = input('');
  readonly busy = input(false);
  readonly typeChosen = input(false);
  /** The panel draws its own buttons (a dialog); a phone's sheet has them in its footer. */
  readonly footer = input(true);
  readonly picked = output<string>();
  readonly confirm = output<void>();
  readonly cancel = output<void>();
  protected readonly format = formatInt;
}
