import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { TableOptionRow } from '../../../../core/combat/table-cast-options';

let nextId = 0;

/**
 * "Opções das suas características" on the cast sheet (MR-025): the cast options the character's table features offer for
 * this spell. An option that applies by itself is told, not asked ("Sempre ativa"); the others are real checkboxes with
 * their cost in uses, and an option that swaps the damage type asks, once ticked, which type the damage becomes. One the
 * caster may not tick now stays grey and dashed with the server's reason written. Space marks.
 */
@Component({
  selector: 'app-table-option-picker',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <section class="to" [attr.aria-labelledby]="id + '-t'">
      <h3 class="to__title" [id]="id + '-t'">Opções das suas características</h3>
      @for (r of rows(); track r.key) {
        @if (r.alwaysOn) {
          <div class="opt opt--on opt--fixed">
            <mat-icon class="opt__mark" aria-hidden="true">check_circle</mat-icon>
            <span class="opt__text">
              <span class="opt__head">
                <b>{{ r.name }}</b>
                <span class="opt__cost">Sempre ativa</span>
              </span>
              <span class="opt__sub">{{ r.summary }}</span>
            </span>
          </div>
        } @else {
          <label class="opt" [class.opt--on]="r.checked" [class.opt--off]="!!r.blocked">
            <input
              type="checkbox"
              class="mr-visually-hidden"
              [checked]="r.checked"
              [disabled]="!!r.blocked"
              (change)="flip.emit(r.key)"
            />
            <span class="opt__box" aria-hidden="true">
              @if (r.checked) {
                <mat-icon>check</mat-icon>
              }
            </span>
            <span class="opt__text">
              <span class="opt__head">
                <b>{{ r.name }}</b>
                @if (r.cost) {
                  <span class="opt__cost">{{ r.cost }}</span>
                }
              </span>
              <span class="opt__sub">{{ r.summary }}</span>
              @if (r.blocked) {
                <span class="opt__why"><mat-icon aria-hidden="true">block</mat-icon>{{ r.blocked }}</span>
              }
            </span>
          </label>
        }
        @if (r.checked && r.types.length > 0) {
          <fieldset class="types">
            <legend class="types__legend">Novo tipo de dano · {{ r.name }}</legend>
            @for (t of r.types; track t.key) {
              <label class="type">
                <input
                  type="radio"
                  [name]="id + '-' + r.key"
                  [checked]="t.key === r.type"
                  (change)="typed.emit({ key: r.key, type: t.key })"
                />
                <span>{{ t.name }}</span>
              </label>
            }
          </fieldset>
        }
      }
    </section>
  `,
  styleUrl: './table-option-picker.scss',
})
export class TableOptionPicker {
  readonly rows = input.required<readonly TableOptionRow[]>();

  readonly flip = output<string>();
  readonly typed = output<{ key: string; type: string }>();

  protected readonly id = `table-options-${nextId++}`;
}
