import { Component, Input, forwardRef, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  ControlValueAccessor,
  NG_VALUE_ACCESSOR,
  ReactiveFormsModule,
} from '@angular/forms';
import { MatDialog } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatSelectModule } from '@angular/material/select';
import { DndOptionsService, type DndOptionCategory } from './dnd-options.service';
import { DndOptionDialogComponent } from './dnd-option-dialog.component';

const CUSTOM_OPTION = '__custom__';

@Component({
  selector: 'app-dnd-option-select',
  standalone: true,
  imports: [
    CommonModule,
    ReactiveFormsModule,
    MatFormFieldModule,
    MatIconModule,
    MatSelectModule,
  ],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => DndOptionSelectComponent),
      multi: true,
    },
  ],
  template: `
    <mat-form-field appearance="outline" subscriptSizing="dynamic">
      <mat-label>{{ label }}</mat-label>
      <mat-select [value]="value" (selectionChange)="onSelect($event.value)">
        @for (option of options; track option) {
          <mat-option [value]="option">{{ option }}</mat-option>
        }
        <mat-option [value]="CUSTOM_OPTION" class="custom-option">
          <mat-icon>add</mat-icon>
          Cadastrar nova {{ label.toLowerCase() }}...
        </mat-option>
      </mat-select>
    </mat-form-field>
  `,
  styles: [
    `
      .custom-option {
        color: #a78bfa;
        font-weight: 500;
      }
      .custom-option mat-icon {
        font-size: 18px;
        width: 18px;
        height: 18px;
        line-height: 18px;
        margin-right: 4px;
        vertical-align: middle;
      }
    `,
  ],
})
export class DndOptionSelectComponent implements ControlValueAccessor {
  @Input() label = '';
  @Input() category: DndOptionCategory = 'race';

  protected readonly CUSTOM_OPTION = CUSTOM_OPTION;

  private readonly optionsService = inject(DndOptionsService);
  private readonly dialog = inject(MatDialog);

  value: string | null = null;
  private onChange: (value: string | null) => void = () => {};
  private onTouched: () => void = () => {};

  /** All registered options, always including the current value if custom. */
  get options(): string[] {
    const base = this.optionsService.getOptions(this.category);
    if (this.value && !base.includes(this.value)) {
      return [...base, this.value];
    }
    return base;
  }

  writeValue(value: string | null): void {
    this.value = value;
  }

  registerOnChange(fn: (value: string | null) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  onSelect(value: string | null): void {
    if (value === CUSTOM_OPTION) {
      this.openRegisterDialog();
      return;
    }
    this.value = value;
    this.onChange(value);
    this.onTouched();
  }

  private openRegisterDialog(): void {
    const dialogRef = this.dialog.open(DndOptionDialogComponent, {
      data: { categoryLabel: this.label },
      width: '400px',
      maxWidth: '95vw',
    });

    dialogRef.afterClosed().subscribe((newValue?: string) => {
      if (!newValue) return;
      this.optionsService.addOption(this.category, newValue);
      this.value = newValue;
      this.onChange(newValue);
      this.onTouched();
    });
  }
}
