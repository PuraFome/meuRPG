import { Component, inject } from '@angular/core';
import { trigger, transition, style, animate } from '@angular/animations';
import { MatDialogModule, MatDialogRef, MAT_DIALOG_DATA } from '@angular/material/dialog';
import { MatButtonModule } from '@angular/material/button';
import { ImageCropComponent } from '../../shared/components/image-crop.component';

@Component({
  selector: 'app-avatar-crop-dialog',
  standalone: true,
  imports: [MatDialogModule, MatButtonModule, ImageCropComponent],
  template: `
    <h2 mat-dialog-title>Recortar Avatar</h2>
    <mat-dialog-content>
      <app-image-crop
        [imageSrc]="imageSrc"
        [aspectRatio]="1"
        (cropComplete)="onCropComplete($event)"
      />
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button (click)="onCancel()">Cancelar</button>
    </mat-dialog-actions>
  `,
  styles: [
    `
      :host { display: block; }
      mat-dialog-content {
        min-width: min(360px, 90vw);
        min-height: min(200px, 50vh);
      }
    `,
  ],
  animations: [
    trigger('fadeSlide', [
      transition(':enter', [
        style({ opacity: 0, transform: 'translateY(-12px)' }),
        animate('250ms ease-out', style({ opacity: 1, transform: 'translateY(0)' })),
      ]),
    ]),
  ],
  host: { '[@fadeSlide]': '' },
})
export class AvatarCropDialogComponent {
  private readonly dialogRef = inject(MatDialogRef<AvatarCropDialogComponent>);
  readonly imageSrc = inject<string>(MAT_DIALOG_DATA);

  onCropComplete(result: Blob | string): void {
    if (typeof result === 'string') {
      this.dialogRef.close(result);
    }
  }

  onCancel(): void {
    this.dialogRef.close();
  }
}
