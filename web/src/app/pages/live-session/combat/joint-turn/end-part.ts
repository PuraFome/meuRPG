import { Component, ElementRef, Injector, afterNextRender, inject, input, output, signal, viewChild } from '@angular/core';
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
        <h3 class="ask__title" id="part-title">Encerrar a sua parte?</h3>
        <p class="ask__text" id="part-text">
          @if (left()) {
            Ainda sobram {{ left() }}.
          }
          Não dá para reabrir a sua parte depois.
        </p>
        <button #safe mat-stroked-button type="button" class="btn" (click)="back()">Voltar</button>
        <button mat-stroked-button type="button" class="btn" [disabled]="busy()" (click)="confirm()">
          Encerrar a minha parte
        </button>
      </div>
    } @else {
      @if (note()) {
        <p class="note">{{ note() }}</p>
      }
      <button #open mat-stroked-button type="button" class="btn" [disabled]="busy()" disabledInteractive (click)="ask()">
        <mat-icon aria-hidden="true">flag</mat-icon>Encerrar a minha parte
      </button>
    }
  `,
  styles: `
    :host {
      display: block;
    }

    .btn {
      --mat-button-outlined-container-height: 48px;
      --mat-button-outlined-label-text-color: var(--mr-ink);
      --mat-button-outlined-outline-color: var(--mr-control-line);
      width: 100%;
      white-space: nowrap;
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

  /** What is still left, "Ação, Ação bônus, Reação e 9 m · 6 quadrados"; empty when nothing. */
  readonly left = input('');
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
