import { NgTemplateOutlet } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { parseSrdText } from '../../core/format/srd-text';

/**
 * An SRD description (a spell's or a magic item's) drawn from its markdown: paragraphs, bold and
 * italics, lists and tables. Text bindings only, never `innerHTML`. `lang` is the language of the
 * paragraphs: English gets `lang="en"` so a screen reader switches voice; Portuguese leaves it off.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-srd-text',
  imports: [NgTemplateOutlet],
  templateUrl: './srd-text.html',
  styleUrl: './srd-text.scss',
})
export class SrdText {
  readonly paragraphs = input.required<readonly string[]>();
  readonly lang = input<'pt' | 'en'>('pt');

  protected readonly blocks = computed(() => parseSrdText(this.paragraphs()));
}
