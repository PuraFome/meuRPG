import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

/**
 * A panel's action on the campaign page (boards C6-02 and C6-03): a text action in the accent ink,
 * bold, with its icon before the words ("+ Novo mapa") or, with `underline`, a plain link
 * ("Ver todos os 9 NPCs", "Abrir as regras"). It is a link when it has a `link` and a button
 * otherwise (`pressed`). The target is 44 px high.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-panel-action',
  imports: [MatIconModule, RouterLink],
  template: `
    @if (link(); as to) {
      <a class="act" [class.act--underline]="underline()" [routerLink]="to">
        @if (icon()) {
          <mat-icon aria-hidden="true">{{ icon() }}</mat-icon>
        }
        {{ label() }}
      </a>
    } @else {
      <button
        type="button"
        class="act"
        [class.act--underline]="underline()"
        (click)="pressed.emit()"
      >
        @if (icon()) {
          <mat-icon aria-hidden="true">{{ icon() }}</mat-icon>
        }
        {{ label() }}
      </button>
    }
  `,
  styleUrl: './panel-action.scss',
})
export class PanelAction {
  readonly label = input.required<string>();
  /** The Material Symbols name before the words; empty for none. */
  readonly icon = input('');
  /** Where it goes (router commands); empty makes it a button. */
  readonly link = input<readonly string[] | null>(null);
  /** Underlined, for a link that reads as a sentence. */
  readonly underline = input(false);

  readonly pressed = output();
}
