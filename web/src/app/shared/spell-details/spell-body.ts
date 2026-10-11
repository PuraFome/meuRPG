import { Component, computed, inject, input } from '@angular/core';

import { DescriptionLanguage, chooseText } from '../../core/text/description-language';
import { DescriptionLangButton } from '../srd-text/description-lang-button';
import { SrdText } from '../srd-text/srd-text';
import { SpellDetailsVm } from './spell-details.types';
import { spellRows } from './spell-details-format';

/**
 * What a spell says, below its title (E6-22, E10-11): the ritual and concentration tags, the facts
 * (casting time, range, "Alvo", components, duration, and "Ataque" and "Dano" for a table spell) and the
 * text. The "?" dialog (`SpellDetails`) and the "Magias" page's card both draw it, so a spell reads the
 * same in both. An SRD spell's text is our Portuguese translation, with "Ver em inglês" to read the
 * SRD's own (the choice holds for the whole app, `DescriptionLanguage`); a spell with no translation
 * yet shows the English with "(em inglês)". A table spell's text is the master's, in Portuguese, and
 * has neither the label nor the button.
 */
@Component({
  selector: 'app-spell-body',
  imports: [DescriptionLangButton, SrdText],
  templateUrl: './spell-body.html',
  styleUrl: './spell-body.scss',
})
export class SpellBody {
  private readonly language = inject(DescriptionLanguage);
  readonly details = input.required<SpellDetailsVm>();
  /** The page's card (E10-11): one fact per row, no boxes, the credit line says it is the SRD's text. The dialog keeps its boxes, two by two. */
  readonly plain = input(false);

  /** The master wrote it: Portuguese, so no `lang="en"` and no "Texto do SRD". */
  protected readonly ours = computed(() => this.details().table === true);
  private readonly wantEnglish = computed(() => !this.ours() && this.language.english());
  protected readonly facts = computed(() => spellRows(this.details(), this.wantEnglish()));
  protected readonly description = computed(() => {
    const d = this.details();
    return chooseText(
      {
        pt: d.descriptionPt ?? [],
        en: d.description,
        ptMissing: d.textPtMissing === true,
        ptOnly: this.ours() || d.textPtOnly === true,
      },
      this.wantEnglish(),
    );
  });
  protected readonly higher = computed(() => {
    const d = this.details();
    return chooseText(
      {
        pt: d.higherLevelPt ?? [],
        en: d.higherLevel,
        ptMissing: d.textPtMissing === true,
        ptOnly: this.ours() || d.textPtOnly === true,
      },
      this.wantEnglish(),
    );
  });
}
