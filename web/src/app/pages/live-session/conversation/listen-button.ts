import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { ListenController } from '../../../core/conversation/listen-controller';

/**
 * "Ouvir" on a narrator or NPC line (MR-051): the words and the state come from the page's one
 * `ListenController`, so only one line plays at a time. While the voice is made the visible words
 * are the name of the button; while it plays the button is pressed and says "Parar". A failure is
 * a quiet line beside it, in a status that is always in the page so it is announced.
 */
@Component({
  selector: 'app-listen-button',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <button
      type="button"
      class="listen"
      [class.listen--on]="playing()"
      [attr.aria-pressed]="playing() ? 'true' : null"
      [attr.aria-label]="ariaLabel()"
      (click)="controller().toggle(lineId(), audioUrl())"
    >
      <mat-icon aria-hidden="true">{{ playing() ? 'stop' : 'volume_up' }}</mat-icon>
      {{ controller().labelOf(lineId()) }}
    </button>
    <span class="listen__error" role="status">{{ controller().errorOf(lineId()) }}</span>
  `,
  styleUrl: './listen-button.scss',
})
export class ListenButton {
  readonly controller = input.required<ListenController>();
  readonly lineId = input.required<string>();
  readonly audioUrl = input.required<string>();
  /** What is read, for the name of the button: "a narração", "a fala de Bartolo". */
  readonly subject = input.required<string>();

  protected readonly playing = computed(() => this.controller().stateOf(this.lineId()) === 'playing');
  protected readonly ariaLabel = computed(() => {
    const state = this.controller().stateOf(this.lineId());
    if (state === 'loading') {
      return null;
    }
    return state === 'playing' ? 'Parar a leitura' : `Ouvir ${this.subject()}`;
  });
}
