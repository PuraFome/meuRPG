import { Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { combatantInitial } from '../../../core/combat/combat-view';
import { CombatantToken } from '../../../shared/combatant-token/combatant-token';

/** A member of the party as the chips say it. */
export interface PartyChip {
  readonly name: string;
  readonly level: number;
  /** An NPC the master put in the party: the token is a rounded square and the chip has a way to take it out. */
  readonly npc: boolean;
  /** Which of the party's NPCs it is (in the order added), or -1 for a player's character. */
  readonly index: number;
}

/**
 * The party of the builder (E10-09 states 1 and 2): one chip per member, with the token, the name and "nível 4"; an NPC says
 * "NPC · nível 3" and has a 44 px "×" ("Tirar Orin do grupo"). Two columns on a phone, as many as fit from a tablet up.
 */
@Component({
  selector: 'app-party-chips',
  imports: [CombatantToken, MatIconModule],
  template: `
    @if (members().length > 0) {
      <ul class="chips" aria-label="O grupo">
        @for (m of members(); track $index) {
          <li class="chip">
            <app-combatant-token [initial]="initial(m.name)" [npc]="m.npc" [size]="32" />
            <span class="chip__t">
              <span class="chip__n">{{ m.name }}</span>
              <span class="chip__s">{{ m.npc ? 'NPC · nível ' + m.level : 'nível ' + m.level }}</span>
            </span>
            @if (m.npc) {
              <button type="button" class="chip__x" [attr.aria-label]="'Tirar ' + m.name + ' do grupo'" (click)="remove.emit(m.index)">
                <mat-icon aria-hidden="true">close</mat-icon>
              </button>
            }
          </li>
        }
      </ul>
    }
  `,
  styles: `
    :host {
      display: block;
    }

    .chips {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: var(--mr-space-2);
      margin: 0;
      padding: 0;
      list-style: none;

      // One column under 360 px, so a name never has to break inside a chip.
      @media (max-width: 359.98px) {
        grid-template-columns: minmax(0, 1fr);
      }

      @media (min-width: 768px) {
        grid-template-columns: repeat(auto-fill, minmax(170px, 1fr));
      }
    }

    .chip {
      display: flex;
      align-items: center;
      gap: var(--mr-space-2);
      min-height: 54px;
      box-sizing: border-box;
      padding: 6px 10px;
      border: 1px solid var(--mr-line);
      border-radius: var(--mr-radius-md);
    }

    .chip__t {
      display: flex;
      flex: 1;
      flex-direction: column;
      min-width: 0;
    }

    .chip__n {
      font-family: var(--mr-font-display);
      font-size: 19px;
      font-weight: 700;
      line-height: 22px;
    }

    .chip__s {
      font-size: 13px;
      line-height: 16px;
      color: var(--mr-ink-muted);
    }

    .chip__x {
      display: inline-flex;
      flex: none;
      align-items: center;
      justify-content: center;
      width: 44px;
      height: 44px;
      margin: -6px -10px -6px 0;
      border: 0;
      background: none;
      color: var(--mr-ink-muted);
      cursor: pointer;

      &:focus-visible {
        outline: 2px solid var(--mr-focus);
        outline-offset: -4px;
      }
    }
  `,
})
export class PartyChips {
  readonly members = input.required<readonly PartyChip[]>();
  /** "Tirar … do grupo": the index among the party's NPCs. */
  readonly remove = output<number>();
  protected readonly initial = combatantInitial;
}
