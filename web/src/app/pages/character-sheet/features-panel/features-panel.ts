import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { FullSheetVm } from '../character-sheet.types';

/**
 * "Características e traços": one expandable row per feature (a native
 * `<details>`, closed by default) with its source, the SRD's English text
 * marked `lang="en"`; the sheet's own free text; the rules hints as quiet
 * reminders (the issues are in the notice under the header); then the
 * languages and proficiencies.
 */
@Component({
  selector: 'app-features-panel',
  imports: [MatIconModule],
  templateUrl: './features-panel.html',
  styleUrl: './features-panel.scss',
})
export class FeaturesPanel {
  readonly sheet = input.required<FullSheetVm>();
}
