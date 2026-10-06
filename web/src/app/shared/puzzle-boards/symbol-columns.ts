import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { SymbolFace } from '../../core/puzzles/puzzle-symbols';
import { SymbolGlyph } from './symbol-glyph';

/** A wheel or a pillar the person turned: from 0, and `+1` (the next face) or `-1` (the previous). */
export interface Turn {
  readonly index: number;
  readonly delta: 1 | -1;
}

/**
 * The columns the lock and the pillars share: one column per wheel or pillar, each a face (our drawing and its name, always
 * written) and, by `mode`, the way to turn it.
 *
 * - `play`: a wheel has an arrow above and one below (the next and the previous face); a pillar has "Girar", a 48 px button
 *   under its name, since pillars only turn forward.
 * - `edit`: arrows on both (the master chooses a solution, a start or a mural).
 * - `view`: the faces alone (the master's live panel, a start preview). Nothing in it is a control.
 *
 * `changed` draws a dashed frame on the columns the last move turned. 48 px arrows and buttons, and a column never narrower
 * than 44 px at 320 px.
 */
@Component({
  selector: 'app-symbol-columns',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule, SymbolGlyph],
  host: { '[style.--n]': 'positions().length', '[style.--per]': 'perRow()', '[class.sc--pillar]': "kind() === 'pillar'" },
  template: `
    <div class="cols" role="group" [attr.aria-label]="groupLabel()" [attr.aria-disabled]="disabled() ? 'true' : null">
      @for (at of positions(); track $index) {
        @let face = faces()[at];
        @let n = $index + 1;
        <div class="col" role="group" [class.col--changed]="changedSet().has($index)" [attr.aria-label]="noun() + ' ' + n + ': ' + (face?.namePt ?? '')">
          @if (kind() === 'pillar' && mode() === 'play') {
            <span class="head" aria-hidden="true">Pilar {{ n }}</span>
          }
          @if (arrows()) {
            <button type="button" class="step" [attr.aria-label]="'Próximo símbolo: ' + noun() + ' ' + n" [attr.aria-disabled]="disabled() ? 'true' : null" (click)="!disabled() && turn.emit({ index: $index, delta: 1 })">
              <mat-icon aria-hidden="true">keyboard_arrow_up</mat-icon>
            </button>
          }
          <span class="face" aria-hidden="true">
            @if (face) {
              <app-symbol-glyph [face]="face.key" />
              @if (face.key !== face.namePt) {
                <span class="name">{{ face.namePt }}</span>
              }
            }
          </span>
          @if (arrows()) {
            <button type="button" class="step" [attr.aria-label]="'Símbolo anterior: ' + noun() + ' ' + n" [attr.aria-disabled]="disabled() ? 'true' : null" (click)="!disabled() && turn.emit({ index: $index, delta: -1 })">
              <mat-icon aria-hidden="true">keyboard_arrow_down</mat-icon>
            </button>
          }
          @if (mode() === 'play' && kind() === 'pillar') {
            <button type="button" class="turn" [attr.aria-label]="'Girar o pilar ' + n" [attr.aria-disabled]="disabled() ? 'true' : null" (click)="!disabled() && turn.emit({ index: $index, delta: 1 })">Girar</button>
          }
        </div>
      }
    </div>
  `,
  styleUrl: './symbol-columns.scss',
})
export class SymbolColumns {
  readonly kind = input<'wheel' | 'pillar'>('wheel');
  /** Where each wheel or pillar stands: an index into `faces`. */
  readonly positions = input.required<readonly number[]>();
  readonly faces = input.required<readonly SymbolFace[]>();
  readonly mode = input<'play' | 'view' | 'edit'>('view');
  readonly changed = input<readonly number[]>([]);
  /** Solved or stopped: nothing turns any more. */
  readonly disabled = input(false);
  /** The name of the whole group for a screen reader: "Rodas da fechadura". */
  readonly label = input('');

  readonly turn = output<Turn>();

  /** Columns in a row on a phone under 375 px: more than 4 go in two rows, so every target keeps its 44 px. */
  protected readonly perRow = computed(() => (this.positions().length > 4 ? Math.ceil(this.positions().length / 2) : this.positions().length));
  protected readonly noun = computed(() => (this.kind() === 'pillar' ? 'Pilar' : 'Roda'));
  protected readonly groupLabel = computed(() => this.label() || (this.kind() === 'pillar' ? 'Pilares' : 'Rodas'));
  protected readonly changedSet = computed(() => new Set(this.changed()));
  /** Arrows above and below: a wheel in `play` and `edit`, a pillar in `edit` only (it has "Girar" in `play`). */
  protected readonly arrows = computed(() => this.mode() === 'edit' || (this.mode() === 'play' && this.kind() === 'wheel'));
}
