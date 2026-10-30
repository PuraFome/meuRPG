import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

import { OptionCard } from '../../../../shared/option-card/option-card';
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
  imports: [OptionCard],
  template: `
    <section class="to" [attr.aria-labelledby]="id + '-t'">
      <h3 class="to__title" [id]="id + '-t'">Opções das suas características</h3>
      @for (r of rows(); track r.key) {
        @if (r.alwaysOn) {
          <app-option-card kind="fixed" [title]="r.name" cost="Sempre ativa" [sub]="r.summary" />
        } @else {
          <app-option-card
            [title]="r.name"
            [cost]="r.cost"
            [sub]="r.summary"
            [why]="r.blocked"
            [checked]="r.checked"
            (chosen)="flip.emit(r.key)"
          />
        }
        @if (r.checked && r.types.length > 0) {
          <fieldset class="types" role="radiogroup">
            <legend class="types__legend">Novo tipo de dano · {{ r.name }}</legend>
            @for (t of r.types; track t.key) {
              <app-option-card
                kind="radio"
                [name]="id + '-' + r.key"
                [title]="t.name"
                [checked]="t.key === r.type"
                (chosen)="typed.emit({ key: r.key, type: t.key })"
              />
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
