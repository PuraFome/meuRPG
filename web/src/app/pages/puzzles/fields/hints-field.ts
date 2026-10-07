import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  inject,
  input,
  output,
  viewChild,
  viewChildren,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { HINTS_LIMIT } from '../../../core/puzzles/puzzle-draft';

/**
 * "Dicas" of the three forms (E10-06 state 2, Q79): the hints the master releases one at a time during the session, each 1 to
 * 300 letters, up to ten. A numbered row for each, a 44 px "Remover a dica N", and "Adicionar uma dica" under the list. The
 * focus goes to the new row when one is added and to the button that added it when one is taken out. The words under the
 * rows say who reads them and when; the errors are the form's, by row.
 */
@Component({
  selector: 'app-hints-field',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule],
  template: `
    <section class="hf" aria-labelledby="hints-title">
      <h3 class="hf__title" id="hints-title">Dicas</h3>
      <p class="hf__help">Você solta uma dica de cada vez, durante a sessão. Os jogadores veem as que você soltou.</p>
      @for (hint of hints(); track $index) {
        <div class="row">
          <label class="row__box" [class.row__box--bad]="errors()[$index]">
            <span class="row__n" aria-hidden="true">{{ $index + 1 }}</span>
            <span class="mr-visually-hidden">Dica {{ $index + 1 }}</span>
            <input #field type="text" [value]="hint" [attr.aria-invalid]="errors()[$index] ? 'true' : null" [attr.aria-describedby]="errors()[$index] ? 'hint-err-' + $index : null" autocomplete="off" (input)="edit($index, $any($event.target).value)" />
          </label>
          <button type="button" class="row__remove" [attr.aria-label]="'Remover a dica ' + ($index + 1)" (click)="remove($index)">
            <mat-icon aria-hidden="true">delete</mat-icon>
          </button>
        </div>
        @if (errors()[$index]; as message) {
          <p class="hf__error" [id]="'hint-err-' + $index" role="alert">{{ message }}</p>
        }
      } @empty {
        <p class="hf__empty">Nenhuma dica ainda.</p>
      }
      <button #add matButton type="button" class="hf__add" [disabled]="hints().length >= limit" disabledInteractive [class.mr-button--off]="hints().length >= limit" (click)="hints().length < limit && append()">
        <mat-icon aria-hidden="true">add</mat-icon>Adicionar uma dica
      </button>
      @if (hints().length >= limit) {
        <p class="hf__help">Máximo de {{ limit }} dicas.</p>
      }
    </section>
  `,
  styleUrl: './hints-field.scss',
})
export class HintsField {
  private readonly injector = inject(Injector);
  protected readonly limit = HINTS_LIMIT;

  readonly hints = input.required<readonly string[]>();
  /** What is wrong with each row, by index (the form's). */
  readonly errors = input<Readonly<Record<number, string>>>({});
  readonly hintsChange = output<string[]>();

  private readonly fields = viewChildren<ElementRef<HTMLInputElement>>('field');
  private readonly addButton = viewChild<string, ElementRef<HTMLElement>>('add', {
    read: ElementRef,
  });

  protected edit(index: number, text: string): void {
    this.hintsChange.emit(this.hints().map((h, i) => (i === index ? text : h)));
  }

  protected append(): void {
    this.hintsChange.emit([...this.hints(), '']);
    afterNextRender(() => this.fields().at(-1)?.nativeElement.focus(), { injector: this.injector });
  }

  protected remove(index: number): void {
    this.hintsChange.emit(this.hints().filter((_, i) => i !== index));
    afterNextRender(() => this.addButton()?.nativeElement.focus(), { injector: this.injector });
  }
}
