import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

import type { SymbolFace } from '../../core/puzzles/puzzle-symbols';
import { SymbolColumns, type Turn } from './symbol-columns';

/**
 * The turning symbols' pillars (MR-038, E10-06): 3 to 6 pillars of our own glyphs. A player turns a pillar with "Girar"
 * and the pillars linked to it turn with it (the rule is written under the board); the master chooses the mural with the
 * arrows. The same pillars, in `view`, draw the mural a player copies and the master's live panel.
 */
@Component({
  selector: 'app-pillars-board',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [SymbolColumns],
  template: `<app-symbol-columns kind="pillar" [positions]="pillars()" [faces]="faces()" [mode]="mode()" [changed]="changed()" [disabled]="disabled()" [label]="label()" (turn)="turn.emit($event)" />`,
  styles: ':host { display: block; }',
})
export class PillarsBoard {
  /** The position of each pillar among the glyphs, from 0. */
  readonly pillars = input.required<readonly number[]>();
  readonly faces = input.required<readonly SymbolFace[]>();
  readonly mode = input<'play' | 'view' | 'edit'>('view');
  readonly changed = input<readonly number[]>([]);
  readonly disabled = input(false);
  readonly label = input('Pilares');

  readonly turn = output<Turn>();
}
