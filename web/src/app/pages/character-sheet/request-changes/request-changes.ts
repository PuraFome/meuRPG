import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { ErrorStateMatcher } from '@angular/material/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

/** The longest reason the server takes (characters after trimming). */
export const REASON_MAX = 500;

/**
 * "Pedir ajustes em Lyra" (RN-15, MR-024): the master's form, in place of the
 * approval notice. The reason is required, 1 to 500 characters without the
 * spaces at the ends. "Enviar pedido" never disables on its own: tapped with
 * nothing to send, the field takes the error border and the words, as an
 * alert, and the focus goes to the field. Past 500 the error shows as the
 * counter passes it. The parent sends the call and hands back `busy` and the
 * server's `error`; this component only asks.
 */
@Component({
  selector: 'app-request-changes',
  imports: [
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    ReactiveFormsModule,
  ],
  templateUrl: './request-changes.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
  styleUrl: './request-changes.scss',
})
export class RequestChanges {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly characterName = input.required<string>();
  readonly playerName = input.required<string>();
  /** The call is on its way. */
  readonly busy = input(false);
  /** What the server refused with, in words. */
  readonly error = input<string | null>(null);
  /** The reason, trimmed, ready to send. */
  readonly requested = output<string>();
  readonly cancelled = output<void>();

  protected readonly max = REASON_MAX;
  protected readonly text = new FormControl('', { nonNullable: true });
  private readonly typed = signal('');
  private readonly asked = signal(false);

  /** The characters the server counts: the text without the spaces at the ends. */
  protected readonly length = computed(() => [...this.typed().trim()].length);
  protected readonly problem = computed<string | null>(() => {
    if (this.length() > REASON_MAX) {
      return 'O motivo passa de 500 caracteres.';
    }
    if (this.asked() && this.length() === 0) {
      return `Escreva o que ${this.playerName()} precisa ajustar. O pedido não vai sem motivo.`;
    }
    return null;
  });
  protected readonly matcher: ErrorStateMatcher = { isErrorState: () => this.problem() !== null };

  private readonly field = viewChild.required<ElementRef<HTMLTextAreaElement>>('field');

  constructor() {
    this.text.valueChanges.subscribe((value) => this.typed.set(value));
    afterNextRender(
      () => {
        this.host.nativeElement.scrollIntoView({ block: 'nearest' });
        this.field().nativeElement.focus({ preventScroll: true });
      },
      { injector: this.injector },
    );
  }

  protected submit(): void {
    if (this.busy()) {
      return;
    }
    this.asked.set(true);
    if (this.problem() !== null) {
      this.field().nativeElement.focus();
      return;
    }
    this.requested.emit(this.typed().trim());
  }
}
