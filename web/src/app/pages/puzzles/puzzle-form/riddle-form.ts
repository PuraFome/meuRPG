import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import {
  type Draft,
  ANSWERS_MAX,
  ANSWER_MAX,
  RIDDLE_MAX,
  textLength,
} from '../../../core/puzzles/puzzle-draft';
import { SecretPill } from '../../../shared/puzzle-boards/secret-pill';

/**
 * The form of the riddle (MR-038, RN-27, E10-12 state 1): the riddle the players read (1 to 500 letters, in the master's own words)
 * and the answers he accepts. The answers are chips with a 44 px "×", and a field for the next one (Enter, "Adicionar" or leaving the
 * field puts it in the list); the list is "Só você vê", and no player ever receives it. A typed answer is right when it equals one of
 * them without capitals, accents or punctuation; the server compares, so the field says only what the server would refuse: none, too
 * long, empty once folded, or the same twice. Each refusal stands under the field it belongs to.
 */
@Component({
  selector: 'app-riddle-form',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule, SecretPill],
  template: `
    <mat-form-field appearance="outline" subscriptSizing="dynamic" class="full" [class.field-bad]="riddleError()">
      <mat-label>O enigma</mat-label>
      <textarea matInput rows="4" name="riddle" [value]="draft().riddleText" [attr.aria-invalid]="riddleError() ? 'true' : null" (input)="patch.emit({ riddleText: $any($event.target).value })"></textarea>
      @if (riddleError()) {
        <mat-hint class="field-error" role="alert">{{ riddleError() }}</mat-hint>
      } @else {
        <mat-hint>Escreva com as suas palavras. Os jogadores leem isto.</mat-hint>
      }
      @if (!riddleError()) {
        <mat-hint align="end">{{ length(draft().riddleText) }}&nbsp;de&nbsp;{{ riddleMax }}</mat-hint>
      }
    </mat-form-field>

    <section class="answers" aria-labelledby="answers-title">
      <h3 class="answers__title"><span id="answers-title">Respostas aceitas</span><app-secret-pill /></h3>
      @if (draft().answers.length > 0) {
        <ul class="chips" aria-labelledby="answers-title">
          @for (a of draft().answers; track $index) {
            <li class="chip" [class.chip--bad]="answerRows()[$index]">
              <span class="chip__text">{{ a }}</span>
              <button type="button" class="chip__x" [attr.aria-label]="'Tirar a resposta ' + a" (click)="remove($index)">
                <mat-icon aria-hidden="true">close</mat-icon>
              </button>
            </li>
          }
        </ul>
        @for (row of badRows(); track row[0]) {
          <p class="answers__bad field-error" role="alert">Resposta {{ row[0] + 1 }}: {{ row[1] }}</p>
        }
      }
      <div class="add">
        <mat-form-field appearance="outline" subscriptSizing="dynamic" class="add__field" [class.field-bad]="answersError()">
          <mat-label>{{ draft().answers.length === 0 ? 'Uma resposta' : 'Outra resposta' }}</mat-label>
          <input matInput type="text" name="answer" autocomplete="off" [value]="pending()" [attr.aria-invalid]="answersError() ? 'true' : null" (input)="pending.set($any($event.target).value)" (keydown.enter)="enter($event)" (blur)="commit()" />
        </mat-form-field>
        <button matButton="outlined" type="button" class="add__go" [disabled]="pending().trim() === '' || full()" disabledInteractive [class.mr-button--off]="pending().trim() === '' || full()" (click)="commit()">
          <mat-icon aria-hidden="true">add</mat-icon>Adicionar
        </button>
      </div>
      @if (answersError()) {
        <p class="answers__bad field-error" role="alert">{{ answersError() }}</p>
      }
      <p class="answers__help">
        Maiúsculas, acentos e pontuação não contam. De 1 a {{ answersMax }} respostas, cada uma com até {{ answerMax }} letras. O jogador nunca vê estas respostas.
      </p>
    </section>
  `,
  styleUrl: './riddle-form.scss',
})
export class RiddleForm {
  protected readonly riddleMax = RIDDLE_MAX;
  protected readonly answersMax = ANSWERS_MAX;
  protected readonly answerMax = ANSWER_MAX;

  readonly draft = input.required<Draft>();
  /** What is wrong with the riddle, the list, and each answer, once the master tried to save (then the server's). */
  readonly riddleError = input('');
  readonly answersError = input('');
  readonly answerRows = input<Readonly<Record<number, string>>>({});
  readonly patch = output<Partial<Draft>>();

  protected readonly pending = signal('');
  protected readonly full = computed(() => this.draft().answers.length >= ANSWERS_MAX);
  protected readonly badRows = computed(() =>
    Object.entries(this.answerRows()).map(([i, msg]) => [Number(i), msg] as const),
  );

  protected length(text: string): number {
    return textLength(text.trim());
  }

  protected enter(event: Event): void {
    // Enter in this field adds the answer; it never saves the form.
    event.preventDefault();
    this.commit();
  }

  /** The typed answer goes into the list (also when the field is left, so a save never loses it). */
  commit(): void {
    const text = this.pending().trim();
    if (text === '' || this.full()) {
      return;
    }
    this.patch.emit({ answers: [...this.draft().answers, text] });
    this.pending.set('');
  }

  protected remove(index: number): void {
    this.patch.emit({ answers: this.draft().answers.filter((_, i) => i !== index) });
  }
}
