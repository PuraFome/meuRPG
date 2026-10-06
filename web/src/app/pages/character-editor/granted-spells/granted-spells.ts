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
    <section class="granted" aria-labelledby="granted-title">
      <h3 class="granted__title" id="granted-title">Sempre preparadas</h3>
      <p class="granted__hint">Vêm da subclasse, já estão na ficha e não contam no limite de preparadas.</p>
      <ul class="granted__list">
        @for (spell of spells(); track spell.key) {
          <li class="granted__row">
            <mat-icon aria-hidden="true">lock</mat-icon>
            <span class="granted__name">
              {{ spell.namePt }}
              <span class="granted__level">({{ circle(spell.level) }})</span>
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
  readonly describe = output<SpellOptionVm>();
  protected readonly circle = spellLevelLabel;
}
