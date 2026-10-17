import { ChangeDetectionStrategy, Component, computed, input, model, output } from '@angular/core';

import { MAX_TEXT } from '../../../core/conversation/message-view';

let nextId = 0;

/** The counter shows from this many characters, so it is quiet until the limit is near. */
const COUNT_FROM = MAX_TEXT - 200;

const OVER = 'A mensagem passa de 2.000 caracteres.';

/** 1850 as "1.850". */
function thousands(n: number): string {
  return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, '.');
}

/**
 * The message box of both composers: a visible label ("Mensagem"; hidden on a phone, where the
 * placeholder and the send button say it), the textarea, and, near the 2,000-character limit, a
 * counter. A refusal (empty, or too long) is a line under the box that the box points at with
 * `aria-describedby`, so a screen reader hears it with the field. Ctrl+Enter sends.
 */
@Component({
  selector: 'app-composer-field',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <label class="composer__label" [for]="id">Mensagem</label>
    <textarea
      class="composer__area"
      [class.composer__area--dashed]="dashed()"
      [id]="id"
      [rows]="rows()"
      [placeholder]="placeholder()"
      [value]="value()"
      [attr.aria-invalid]="shownError() ? 'true' : null"
      [attr.aria-describedby]="describedBy()"
      (input)="typed($event)"
      (keydown.control.enter)="sent.emit()"
      (keydown.meta.enter)="sent.emit()"
    ></textarea>
    @if (counting()) {
      <p class="composer__count" [id]="id + '-count'">{{ count() }} / 2.000</p>
    }
    @if (shownError(); as message) {
      <p class="composer__error" [id]="id + '-error'">{{ message }}</p>
    }
  `,
  styleUrl: './composer-field.scss',
})
export class ComposerField {
  readonly value = model('');
  readonly placeholder = input('');
  readonly rows = input(2);
  /** A whisper: the box has the dashed accent border. */
  readonly dashed = input(false);
  /** Why the last send was refused; empty when it was not. */
  readonly error = input('');
  /** The person typed: the page clears the refusal and, for the master, tells the table. */
  readonly changed = output<void>();
  /** Ctrl+Enter. */
  readonly sent = output<void>();

  protected readonly id = `msg-${nextId++}`;
  protected readonly counting = computed(() => this.value().length >= COUNT_FROM);
  protected readonly count = computed(() => thousands(this.value().length));
  protected readonly shownError = computed(() =>
    this.value().length > MAX_TEXT ? OVER : this.error(),
  );
  protected readonly describedBy = computed(() => {
    const ids = [
      this.counting() ? `${this.id}-count` : '',
      this.shownError() ? `${this.id}-error` : '',
    ].filter(Boolean);
    return ids.length > 0 ? ids.join(' ') : null;
  });

  protected typed(event: Event): void {
    this.value.set((event.target as HTMLTextAreaElement).value);
    this.changed.emit();
  }
}
