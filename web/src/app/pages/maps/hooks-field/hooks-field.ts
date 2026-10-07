import { Component, computed, input } from '@angular/core';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import { formatInt } from '../../../core/format/text';
import { HOOKS_MAX } from '../../../core/maps/scene-clues';
import { FictionNotice } from '../../../shared/fiction-notice/fiction-notice';

/**
 * "Ganchos e anotações" in the point panel of a SCENE point (E8-04, MR-029):
 * the master's private notes on the scene (Markdown, up to 4.000 characters).
 * The first line says, with a lock and in words, that no player ever sees it.
 * It is a field of the point, so it waits for "Salvar ponto" like the
 * description; the panel owns the control and the error. The panel's single
 * "É ficção" notice sits under it.
 */
@Component({
  selector: 'app-hooks-field',
  imports: [FictionNotice, MatFormFieldModule, MatIconModule, MatInputModule, ReactiveFormsModule],
  template: `
    <section class="hf" aria-labelledby="hf-title">
      <h3 class="hf__title" id="hf-title">Ganchos e anotações</h3>
      <p class="hf__lock">
        <mat-icon aria-hidden="true">lock</mat-icon>
        <b>Só você vê. Nunca aparece para os jogadores.</b>
      </p>
      <mat-form-field appearance="outline" subscriptSizing="dynamic">
        <textarea
          matInput
          rows="5"
          aria-labelledby="hf-title"
          placeholder="Quem levou o mercador? Onde ficam os goblins? O que a cena esconde?"
          [formControl]="control()"
        ></textarea>
        @if (error(); as message) {
          <mat-error>{{ message }}</mat-error>
        } @else {
          <mat-hint>Markdown, como o documento da campanha.</mat-hint>
        }
        <mat-hint align="end" [class.hf__over]="over()">{{ counter() }}</mat-hint>
      </mat-form-field>
      <app-fiction-notice />
    </section>
  `,
  styles: `
    :host {
      display: block;
    }

    .hf {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-2);
    }

    .hf__title {
      margin: 0;
      font-family: var(--mr-font-display);
      font-size: 21px;
      font-weight: 700;
      line-height: 26px;
    }

    // The lock and the sentence come first: nobody mistakes this for the description.
    .hf__lock {
      display: flex;
      align-items: center;
      gap: var(--mr-space-2);
      margin: 0;
      padding: 10px 12px;
      border-radius: var(--mr-radius-md);
      background: var(--mr-ground);
      font-size: 14px;
      line-height: 19px;

      .mat-icon {
        flex: none;
        width: 18px;
        height: 18px;
        font-size: 18px;
      }
    }

    .hf__over {
      color: var(--mr-danger-ink);
      font-weight: 700;
    }
  `,
})
export class HooksField {
  readonly control = input.required<FormControl<string>>();
  readonly error = input<string | undefined>(undefined);

  /** How many characters the draft has now (the panel counts them). */
  readonly length = input(0);

  protected readonly over = computed(() => this.length() > HOOKS_MAX);
  protected readonly counter = computed(
    () => `${formatInt(this.length())} de ${formatInt(HOOKS_MAX)}`,
  );
}
