import {
  Component,
  ElementRef,
  Injector,
  OnInit,
  afterNextRender,
  computed,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import { formatInt } from '../../../core/format/text';
import { CLUE_MAX, clueTextError, oneLine, textLength } from '../../../core/maps/scene-clues';

let nextId = 0;

/**
 * "Nova pista" and "Editar a pista" (E8-04): the form that opens in place in
 * the clue list. One text of up to 500 characters on one line (a pasted line
 * break becomes a space; Enter sends). Nothing is cut at the limit: the
 * counter turns into "512 de 500" and the message says how many to take off,
 * under the field, in a box with an icon, and the focus goes back to the
 * field. The buttons sit under the field: the outlined one that sends and
 * "Cancelar" as text. The list runs the call and shows what the server
 * refused in `error`. The panel's one "É ficção" notice (under the hooks)
 * covers it: the panel never says it twice.
 */
@Component({
  selector: 'app-clue-form',
  imports: [
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    ReactiveFormsModule,
  ],
  template: `
    <form class="cf" novalidate (submit)="$event.preventDefault(); submit()" [attr.aria-labelledby]="id + '-t'">
      <h4 class="cf__title" [id]="id + '-t'">{{ title() }}</h4>
      <mat-form-field appearance="outline" subscriptSizing="dynamic">
        <mat-label>Texto da pista</mat-label>
        <textarea
          #field
          matInput
          rows="4"
          autocomplete="off"
          [formControl]="control"
          (keydown.enter)="$event.preventDefault(); submit()"
        ></textarea>
        <mat-hint>Os jogadores leem exatamente este texto.</mat-hint>
        <mat-hint align="end" [class.cf__over]="over()">{{ counter() }}</mat-hint>
      </mat-form-field>
      @if (message(); as text) {
        <div class="mr-notice mr-notice--danger" role="alert">
          <mat-icon aria-hidden="true">error</mat-icon>
          <p>{{ text }}</p>
        </div>
      }
      <div class="cf__actions">
        <button matButton="outlined" type="submit" [attr.aria-disabled]="busy()">{{ submitLabel() }}</button>
        <button matButton type="button" (click)="cancelled.emit()">Cancelar</button>
      </div>
    </form>
  `,
  styles: `
    :host {
      display: block;
    }

    // The form sits in the clue list, inside its own bordered card.
    .cf {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
      padding: var(--mr-space-4);
      border: 1px solid var(--mr-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-surface);
      // A form opened low in the panel scrolls whole into view under the sticky bar.
      scroll-margin-block: 72px 16px;
    }

    .cf__title {
      margin: 0;
      font-family: var(--mr-font-display);
      font-size: 19px;
      font-weight: 700;
      line-height: 24px;
    }

    // Over the limit the counter says so in colour and in numbers ("512 de 500").
    .cf__over {
      color: var(--mr-danger-ink);
      font-weight: 700;
    }

    // The buttons are under the field, never beside it.
    .cf__actions {
      display: flex;
      align-items: center;
      gap: var(--mr-space-3);
    }
  `,
})
export class ClueForm implements OnInit {
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  /** The clue being edited, or `null` for a new one. */
  readonly clue = input<{ readonly text: string; readonly number: number } | null>(null);
  /** What the server refused (the 30-clue limit, a lost point). */
  readonly error = input('');
  readonly busy = input(false);

  readonly submitted = output<string>();
  readonly cancelled = output<void>();

  protected readonly id = `cf-${nextId++}`;
  protected readonly control = new FormControl('', { nonNullable: true });
  private readonly length = signal(0);
  private readonly local = signal('');
  protected readonly title = computed(() => {
    const clue = this.clue();
    return clue ? `Editar a pista ${clue.number}` : 'Nova pista';
  });
  protected readonly submitLabel = computed(() =>
    this.clue() ? 'Salvar pista' : 'Adicionar pista',
  );
  protected readonly over = computed(() => this.length() > CLUE_MAX);
  protected readonly counter = computed(
    () => `${formatInt(this.length())} de ${formatInt(CLUE_MAX)}`,
  );
  protected readonly message = computed(() => this.local() || this.error());

  private readonly field = viewChild.required<ElementRef<HTMLTextAreaElement>>('field');

  constructor() {
    this.control.valueChanges.pipe(takeUntilDestroyed()).subscribe((value) => {
      const flat = oneLine(value);
      if (flat !== value) {
        this.control.setValue(flat, { emitEvent: false });
      }
      this.length.set(textLength(flat));
      this.local.set('');
    });
    afterNextRender(
      () => {
        // The whole form comes into view, then the focus goes to the field without scrolling again.
        this.host.nativeElement.scrollIntoView({ block: 'nearest' });
        this.field().nativeElement.focus({ preventScroll: true });
      },
      { injector: this.injector },
    );
  }

  ngOnInit(): void {
    const clue = this.clue();
    if (clue) {
      this.control.setValue(oneLine(clue.text));
    }
  }

  protected submit(): void {
    if (this.busy()) {
      return;
    }
    const text = this.control.value.trim();
    const problem = clueTextError(text);
    if (problem) {
      this.local.set(problem);
      this.field().nativeElement.focus();
      return;
    }
    this.submitted.emit(text);
  }
}
