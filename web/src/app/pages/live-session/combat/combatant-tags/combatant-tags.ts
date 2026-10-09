import { Component, computed, input, signal } from '@angular/core';

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
 * states the server sends (a rage, a mark, a dodge) come first, as chips that
 * are buttons: focus or a tap shows what the state does, and the same text is
 * read as the description of the chip. In the strip of chips (`compact`) there
 * is room for one: the first and "+2" for the rest, so a chip keeps its height.
 * A list with a name for screen readers; nothing is drawn for a combatant with
 * no condition and no state.
 */
@Component({
  selector: 'app-combatant-tags',
  template: `
    @if (shown().length) {
      <ul class="tags" [attr.aria-label]="'Condições e estados de ' + label()">
        @for (t of shown(); track t.key) {
          @if (t.state; as chip) {
            <li class="tag tag--state">
              <button
                type="button"
                class="tag__btn"
                [attr.aria-expanded]="open() === chip.id"
                [attr.title]="chip.description"
                [attr.aria-describedby]="uid + chip.id"
                (click)="toggle(chip.id)"
                (keydown.escape)="open.set('')"
              >
                {{ chip.text }}
              </button>
              <span class="sr" [id]="uid + chip.id">{{ chip.description }}</span>
              @if (open() === chip.id) {
                <span class="tag__note" role="note">{{ chip.description }}</span>
              }
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
      font-size: 13px;
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

    .tag--state {
      padding: 0;
      border-style: double;
      border-width: 3px;
    }

    .tag__btn {
      box-sizing: border-box;
      min-height: 24px;
      padding: 0 9px;
      border: 0;
      border-radius: 9px;
      background: none;
      color: var(--mr-ink);
      font: inherit;
      cursor: pointer;

      &:focus-visible {
        outline: 2px solid var(--mr-ink);
        outline-offset: 2px;
      }
    }

    .tag__note {
      display: block;
      padding: 2px 9px 4px;
      font-weight: 400;
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
  protected readonly open = signal('');

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

  protected toggle(id: string): void {
    this.open.update((current) => (current === id ? '' : id));
  }
}
