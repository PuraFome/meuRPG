import { Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { spellLevelLabel } from '../../../core/characters/character-labels';
import type { SpellOptionVm } from '../character-editor.types';

/**
 * "Sempre preparadas" on the Magias step of an edit (E10-11, state 4): the spells a subclass always
 * prepares (a domain's, an oath's), read from the sheet the server derived, never ticked and never
 * counted against the limit. A lock, the words "Sempre preparada" and the "?" of 44 px: the state
 * is never colour alone.
 */
@Component({
  selector: 'app-granted-spells',
  imports: [MatIconModule],
  template: `
    <section class="granted" [attr.aria-label]="headed() ? null : 'Sempre preparadas'" [attr.aria-labelledby]="headed() ? 'granted-title' : null">
      @if (headed()) {
        <h3 class="granted__title" id="granted-title">{{ title() }}</h3>
        <p class="granted__hint">{{ hint() }}</p>
      }
      <ul class="granted__list">
        @for (spell of spells(); track spell.key) {
          <li class="granted__row">
            <mat-icon aria-hidden="true">lock</mat-icon>
            <span class="granted__name">
              {{ spell.namePt }}
              <span class="granted__level">({{ circle(spell.level) }})</span>
              @if (source()) {
                <span class="granted__source">{{ source() }}</span>
              }
            </span>
            <span class="mr-tag"><mat-icon aria-hidden="true">lock</mat-icon>Sempre preparada</span>
            <button type="button" class="granted__help" [attr.aria-label]="'Descrição de ' + spell.namePt" (click)="describe.emit(spell)">
              <mat-icon aria-hidden="true">help_outline</mat-icon>
            </button>
          </li>
        }
      </ul>
    </section>
  `,
  styleUrl: './granted-spells.scss',
})
export class GrantedSpells {
  readonly spells = input.required<readonly SpellOptionVm[]>();
  /** Inside a class's section the rows come without a heading of their own, and say which subclass gives them. */
  readonly headed = input(true);
  readonly title = input('Sempre preparadas');
  readonly hint = input(
    'Vêm da subclasse, já estão na ficha e não contam no limite de preparadas.',
  );
  readonly source = input('');
  readonly describe = output<SpellOptionVm>();
  protected readonly circle = spellLevelLabel;
}
