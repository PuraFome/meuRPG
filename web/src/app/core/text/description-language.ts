import { Injectable, signal } from '@angular/core';

/**
 * Which language the SRD descriptions (spells and magic items) are read in, for the whole app until
 * the page reloads: Portuguese by default, English after "Ver em inglês". One signal on the root
 * injector, so the spell dialog, the "Magias" page and the item sheet all agree. It is never saved
 * (no Web Storage): a reload starts in Portuguese again.
 */
@Injectable({ providedIn: 'root' })
export class DescriptionLanguage {
  private readonly englishState = signal(false);
  /** True while descriptions that have both languages show the English one. */
  readonly english = this.englishState.asReadonly();

  toggle(): void {
    this.englishState.update((v) => !v);
  }
}

/** A description in the languages the server sent. */
export interface DescribedText {
  /** Our Portuguese paragraphs; empty while `ptMissing`. */
  readonly pt: readonly string[];
  /** The SRD's English paragraphs (for a table spell, the table's own Portuguese). */
  readonly en: readonly string[];
  /** The SRD text has no Portuguese translation yet. */
  readonly ptMissing: boolean;
  /** The text exists only in Portuguese (the table's own): there is no English to offer. */
  readonly ptOnly?: boolean;
}

/** What to draw for a description. */
export interface ShownText {
  readonly paragraphs: readonly string[];
  readonly lang: 'pt' | 'en';
  /** English is shown only because the Portuguese is missing: say "(em inglês)". */
  readonly missing: boolean;
  /** Both languages exist, so the "Ver em inglês" button makes sense. */
  readonly canToggle: boolean;
}

/** Picks the paragraphs to show: Portuguese unless the reader asked for English and it exists. */
export function chooseText(text: DescribedText, english: boolean): ShownText {
  if (text.ptOnly) {
    return {
      paragraphs: text.pt.length > 0 ? text.pt : text.en,
      lang: 'pt',
      missing: false,
      canToggle: false,
    };
  }
  if (text.ptMissing || text.pt.length === 0) {
    return { paragraphs: text.en, lang: 'en', missing: true, canToggle: false };
  }
  return {
    paragraphs: english ? text.en : text.pt,
    lang: english ? 'en' : 'pt',
    missing: false,
    canToggle: true,
  };
}
