import { Component, input } from '@angular/core';

import type { Creature } from '../../../gen/meurpg/rules/v1/rules_pb';

/**
 * "Traços do Lobo" (MR-037, E9-11 state 3): the book's traits of the beast the druid is, in the SRD's English
 * (labelled once, and `lang="en"` for a screen reader); the master applies the advantage they give (the app does
 * not see a scent on the map). Nothing here when the beast has none.
 */
@Component({
  selector: 'app-beast-traits',
  template: `
    @if (traits().length > 0) {
      <section class="mr-panel traits" [attr.aria-label]="'Traços do ' + beast()">
        <h3 class="traits__title">Traços do {{ beast() }}</h3>
        <p class="traits__srd">Textos do livro de regras (SRD 5.1), em inglês.</p>
        @for (t of traits(); track t.name) {
          <div class="trait" lang="en">
            <h4 class="trait__name">{{ t.name }}</h4>
            <p class="trait__text">{{ t.text }}</p>
          </div>
        }
      </section>
    }
  `,
  styles: `
    :host {
      display: block;
    }

    .traits__title {
      margin: 0;
      font-family: var(--mr-font-display);
      font-size: 24px;
      line-height: 28px;
    }

    .traits__srd {
      margin: 2px 0 0;
      padding-bottom: 8px;
      border-bottom: 1px solid var(--mr-rule);
      font-size: 14px;
      color: var(--mr-ink-muted);
    }

    .trait {
      padding: 10px 0 2px;

      & + & {
        margin-top: 8px;
        border-top: 1px solid var(--mr-rule);
      }
    }

    .trait__name {
      margin: 0;
      font-family: var(--mr-font-display);
      font-size: 19px;
      line-height: 24px;
    }

    .trait__text {
      margin: 2px 0 8px;
      font-size: 15px;
      line-height: 21px;
    }
  `,
})
export class BeastTraits {
  readonly creature = input.required<Creature>();
  /** The beast's Portuguese name. */
  readonly beast = input.required<string>();

  protected traits(): Creature['traits'] {
    return this.creature().traits;
  }
}
