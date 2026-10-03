import { Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

let nextId = 0;

/**
 * One option of "Sua vez" (E6-06): the name, a pill ("Truque", "1º
 * círculo"), the detail and, when the option can be used, its button. A
 * disabled option keeps its place and its focus stop (`aria-disabled`, not
 * `disabled`): the name goes muted, the button dashed, and a block icon with
 * the reason sits on its own line, wired with `aria-describedby`. A row
 * with no `button` (Escudo, a feature waiting for 6.5c) has no button by
 * design: its detail says when it is used.
 */
@Component({
  selector: 'app-action-row',
  imports: [MatButtonModule, MatIconModule],
  template: `
    <div class="row__text">
      @if (name()) {
        <span class="row__name">
          {{ name() }}
          @if (pill()) {
            <span class="row__pill">{{ pill() }}</span>
          }
        </span>
      }
      @if (detail()) {
        <span class="row__detail">{{ detail() }}</span>
      }
      @if (off() && reason()) {
        <span class="row__why" [id]="whyId"><mat-icon aria-hidden="true">block</mat-icon>{{ reason() }}</span>
      }
    </div>
    @if (button()) {
      <button
        mat-stroked-button
        type="button"
        class="row__btn"
        [class.row__btn--off]="off()"
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
  host: { '[class.row--off]': 'off()' },
})
export class ActionRow {
  readonly name = input.required<string>();
  readonly pill = input('');
  readonly detail = input('');
  /** "Atacar", "Conjurar"; empty for a row with no button. */
  readonly button = input('');
  /** The button's accessible name: "Atacar com Raio de Fogo". */
  readonly buttonLabel = input<string | null>(null);
  readonly off = input(false);
  readonly reason = input('');
  readonly busy = input(false);

  readonly press = output<void>();

  protected readonly whyId = `row-why-${nextId++}`;

  /** A disabled option keeps its focus stop but does nothing. */
  protected pressed(): void {
    if (!this.off() && !this.busy()) {
      this.press.emit();
    }
  }
}
