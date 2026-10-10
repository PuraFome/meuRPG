import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';

import {
  type DescribedText,
  DescriptionLanguage,
  chooseText,
} from '../../core/text/description-language';
import { SrdText } from '../srd-text/srd-text';

/**
 * The text of an SRD feature, trait, background feature or feat: our Portuguese translation first,
 * the SRD's English after "Ver em inglês" (`DescriptionLangButton`, which the parent draws once).
 * "(em inglês)" shows only while English is on the screen. An entry with no translation yet shows
 * the English, flagged; a table entry (Portuguese only) shows its own text.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-rule-text',
  imports: [SrdText],
  template: `
    @if (shown().lang === 'en') {
      <span class="note" data-testid="lang-note">(em inglês)</span>
    }
    <app-srd-text [paragraphs]="shown().paragraphs" [lang]="shown().lang" />
  `,
  styles: `
    .note {
      display: block;
      font-size: 13px;
      color: var(--mr-ink-muted);
    }
  `,
})
export class RuleText {
  readonly text = input.required<DescribedText>();

  private readonly language = inject(DescriptionLanguage);
  protected readonly shown = computed(() => chooseText(this.text(), this.language.english()));
}

/** Paragraphs of a text the API joins with a blank line (`Feature.description`). */
export function paragraphsOf(text: string): readonly string[] {
  return text === '' ? [] : text.split('\n\n');
}
