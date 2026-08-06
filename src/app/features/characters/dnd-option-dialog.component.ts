import { Component, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatDialogModule, MatDialogRef, MAT_DIALOG_DATA } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

export interface DndOptionDialogData {
  categoryLabel: string;
}

@Component({
  selector: 'app-dnd-option-dialog',
  standalone: true,
  imports: [
    FormsModule,
    MatButtonModule,
    MatDialogModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
  ],
  template: `
    <h2 mat-dialog-title>Nova {{ data.categoryLabel }}</h2>
    <mat-dialog-content>
      <mat-form-field appearance="outline" subscriptSizing="dynamic" class="full-width">
        <mat-label>{{ data.categoryLabel }}</mat-label>
        <input
          matInput
          [(ngModel)]="name"
          (keydown.enter)="onConfirm()"
          placeholder="Digite o nome..."
        />
      </mat-form-field>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button (click)="onCancel()">Cancelar</button>
      <button
        mat-raised-button
        color="primary"
        [disabled]="!name.trim()"
        (click)="onConfirm()"
      >
        <mat-icon>add</mat-icon>
        Cadastrar
      </button>
    </mat-dialog-actions>
  `,
  styles: [
    `
      :host {
        display: block;
      }
      .full-width {
        width: 100%;
        margin-top: 8px;
      }
    `,
  ],
})
export class DndOptionDialogComponent {
  private readonly dialogRef = inject(MatDialogRef<DndOptionDialogComponent>);
  readonly data = inject<DndOptionDialogData>(MAT_DIALOG_DATA);

  name = '';

  onConfirm(): void {
    const value = this.name.trim();
    if (!value) return;
    this.dialogRef.close(value);
  }

  onCancel(): void {
    this.dialogRef.close();
  }
}
