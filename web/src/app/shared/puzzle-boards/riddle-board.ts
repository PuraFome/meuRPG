import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import { textLength } from '../../core/puzzles/puzzle-draft';
import type { CounterRow } from '../../core/puzzles/puzzle-format';
import { LimitCounters } from './limit-counters';

/** The longest text a player may send as an answer (the server's `MaxTyped`). */
export const TYPED_MAX = 600;

/**
 * The riddle (MR-038, E10-12 state 6): what the master wrote, and in `play` the field where a player types an answer and "Responder". The
 * answer is judged by the server, never here: the board only says "Não é isso." when the page says the last answer was wrong, without
 * saying how close it was (the server never says). A player with no attempts left, or any run that is not open, gets a dashed box with
 * the reason in words in the field's place (`blocked`). `view` is the master's card: the riddle alone.
 */
@Component({
  selector: 'app-riddle-board',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { '[class.rb--ask]': "mode() === 'play'" },
  imports: [LimitCounters, MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule],
  template: `
    <p class="text">{{ text() }}</p>
    @if (mode() === 'play') {
      @if (blocked()) {
        <p class="blocked" role="status">{{ blocked() }}</p>
        <app-limit-counters [rows]="counters()" />
      } @else {
        <form class="ask" novalidate (submit)="send($event)">
          <mat-form-field appearance="outline" subscriptSizing="dynamic" class="ask__field">
            <mat-label>Sua resposta</mat-label>
            <input matInput type="text" name="answer" autocomplete="off" autocapitalize="off" spellcheck="false" [attr.maxlength]="max" [value]="typed()" (input)="typed.set($any($event.target).value)" />
          </mat-form-field>
          @if (verdict() === 'wrong') {
            <div class="mr-notice mr-notice--danger" role="alert">
              <mat-icon aria-hidden="true">close</mat-icon>
              <p><strong>Não é isso.</strong> Tente outra resposta.</p>
            </div>
          }
          <app-limit-counters [rows]="counters()" />
          <button matButton="filled" type="submit" class="ask__go" [disabled]="!ready() || busy()" disabledInteractive [class.mr-button--off]="!ready()">
            {{ busy() ? 'Enviando...' : 'Responder' }}
          </button>
        </form>
      }
    }
  `,
  styles: `
    :host {
      display: block;
    }

    .text {
      margin: 0;
      font-size: 18px;
      line-height: 26px;
    }

    // The field is a block away from the riddle; the master's view (no field) has nothing under it.
    :host(.rb--ask) .text {
      margin-bottom: var(--mr-space-3);
    }

    .ask {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
    }

    .ask__field {
      width: 100%;
    }

    .ask__go {
      min-height: 48px;
    }

    // A short screen (320 × 568): the riddle or the letter scrolls and the field, the counters and the button stay at the foot of it.
    @media (max-height: 600px) {
      .ask {
        position: sticky;
        bottom: 0;
        z-index: 1;
        margin-inline: calc(-1 * var(--mr-space-3));
        padding: var(--mr-space-2) var(--mr-space-3) var(--mr-space-2);
        border-top: 1px solid var(--mr-rule);
        background: var(--mr-surface);
      }
    }

    .blocked {
      margin: 0 0 var(--mr-space-3);
      padding: 14px;
      border: 1.5px dashed var(--mr-control-line);
      border-radius: var(--mr-radius-md);
      color: var(--mr-ink-muted);
      font-size: 16px;
      line-height: 22px;
    }
  `,
})
export class RiddleBoard {
  protected readonly max = TYPED_MAX;
  /** What the players read: the riddle. */
  readonly text = input.required<string>();
  readonly mode = input<'play' | 'view'>('view');
  /** The last answer was wrong (the page knows it from the server's last move). */
  readonly verdict = input<'' | 'wrong'>('');
  /** Why nothing can be typed now ("Você não tem mais tentativas."), or `''`. */
  readonly blocked = input('');
  readonly busy = input(false);
  /** "Suas tentativas 2 de 3", "Jogadas 7 de 10", "Tempo 4:48 de 5:00": above the button. */
  readonly counters = input<readonly CounterRow[]>([]);

  /** A typed answer, trimmed. */
  readonly answer = output<string>();

  protected readonly typed = signal('');
  protected readonly ready = computed(() => textLength(this.typed().trim()) > 0);

  protected send(event: Event): void {
    event.preventDefault();
    const text = this.typed().trim();
    if (text !== '' && !this.busy()) {
      this.answer.emit(text);
    }
  }
}
