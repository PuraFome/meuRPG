import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { type DescribedText, chooseText } from '../../../core/text/description-language';
import { DescriptionLangButton } from '../../../shared/srd-text/description-lang-button';
import { RuleText, paragraphsOf } from '../../../shared/rule-text/rule-text';
import { FeatureVm, FullSheetVm } from '../character-sheet.types';

/**
 * "Características e traços": one expandable row per feature (a native
 * `<details>`, closed by default) with its source, the SRD's text
 * in Portuguese first, with "Ver em inglês" for the English marked `lang="en"`; the sheet's own free text; the rules hints as quiet
 * reminders (the issues are in the notice under the header); then the
 * languages and proficiencies.
 */
@Component({
  selector: 'app-features-panel',
  imports: [MatIconModule, DescriptionLangButton, RuleText],
  templateUrl: './features-panel.html',
  styleUrl: './features-panel.scss',
})
export class FeaturesPanel {
  readonly sheet = input.required<FullSheetVm>();

  /** Both languages exist for some feature, so "Ver em inglês" makes sense. */
  protected readonly canToggle = computed(() =>
    this.sheet().features.some((f) => chooseText(this.textOf(f), false).canToggle),
  );

  protected textOf(f: FeatureVm): DescribedText {
    return {
      pt: paragraphsOf(f.descriptionPt),
      en: paragraphsOf(f.description),
      ptMissing: f.descriptionPtMissing,
      ptOnly: f.descriptionPtOnly,
    };
  }
}
