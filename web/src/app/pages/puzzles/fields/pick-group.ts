import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/** One choice of a `PickGroup`. */
export interface PickOption<T extends string | number> {
  readonly value: T;
  readonly title: string;
  /** A line under the title (cards only). */
  readonly sub?: string;
  /** A Material Symbols icon before the title (cards only). */
  readonly icon?: string;
}

let nextId = 0;

/**
 * One choice out of a few (the forms of E10-06): native radios, so the arrow keys, the group's name and "marcado" all come
 * for free, drawn three ways.
 *
 * - `cards`: a title, a line and an icon in a bordered card (the kind of puzzle);
 * - `rows`: a round radio and a title in a row (the "Ao resolver" choices);
 * - `segments`: joined boxes with a check on the marked one (the size, the alphabet).
 *
 * The marked one gets the accent border and the soft ground and, in the segments, a check mark: the state is never the color
 * alone. Every target is at least 44 px (48 px on a phone).
 */
@Component({
  selector: 'app-pick-group',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <fieldset class="pg" [class.pg--cards]="layout() === 'cards'" [class.pg--rows]="layout() === 'rows'" [class.pg--segments]="layout() === 'segments'">
      <legend [class.mr-visually-hidden]="hideLegend()" class="pg__legend">{{ legend() }}</legend>
      <div class="pg__options">
        @for (o of options(); track o.value) {
          <label class="opt" [class.opt--on]="o.value === value()">
            <input type="radio" [name]="name" [value]="o.value" [checked]="o.value === value()" [disabled]="disabled()" (change)="valueChange.emit(o.value)" />
            @if (layout() === 'cards' && o.icon) {
              <mat-icon class="opt__icon" aria-hidden="true">{{ o.icon }}</mat-icon>
            }
            @if (layout() === 'rows') {
              <span class="opt__dot" aria-hidden="true"></span>
            }
            @if (layout() === 'segments' && o.value === value()) {
              <mat-icon class="opt__check" aria-hidden="true">check</mat-icon>
            }
            <span class="opt__text">
              <span class="opt__title">{{ o.title }}</span>
              @if (o.sub) {
                <span class="opt__sub">{{ o.sub }}</span>
              }
            </span>
          </label>
        }
      </div>
    </fieldset>
  `,
  styleUrl: './pick-group.scss',
})
export class PickGroup<T extends string | number> {
  protected readonly name = `pick-${nextId++}`;
  readonly legend = input.required<string>();
  readonly hideLegend = input(false);
  readonly options = input.required<readonly PickOption<T>[]>();
  readonly value = input.required<T>();
  readonly layout = input<'cards' | 'rows' | 'segments'>('rows');
  readonly disabled = input(false);
  readonly valueChange = output<T>();
}
