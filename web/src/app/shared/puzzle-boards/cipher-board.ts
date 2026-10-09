import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import { textLength } from '../../core/puzzles/puzzle-draft';
import type { CounterRow } from '../../core/puzzles/puzzle-format';
import { cipherColumns } from '../../core/puzzles/puzzle-text';
import { LimitCounters } from './limit-counters';
import { TYPED_MAX } from './riddle-board';

/**
 * The cipher (MR-038, E10-12 state 8): the letter as the players read it, a decoding table and the field for the deciphered message.
 * The table is a helper for the person's own head: a column for each letter of the letter (the ciphered letter above, a 44 px box
 * for the guess below) that the app never reads. Only "Conferir" talks to the server, which compares the message without capitals
 * or accents; the app never deciphers anything, and never knows the key. `view` is the master's card: the letter alone.
 */
@Component({
  selector: 'app-cipher-board',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [LimitCounters, MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule],
  template: `
    <section class="letter" aria-labelledby="cb-letter">
      <h3 class="title" id="cb-letter">{{ mode() === 'play' ? 'A carta, como está' : 'Os jogadores veem' }}</h3>
      <p class="cipher" lang="pt">{{ ciphertext() }}</p>
    </section>
    @if (mode() === 'play') {
      <section class="table" aria-labelledby="cb-table">
        <h3 class="title" id="cb-table">Tabela de decifrar</h3>
        <p class="help">Em cima, a letra da carta; embaixo, a sua aposta. O app não confere o que está aqui.</p>
        <div class="cols" role="group" aria-labelledby="cb-table">
          @for (letter of columns(); track letter) {
            <label class="col">
              <span class="col__letter" aria-hidden="true">{{ letter }}</span>
              <input class="col__bet" type="text" maxlength="1" autocomplete="off" autocapitalize="characters" spellcheck="false" [attr.aria-label]="'Sua aposta para a letra ' + letter" [value]="bets()[letter] || ''" (input)="bet(letter, $any($event.target).value)" />
            </label>
          }
        </div>
      </section>
      @if (blocked()) {
        <p class="blocked" role="status">{{ blocked() }}</p>
        <app-limit-counters [rows]="counters()" />
      } @else {
        <form class="ask" novalidate (submit)="send($event)">
          <mat-form-field appearance="outline" subscriptSizing="dynamic" class="ask__field">
            <mat-label>A mensagem decifrada</mat-label>
            <textarea matInput rows="2" name="message" autocomplete="off" spellcheck="false" [attr.maxlength]="max" [value]="typed()" (input)="typed.set($any($event.target).value)"></textarea>
          </mat-form-field>
          @if (verdict() === 'wrong') {
            <div class="mr-notice mr-notice--danger" role="alert">
              <mat-icon aria-hidden="true">close</mat-icon>
              <p><strong>Não é isso.</strong> Confira as letras da tabela.</p>
            </div>
          }
          <app-limit-counters [rows]="counters()" />
          <button matButton="filled" type="submit" class="ask__go" [disabled]="!ready() || busy()" disabledInteractive [class.mr-button--off]="!ready()">
            {{ busy() ? 'Enviando...' : 'Conferir' }}
          </button>
        </form>
      }
    }
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
    }

    .title {
      margin: 0 0 6px;
      font-size: 16px;
      font-weight: 700;
    }

    .cipher {
      margin: 0;
      padding: 10px 12px;
      border: 1px solid var(--mr-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-ground);
      font-family: var(--mr-font-display);
      font-size: 22px;
      line-height: 30px;
      letter-spacing: 0.06em;
      overflow-wrap: anywhere;
    }

    .help {
      margin: 0 0 var(--mr-space-2);
      font-size: 14px;
      line-height: 20px;
      color: var(--mr-ink-muted);
    }

    .cols {
      display: flex;
      flex-wrap: wrap;
      gap: 8px 6px;
    }

    .col {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 2px;
    }

    .col__letter {
      font-family: var(--mr-font-display);
      font-size: 18px;
      font-weight: 700;
      line-height: 24px;
    }

    .col__bet {
      box-sizing: border-box;
      width: 44px;
      height: 44px;
      padding: 0;
      border: 1.5px solid var(--mr-control-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-surface);
      color: var(--mr-ink);
      font-family: var(--mr-font-display);
      font-size: 20px;
      font-weight: 700;
      text-align: center;
      text-transform: uppercase;

      &:focus-visible {
        outline: 3px solid var(--mr-focus);
        outline-offset: 1px;
      }
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
export class CipherBoard {
  protected readonly max = TYPED_MAX;
  /** The ciphered message as the players read it (the server's work). */
  readonly ciphertext = input.required<string>();
  readonly mode = input<'play' | 'view'>('view');
  readonly verdict = input<'' | 'wrong'>('');
  readonly blocked = input('');
  readonly busy = input(false);
  /** "Tentativas restantes 2 de 3", "Jogadas 7 de 10", "Tempo 4:48 de 5:00": above the button. */
  readonly counters = input<readonly CounterRow[]>([]);

  /** The deciphered message the player typed, trimmed. */
  readonly answer = output<string>();

  protected readonly columns = computed(() => cipherColumns(this.ciphertext()));
  protected readonly bets = signal<Readonly<Record<string, string>>>({});
  protected readonly typed = signal('');
  protected readonly ready = computed(() => textLength(this.typed().trim()) > 0);

  protected bet(letter: string, value: string): void {
    this.bets.update((b) => ({ ...b, [letter]: value.trim().slice(0, 1).toUpperCase() }));
  }

  protected send(event: Event): void {
    event.preventDefault();
    const text = this.typed().trim();
    if (text !== '' && !this.busy()) {
      this.answer.emit(text);
    }
  }
}
