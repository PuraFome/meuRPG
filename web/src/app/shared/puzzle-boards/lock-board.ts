import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

import type { SymbolFace } from '../../core/puzzles/puzzle-symbols';
import { SymbolColumns, type Turn } from './symbol-columns';

/**
 * The combination lock's wheels (MR-038, E10-06): 2 to 6 wheels of digits, letters or our own runes, each turned a face
 * at a time with the arrows above and below it. The player never reads the lock's solution (RN-10); the master reads it only
 * when he asks, and draws it with the same wheels in `view`.
 */
@Component({
  selector: 'app-lock-board',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [SymbolColumns],
  template: `<app-symbol-columns kind="wheel" [positions]="wheels()" [faces]="faces()" [mode]="mode()" [changed]="changed()" [disabled]="disabled()" [label]="label()" (turn)="turn.emit($event)" />`,
  styles: ':host { display: block; }',
})
export class LockBoard {
  /** The position of each wheel in the alphabet, from 0. */
  readonly wheels = input.required<readonly number[]>();
  readonly faces = input.required<readonly SymbolFace[]>();
  readonly mode = input<'play' | 'view' | 'edit'>('view');
  readonly changed = input<readonly number[]>([]);
  readonly disabled = input(false);
  readonly label = input('Rodas da fechadura');

  readonly turn = output<Turn>();
}
