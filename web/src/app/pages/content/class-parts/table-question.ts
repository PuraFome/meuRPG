import { Component, ElementRef, afterNextRender, inject, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

/**
 * The question in place before a table the master edited is replaced (E10-02 state 1: changing how the class casts, "Restaurar o
 * padrão"): a title, the sentence, "Refazer" and "Manter a minha tabela". It sits under the control that asked it and takes the
 * focus on its title when it opens, so the keyboard and the screen reader land on it.
 */
@Component({
  selector: 'app-table-question',
  imports: [MatButtonModule],
  template: `
    <div class="ask" role="alertdialog" [attr.aria-labelledby]="id + '-t'" [attr.aria-describedby]="id + '-d'">
      <p class="ask__title" [id]="id + '-t'" tabindex="-1" #title>{{ heading() }}</p>
      <p [id]="id + '-d'">{{ text() }}</p>
      <div class="ask__actions">
        <button matButton="filled" type="button" (click)="answer.emit(true)">{{ confirm() }}</button>
        <button matButton="outlined" type="button" (click)="answer.emit(false)">Manter a minha tabela</button>
      </div>
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .ask {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-2);
      padding: var(--mr-space-4);
      border: 2px solid var(--mr-warning-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-warning-surface);

      p {
        margin: 0;
      }

      &__title {
        font-weight: 800;

        &:focus {
          outline: none;
        }
      }

      &__actions {
        display: flex;
        flex-wrap: wrap;
        gap: var(--mr-space-3);

        button {
          min-height: 44px;
        }
      }
    }
  `,
})
export class TableQuestion {
  private static next = 0;
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly heading = input.required<string>();
  readonly text = input.required<string>();
  readonly confirm = input('Refazer a tabela');
  /** true: replace the table; false: keep the master's. */
  readonly answer = output<boolean>();
  protected readonly id = `tq-${TableQuestion.next++}`;

  constructor() {
    afterNextRender(() => this.host.nativeElement.querySelector<HTMLElement>('.ask__title')?.focus({ preventScroll: true }));
  }
}
