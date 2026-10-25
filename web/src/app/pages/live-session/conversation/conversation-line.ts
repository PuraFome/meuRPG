import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { LineView } from '../../../core/conversation/message-view';
import type { ListenController } from '../../../core/conversation/listen-controller';
import { ListenButton } from './listen-button';

/**
 * One line of the conversation (MR-051), in the look its kind has: narration on the page ground
 * with an optional picture; a spoken line (NPC or another player's character) with a round
 * portrait and the text in a bordered bubble; the reader's own line to the right, tinted; an
 * out-of-character remark small and muted; a line for chosen readers with a dashed border and a
 * lock. Everything on it is worded by `viewLine`; this draws it. The feed wraps it in its list item.
 */
@Component({
  selector: 'app-conversation-line',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ListenButton, MatIconModule],
  host: {
    class: 'line',
    '[class.line--narration]': "view().look === 'narration'",
    '[class.line--card]': "card() && view().look === 'narration'",
    '[class.line--npc]': "view().look === 'npc'",
    '[class.line--own]': "view().look === 'own'",
    '[class.line--own-portrait]': "view().look === 'own' && view().npc === true",
    '[class.line--ooc]': "view().look === 'out-of-character'",
    '[class.line--restricted]': "view().look === 'restricted'",
  },
  template: `
    @if (view().look === 'npc') {
      @if (view().portraitUrl; as url) {
        <img class="line__portrait" [src]="url" alt="" aria-hidden="true" />
      } @else {
        <span class="line__portrait" aria-hidden="true">{{ view().initial }}</span>
      }
      <div class="line__body">
        <div class="line__head">
          <span class="line__label">{{ view().label }}</span>
          @if (hearable()) {
            <app-listen-button [controller]="listen()" [lineId]="view().id" [audioUrl]="view().audioUrl" [subject]="subject()" />
          }
        </div>
        <p class="line__bubble">{{ view().text }}</p>
      </div>
    } @else {
      <div class="line__head">
        <span class="line__label">
          @if (view().look === 'restricted') {
            <mat-icon aria-hidden="true">lock</mat-icon>
          }
          {{ view().label }}
        </span>
        @if (hearable()) {
          <app-listen-button [controller]="listen()" [lineId]="view().id" [audioUrl]="view().audioUrl" [subject]="subject()" />
        }
      </div>
      @if (view().image; as image) {
        <img
          class="line__image"
          [src]="image.url"
          [alt]="image.name"
          [attr.width]="image.width || null"
          [attr.height]="image.height || null"
          [style.aspect-ratio]="image.width && image.height ? image.width + ' / ' + image.height : null"
          loading="lazy"
          (load)="imageLoaded.emit()"
        />
      }
      <p [class]="textClass()">{{ view().text }}</p>
      @if (view().look === 'own' && view().npc) {
        @if (view().portraitUrl; as url) {
          <img class="line__portrait line__portrait--own" [src]="url" alt="" aria-hidden="true" />
        } @else {
          <span class="line__portrait line__portrait--own" aria-hidden="true">{{ view().initial }}</span>
        }
      }
    }
  `,
  styleUrl: './conversation-line.scss',
})
export class ConversationLine {
  readonly view = input.required<LineView>();
  readonly listen = input.required<ListenController>();
  /** The server can make the voice of a line (`voice_enabled`). */
  readonly voice = input(false);
  /** The phone's full-screen feed is on the page ground, so the narration is a white card there. */
  readonly card = input(false);
  /** The picture finished loading (the feed may have grown under the reader). */
  readonly imageLoaded = output();

  protected readonly hearable = computed(
    () => this.voice() && this.view().canHear && this.view().audioUrl !== '',
  );
  protected readonly subject = computed(() => {
    const view = this.view();
    return view.speakerName ? `a fala de ${view.speakerName}` : 'a narração';
  });
  protected readonly textClass = computed(() => {
    switch (this.view().look) {
      case 'own':
      case 'out-of-character':
        return `line__bubble line__bubble--${this.view().look}`;
      default:
        return 'line__text';
    }
  });
}
