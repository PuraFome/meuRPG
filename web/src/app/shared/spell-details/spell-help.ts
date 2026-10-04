import { Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * The "?" next to a spell (E6-22, E8-02): a 44 × 44 px target that opens the
 * spell's details sheet. It is never disabled: a spell the character cannot cast
 * now still has a description to read. The accessible name carries the spell
 * ("Detalhes de Sono"), so a list of them is not a list of "?".
 */
@Component({
  selector: 'app-spell-help',
  imports: [MatIconModule],
  template: `
    <button type="button" class="help" [attr.aria-label]="'Detalhes de ' + name()" (click)="press.emit()">
      <mat-icon aria-hidden="true">help_outline</mat-icon>
    </button>
  `,
  styles: `
    :host {
      display: inline-flex;
      flex: none;
    }

    .help {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 44px;
      height: 44px;
      padding: 0;
      border: 0;
      border-radius: 50%;
      background: none;
      color: var(--mr-ink);
      cursor: pointer;
    }

    .mat-icon {
      width: 26px;
      height: 26px;
      font-size: 26px;
    }
  `,
})
export class SpellHelp {
  /** The spell's Portuguese name. */
  readonly name = input.required<string>();
  readonly press = output<void>();
}
