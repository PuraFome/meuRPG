import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import { DescriptionLanguage } from '../../core/text/description-language';

/**
 * "Ver em inglês" / "Ver em português": flips the language of every SRD description in the app
 * until reload (`DescriptionLanguage`). Draw it only where the text has both languages.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-description-lang-button',
  imports: [MatButtonModule],
  template: `<button matButton type="button" class="lang" (click)="language.toggle()">
    {{ language.english() ? 'Ver em português' : 'Ver em inglês' }}
  </button>`,
  styles: `
    :host {
      display: inline-flex;
    }
  `,
})
export class DescriptionLangButton {
  protected readonly language = inject(DescriptionLanguage);
}
