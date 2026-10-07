import {
  Component,
  DestroyRef,
  ElementRef,
  Injector,
  afterNextRender,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

/**
 * "Encerrar a minha parte" of a joint turn (E8-01, states 5 and 6): the
 * member's own button, an outline 48px and full width, never filled (the turn
 * does not end with it: the group's does, when the last part ends). A part
 * that ended cannot be reopened, so the button asks in place first,
 * "Encerrar a sua parte?", with what is still left, and "Voltar" takes the
 * focus; "Encerrar a minha parte" under it ends it. It stands in the pinned
 * bar on a phone and in the turn card from the tablet up.
 */
@Component({
  selector: 'app-end-part',
  imports: [MatButtonModule, MatIconModule],
  template: `
    @if (asking()) {
      <div #question class="ask" role="alertdialog" aria-labelledby="part-title" aria-describedby="part-text">
        <h3 class="ask__title" id="part-title">{{ heading() }}</h3>
        <p class="ask__text" id="part-text">
          @if (detail()) {
            {{ detail() }}
          } @else if (left()) {
            Ainda sobram {{ left() }}.
          }
          {{ warning() }}
        </p>
        <!-- "Voltar" outlined, the answer filled: not the same button twice. -->
        <div class="ask__buttons">
          <button #safe mat-stroked-button type="button" class="btn btn--back" (click)="back()">Voltar</button>
          <button mat-flat-button type="button" class="btn btn--go" [disabled]="busy()" (click)="confirm()">
            {{ confirmLabel() }}
          </button>
        </div>
      </div>
    } @else {
      @if (note()) {
        <p class="note">{{ note() }}</p>
      }
      <button #open mat-stroked-button type="button" class="btn" [disabled]="busy()" disabledInteractive (click)="ask()">
        <mat-icon aria-hidden="true">flag</mat-icon>{{ label() }}
      </button>
    }
  `,
  styles: `
    :host {
      display: block;
    }

    // The button that opens the question is the same as "Encerrar turno": an outline in the accent text.
    .btn {
      --mat-button-outlined-container-height: 48px;
      --mat-button-outlined-label-text-color: var(--mr-accent-text);
      --mat-button-outlined-outline-color: var(--mr-control-line);
      --mat-button-filled-container-height: 48px;
      width: 100%;
      white-space: nowrap;
    }

    .btn--back {
      --mat-button-outlined-label-text-color: var(--mr-ink);
    }

    // The two answers: stacked on a phone, side by side from a tablet, never wider than a button needs.
    .ask__buttons {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      gap: 10px;

      @media (min-width: 768px) {
        grid-template-columns: repeat(2, minmax(0, 240px));
      }
    }

    .ask {
      display: flex;
      flex-direction: column;
      gap: 10px;
      // Scrolled into view below the sticky app bar.
      scroll-margin-top: 72px;
      scroll-margin-bottom: 12px;
    }

    .note {
      margin: 0 0 8px;
      font-size: 14px;
      line-height: 19px;
      color: var(--mr-ink-muted);
    }

    .ask__title {
      margin: 0;
      font-family: var(--mr-font-display);
      font-weight: 700;
      font-size: 22px;
      line-height: 27px;
    }

    .ask__text {
      margin: 0;
      font-size: 15px;
      line-height: 20px;
      color: var(--mr-ink-muted);
    }
  `,
})
export class EndPart {
  private readonly injector = inject(Injector);

  constructor() {
    // The question goes away with the card (the turn moved on): whoever froze their tabs for it lets them go.
    inject(DestroyRef).onDestroy(() => {
      if (this.asking()) {
        this.asked.emit(false);
      }
    });
  }

  /** The button's words, and the question's: "Encerrar a parte dos Lobos" asks "Encerrar a parte dos Lobos?" (E9-12). */
  readonly label = input('Encerrar a minha parte');
  readonly heading = input('Encerrar a sua parte?');
  readonly confirmLabel = input('Encerrar a minha parte');
  readonly warning = input('Não dá para reabrir a sua parte depois.');
  /** What is still left, "Ação, Ação bônus, Reação e 9 m · 6 quadrados"; empty when nothing. */
  readonly left = input('');
  /** The whole sentence of what is left, when the caller has it better than a list ("Os 2 Lobos ainda têm ação e movimento."). */
  readonly detail = input('');
  /** The line above the button: "O turno passa quando você e a Brisa encerrarem." */
  readonly note = input('');
  readonly busy = input(false);
  readonly endPart = output<void>();
  /** The question is open: the footer leaves its list out while it asks. */
  readonly asked = output<boolean>();

  protected readonly asking = signal(false);
  private readonly safe = viewChild('safe', { read: ElementRef<HTMLButtonElement> });
  private readonly question = viewChild('question', { read: ElementRef<HTMLElement> });
  private readonly open = viewChild('open', { read: ElementRef<HTMLButtonElement> });

  protected ask(): void {
    this.asking.set(true);
    this.asked.emit(true);
    afterNextRender(
      () => {
        // The whole question in view, then the focus on the safe button.
        this.question()?.nativeElement.scrollIntoView({ block: 'nearest' });
        this.safe()?.nativeElement.focus({ preventScroll: true });
      },
      { injector: this.injector },
    );
  }

  protected back(): void {
    this.asking.set(false);
    this.asked.emit(false);
    afterNextRender(() => this.open()?.nativeElement.focus(), { injector: this.injector });
  }

  protected confirm(): void {
    this.asking.set(false);
    this.asked.emit(false);
    this.endPart.emit();
  }
}
