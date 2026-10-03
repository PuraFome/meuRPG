import { Component, ElementRef, input, viewChild } from '@angular/core';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

import { FictionNotice } from '../fiction-notice/fiction-notice';

/** The longest reason the server takes (`progression.proto`). */
export const REASON_MAX = 120;

/**
 * The one-line text of an award or a milestone ("Motivo", "O que aconteceu"),
 * up to 120 characters, with its counter ("24 de 120") and the fiction
 * notice the privacy checklist asks for under every free-text field. The
 * error shows when the person leaves the field (the control's own
 * `touched`), not on each key.
 */
@Component({
  selector: 'app-xp-reason',
  imports: [FictionNotice, MatFormFieldModule, MatInputModule, ReactiveFormsModule],
  template: `
    <mat-form-field appearance="outline" subscriptSizing="dynamic" class="field">
      <mat-label>{{ label() }}</mat-label>
      <input
        #input
        matInput
        type="text"
        autocomplete="off"
        data-initial-focus
        [maxlength]="max"
        [formControl]="control()"
        [placeholder]="placeholder()"
      />
      <mat-hint>{{ hint() }}</mat-hint>
      <mat-hint align="end">{{ control().value.length }} de {{ max }}</mat-hint>
      <mat-error>{{ errorText() }}</mat-error>
    </mat-form-field>
    <app-fiction-notice />
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-2);
    }

    .field {
      width: 100%;
    }

    // "24 de 120" is one thing: it never breaks.
    mat-hint.mat-mdc-form-field-hint-end {
      white-space: nowrap;
    }
  `,
})
export class XpReason {
  readonly control = input.required<FormControl<string>>();
  readonly label = input('Motivo');
  readonly hint = input('O motivo aparece no histórico de XP.');
  readonly errorText = input('Escreva o motivo do XP.');
  readonly placeholder = input('');

  protected readonly max = REASON_MAX;
  private readonly field = viewChild.required<ElementRef<HTMLInputElement>>('input');

  focus(): void {
    this.field().nativeElement.focus();
  }
}
