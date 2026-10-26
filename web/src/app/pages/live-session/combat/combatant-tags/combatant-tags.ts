import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';
import type {
  CombatantEffect,
  ConditionSource,
} from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { conditionSourceText, stateChip } from './state-text';

let nextId = 0;

/**
 * The condition tags under a combatant's name (E6-29): each condition as a
 * pill with its name written out (the concentration is a pill of its own beside the name, in the order lists). They are
 * labels the master marked (RN-22); the app applies no effect. A condition that
 * comes from someone (a monk's Stunning Strike) says by whom and until when. The
 * states the server sends (a rage, a mark, a dodge) come first,
 * as plain text with an icon: what the state does is its tooltip and the description
 * a screen reader reads. In the strip of chips (`compact`) there
 * is room for one: the first and "+2" for the rest, so a chip keeps its height.
 * A list with a name for screen readers; nothing is drawn for a combatant with
 * no condition and no state.
 */
@Component({
  selector: 'app-combatant-tags',
  imports: [MatIconModule],
  template: `
    @if (shown().length) {
      <ul class="tags" [attr.aria-label]="'Condições de ' + label()">
        @for (t of shown(); track t.key) {
          @if (t.state; as chip) {
            <li class="tag tag--state" [attr.title]="chip.description" [attr.aria-describedby]="uid + chip.id">
              <mat-icon aria-hidden="true">bolt</mat-icon><span class="tag__text">{{ chip.text }}</span>
              <span class="sr" [id]="uid + chip.id">{{ chip.description }}</span>
            </li>
          } @else {
            <li class="tag">
              {{ t.text }}
              @if (t.by) {
                <span class="tag__by">, {{ t.by }}</span>
              }
            </li>
          }
        }
        @if (more() > 0) {
          <li class="tag tag--more">+{{ more() }}</li>
        }
      </ul>
    }
  `,
  styles: `
    :host {
      position: relative;
      display: block;
    }

    .tags {
      display: flex;
      flex-wrap: wrap;
      gap: 4px 6px;
      margin: 0;
      padding: 0;
      list-style: none;
    }

    .tag {
      box-sizing: border-box;
      padding: 1px 9px;
      border: 1px solid var(--mr-ink);
      border-radius: 12px;
      font-size: var(--mr-font-13);
      font-weight: 700;
      line-height: 18px;
      // A long tag ("Três quartos · marcada pelo mestre") wraps in a narrow chip instead of being cut.
      overflow-wrap: anywhere;
    }

    .tag--more {
      border-color: var(--mr-control-line);
      color: var(--mr-ink-muted);
    }

    .tag__by {
      font-weight: 400;
    }

    // A state the server sends (a rage, a dodge): plain text with its icon, no border and never a button; its text is the tooltip and the screen reader's description.
    .tag--state {
      display: inline-flex;
      align-items: center;
      gap: 3px;
      padding: 0;
      border: 0;
      border-radius: 0;
      color: var(--mr-ink);

      .mat-icon {
        flex: none;
        width: 14px;
        height: 14px;
        font-size: var(--mr-font-14);
      }
    }

    .sr {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip-path: inset(50%);
      white-space: nowrap;
    }
  `,
})
export class CombatantTags {
  readonly names = input<readonly string[]>([]);
  readonly label = input('');
  /** One tag only, with "+N" for the rest. */
  readonly compact = input(false);
  /** The states of the combatant (`Combatant.states`). */
  readonly states = input<readonly CombatantEffect[]>([]);
  /** Where its conditions come from (`Combatant.condition_sources`). */
  readonly sources = input<readonly ConditionSource[]>([]);
  /** The combatants of the combat, to name whose turn ends a state. */
  readonly people = input<readonly Pick<Combatant, 'id' | 'label'>[]>([]);

  protected readonly uid = `state-${nextId++}-`;

  private readonly all = computed(() => [
    ...this.states().map((s) => {
      const chip = stateChip(s, this.people());
      return { key: `state:${s.id}`, text: chip.text, by: '', state: chip };
    }),
    ...this.names().map((n) => ({
      key: `tag:${n}`,
      text: n,
      by: conditionSourceText(n, this.sources(), this.people()),
      state: null,
    })),
  ]);
  protected readonly shown = computed(() => (this.compact() ? this.all().slice(0, 1) : this.all()));
  protected readonly more = computed(() =>
    this.compact() ? Math.max(0, this.all().length - 1) : 0,
  );
}
