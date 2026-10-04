import { Component, ElementRef, OnInit, afterNextRender, input, output, signal, viewChild } from '@angular/core';
import { AbstractControl, FormControl, ReactiveFormsModule, ValidationErrors, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import { MILESTONE_MAX } from '../../../core/progression/milestones';
import { FictionNotice } from '../../../shared/fiction-notice/fiction-notice';

const filled = (control: AbstractControl): ValidationErrors | null =>
  String(control.value ?? '').trim() ? null : { required: true };

/**
 * The name of a milestone, in place: "Novo marco" under the list, or the
 * pencil's "Editar marco" in the place of the row (E8-14). One line, up to
 * 120 characters, with its counter ("35 de 120") and the fiction notice the
 * privacy checklist asks for. Enter saves, "Cancelar" closes without
 * saving; a missing or too long name says so under the field, with an icon
 * and words, and the focus goes back to the field. The server's refusal
 * (`error`) reads the same way. The field takes the focus when the form opens.
 */
@Component({
  selector: 'app-milestone-name-form',
  imports: [FictionNotice, MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule, ReactiveFormsModule],
  template: `
    <form class="form" novalidate (submit)="submit($event)">
      <h4 class="form__title">{{ title() }}</h4>
      <mat-form-field appearance="outline" subscriptSizing="dynamic" class="form__field">
        <mat-label>Nome do marco</mat-label>
        <input
          #field
          matInput
          type="text"
          autocomplete="off"
          [formControl]="control"
          [attr.aria-describedby]="message() ? 'name-error' : null"
          [attr.aria-invalid]="message() ? 'true' : null"
        />
        <mat-hint align="end">{{ control.value.length }} de {{ max }}</mat-hint>
      </mat-form-field>
      <app-fiction-notice />
      @if (message()) {
        <p class="mr-notice mr-notice--danger form__error" id="name-error" role="alert">
          <mat-icon aria-hidden="true">error_outline</mat-icon>{{ message() }}
        </p>
      }
      <div class="form__btns">
        <button mat-stroked-button type="submit" class="form__btn form__go" [attr.aria-disabled]="busy()">
          {{ submitLabel() }}
        </button>
        <button mat-stroked-button type="button" class="form__btn" (click)="cancel.emit()">Cancelar</button>
      </div>
    </form>
  `,
  styleUrl: './milestone-name-form.scss',
})
export class MilestoneNameForm implements OnInit {
  readonly title = input.required<string>();
  /** The text the field starts with: empty for a new milestone. */
  readonly initial = input('');
  readonly submitLabel = input.required<string>();
  readonly busy = input(false);
  /** What the server said when it refused (the host maps the code to words). */
  readonly error = input('');

  /** The cleaned name, to add or to save. */
  readonly save = output<string>();
  readonly cancel = output<void>();

  protected readonly max = MILESTONE_MAX;
  protected readonly control = new FormControl('', {
    nonNullable: true,
    validators: [filled, Validators.maxLength(MILESTONE_MAX)],
  });
  private readonly invalid = signal(false);
  private readonly field = viewChild.required<ElementRef<HTMLInputElement>>('field');

  protected readonly message = () =>
    this.invalid() ? 'Escreva o nome do marco. Ele pode ter até 120 caracteres.' : this.error();

  constructor() {
    afterNextRender(() => {
      const el = this.field().nativeElement;
      // The whole form in view, below the sticky bar, then the field.
      el.closest('form')?.scrollIntoView?.({ block: 'nearest' });
      el.focus({ preventScroll: true });
      el.setSelectionRange(el.value.length, el.value.length);
    });
    this.control.valueChanges.subscribe(() => this.invalid.set(false));
  }

  ngOnInit(): void {
    this.control.setValue(this.initial());
  }

  protected submit(event: Event): void {
    event.preventDefault(); // the form never navigates
    if (this.busy()) {
      return;
    }
    if (this.control.invalid) {
      this.control.markAsTouched();
      this.invalid.set(true);
      this.field().nativeElement.focus();
      return;
    }
    this.save.emit(this.control.value.trim());
  }
}
