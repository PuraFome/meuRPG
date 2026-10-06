import { Component, computed, input } from '@angular/core';

import { SpellDetailsVm } from './spell-details.types';
import { spellRows } from './spell-details-format';

/**
 * What a spell says, below its title (E6-22, E10-11): the ritual and concentration tags, the facts
 * (casting time, range, "Alvo", components, duration, and "Ataque" and "Dano" for a table spell) and the
 * text. The "?" dialog (`SpellDetails`) and the "Magias" page's card both draw it, so a spell reads the
 * same in both. An SRD spell's text stays in English, marked as such; a table spell's is the master's,
 * in Portuguese, and is not marked.
 */
@Component({
  selector: 'app-spell-body',
  templateUrl: './spell-body.html',
  styleUrl: './spell-body.scss',
})
export class SpellBody {
  readonly details = input.required<SpellDetailsVm>();
  /** The page's card (E10-11): one fact per row, no boxes, the credit line says it is the SRD's text. The dialog keeps its boxes, two by two. */
  readonly plain = input(false);

  protected readonly facts = computed(() => spellRows(this.details()));
  /** The master wrote it: Portuguese, so no `lang="en"` and no "Texto do SRD". */
  protected readonly ours = computed(() => this.details().table === true);
}
