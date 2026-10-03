import { Component, computed, input } from '@angular/core';

/**
 * The condition tags under a combatant's name (E6-29): each condition as a
 * pill with its name written out, and the spell it concentrates on. They are
 * labels the master marked (RN-22); the app applies no effect. In the strip of
 * chips (`compact`) there is room for one: the first and "+2" for the rest, so
 * a chip keeps its height. A list with a name for screen readers; nothing is
 * drawn for a combatant with no condition.
 */
@Component({
  selector: 'app-combatant-tags',
  template: `
    @if (shown().length || concentration()) {
      <ul class="tags" [attr.aria-label]="'Condições de ' + label()">
        @for (t of shown(); track t) {
          <li class="tag">{{ t }}</li>
        }
        @if (more() > 0) {
          <li class="tag tag--more">+{{ more() }}</li>
        }
        @if (concentration() && !compact()) {
          <li class="tag tag--conc">Concentrado: {{ concentration() }}</li>
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
      border-radius: var(--mr-radius-pill);
      font-size: 13px;
      font-weight: 700;
      line-height: 18px;
      white-space: nowrap;
    }

    .tag--conc {
      border-style: dashed;
      font-weight: 500;
    }

    .tag--more {
      border-color: var(--mr-control-line);
      color: var(--mr-ink-muted);
    }
  `,
})
export class CombatantTags {
  readonly names = input<readonly string[]>([]);
  /** The spell it is concentrating on; empty when it is not. */
  readonly concentration = input('');
  readonly label = input('');
  /** One tag only, with "+N" for the rest. */
  readonly compact = input(false);

  protected readonly shown = computed(() => (this.compact() ? this.names().slice(0, 1) : this.names()));
  protected readonly more = computed(() => (this.compact() ? Math.max(0, this.names().length - 1) : 0));
}
