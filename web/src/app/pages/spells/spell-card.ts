import { Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { RouterLink } from '@angular/router';

import { joinDots } from '../../core/format/text';
import { SpellBody } from '../../shared/spell-details/spell-body';
import type { SpellDetailsVm } from '../../shared/spell-details/spell-details.types';
import { spellSubtitle } from '../../shared/spell-details/spell-details-format';

/** What the card shows: the spell being read, then its details. */
export type SpellCardState =
  | { readonly status: 'loading'; readonly namePt: string }
  /** `canRetry` is false for a spell that is not there (or not for this person): the sentence says so and there is nothing to try again. */
  | {
      readonly status: 'error';
      readonly namePt: string;
      readonly message: string;
      readonly canRetry: boolean;
    }
  | { readonly status: 'ready'; readonly details: SpellDetailsVm };

/**
 * One spell in full, on the "Magias" page (E10-11): the name, "Da mesa" for a table spell ("Arquivada" for
 * the master's retired one), the line "Shield · 1º nível · Abjuração · Mago, Feiticeiro", and the
 * shared `SpellBody` (the facts with "Alvo", and the text). An SRD spell's text is the SRD's own, in
 * English, with the credit under it. Loading and a failed read keep the card's place, with the way out.
 */
@Component({
  selector: 'app-spell-card',
  imports: [MatButtonModule, MatIconModule, MatProgressSpinnerModule, RouterLink, SpellBody],
  templateUrl: './spell-card.html',
  styleUrl: './spell-card.scss',
})
export class SpellCard {
  readonly state = input.required<SpellCardState>();
  /** The class names for the line under the title, by key. */
  readonly className = input<(key: string) => string>((key) => key);
  readonly retry = output<void>();

  protected readonly details = computed(() => {
    const s = this.state();
    return s.status === 'ready' ? s.details : null;
  });
  protected readonly title = computed(() => {
    const s = this.state();
    return s.status === 'ready' ? s.details.namePt : s.namePt || 'Magia';
  });
  /** "Shield" (SRD only), the circle and school, and the classes. */
  protected readonly line = computed(() => {
    const d = this.details();
    if (!d) {
      return '';
    }
    const classes = (d.classKeys ?? [])
      .map(this.className())
      .filter((n) => n !== '')
      .join(', ');
    return joinDots([spellSubtitle(d), classes].filter((p) => p !== ''));
  });
  protected readonly english = computed(() => {
    const d = this.details();
    return d && !d.table && d.nameEn && d.nameEn !== d.namePt ? d.nameEn : '';
  });
}
