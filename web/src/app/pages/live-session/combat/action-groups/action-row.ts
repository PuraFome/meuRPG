import { Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { SpellHelp } from '../../../../shared/spell-details/spell-help';

let nextId = 0;

/**
 * One option of "Sua vez" (E6-06): the name, its tags on a line of their own
 * ("Truque", "1º nível", "Reação": the name column is narrow beside the "?"
 * and the button), the detail and, when the option can be used, its button. A
 * spell has the "?" (E8-02) between its text and the button, 44 px, never
 * disabled: the description of a spell that cannot be cast now is still worth
 * reading. A
 * disabled option keeps its place and its focus stop (`aria-disabled`, not
 * `disabled`): the name goes muted, the button dashed, and a block icon with
 * the reason sits on its own line, wired with `aria-describedby`. A row
 * with no `button` (Escudo, a feature waiting for 6.5c) has no button by
 * design: its detail says when it is used.
 */
@Component({
  selector: 'app-action-row',
  imports: [MatButtonModule, MatIconModule, SpellHelp],
  template: `
    <div class="row__text">
      @if (name()) {
        <span class="row__name">{{ name() }}</span>
        @if (tags().length) {
          <span class="row__tags">
            @for (tag of tags(); track tag) {
              <span class="row__pill">{{ tag }}</span>
            }
          </span>
        }
      }
      @if (detail()) {
        <span class="row__detail">{{ detail() }}</span>
      }
      @if (rider()) {
        <span class="row__rider" lang="en">{{ rider() }}</span>
      }
      @if (off() && reason()) {
        <span class="row__why" [id]="whyId"><mat-icon aria-hidden="true">block</mat-icon>{{ reason() }}</span>
      }
    </div>
    @if (helpName()) {
      <app-spell-help [name]="helpName()" (press)="help.emit()" />
    }
    @if (button()) {
      <button
        mat-stroked-button
        type="button"
        class="row__btn"
        [class.row__btn--off]="off()"
        [class.mr-button--off]="off()"
        [disabled]="off() || busy()"
        disabledInteractive
        [attr.aria-label]="buttonLabel()"
        [attr.aria-describedby]="off() && reason() ? whyId : null"
        (click)="pressed()"
      >
        {{ button() }}
      </button>
    }
  `,
  styleUrl: './action-row.scss',
  host: { '[class.row--off]': 'off()', '[class.row--help]': '!!helpName()' },
})
export class ActionRow {
  readonly name = input.required<string>();
  /** The small tags under the name: the circle, and "Reação" or "Ação bônus" for a spell of another economy. */
  readonly tags = input<readonly string[]>([]);
  /** The spell's name, when the row has the "?": it names the button ("Detalhes de Sono"). */
  readonly helpName = input('');
  readonly detail = input('');
  /** The SRD's own text of a creature's attack (its rider), under the detail; English, as the book has it, and never translated here. */
  readonly rider = input('');
  /** "Atacar", "Conjurar"; empty for a row with no button. */
  readonly button = input('');
  /** The button's accessible name: "Atacar com Raio de Fogo". */
  readonly buttonLabel = input<string | null>(null);
  readonly off = input(false);
  readonly reason = input('');
  readonly busy = input(false);

  readonly press = output<void>();
  /** The "?": open the spell's details. */
  readonly help = output<void>();

  protected readonly whyId = `row-why-${nextId++}`;

  /** A disabled option keeps its focus stop but does nothing. */
  protected pressed(): void {
    if (!this.off() && !this.busy()) {
      this.press.emit();
    }
  }
}
