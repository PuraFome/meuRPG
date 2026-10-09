import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * The label of a lasting effect or state under the vitals cards (PM-03a): an icon and a sentence in a pill.
 * The tone says which: `shield` (Escudo Arcano) and `accent` (a tag), both in the accent, `aid` (Ajuda, the warning colours), `down`
 * (Inconsciente, the accent filled), `ok` (Acordado, the success colours) and `neutral` (the death save counts).
 * The host carries `effect` and `effect--<tone>`.
 */
@Component({
  selector: 'app-effect-pill',
  imports: [MatIconModule],
  template: `@if (icon(); as name) {<mat-icon aria-hidden="true">{{ name }}</mat-icon>}<ng-content />`,
  styleUrl: './effect-pill.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { '[class]': "'effect effect--' + tone()" },
})
export class EffectPill {
  readonly tone = input<'shield' | 'accent' | 'aid' | 'down' | 'ok' | 'neutral'>('shield');
  readonly icon = input('');
}
