import { Component, input } from '@angular/core';

/**
 * The "Ao vivo" indicator (docs/design.md#componentes): an 8px accent dot
 * and a word, in an accent pill. One component for every place a live
 * session shows: the app bar link, the "Sessão ao vivo" tag on "Minhas
 * campanhas", the status line of the session page and the campaign's
 * "Sessão" panel.
 *
 * It is only the look. Whoever places it gives the meaning: a link around
 * it in the app bar (with an `aria-label` that names the session), plain
 * text everywhere else. The dot never pulses: motion only answers an
 * action (docs/design.md#movimento).
 */
@Component({
  selector: 'app-live-pill',
  template: `<span class="live-pill__dot" aria-hidden="true"></span>{{ label() }}`,
  styleUrl: './live-pill.scss',
  host: { class: 'live-pill', '[class.live-pill--small]': "size() === 'small'" },
})
export class LivePill {
  /** The word or words: "Ao vivo" by default, "Sessão ao vivo" in a list. */
  readonly label = input('Ao vivo');
  /** `small` is a list tag's height (26px), next to `.mr-tag`. */
  readonly size = input<'regular' | 'small'>('regular');
}
