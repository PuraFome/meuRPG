import { Component, Inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  MatDialogModule,
  MatDialogRef,
  MAT_DIALOG_DATA,
} from '@angular/material/dialog';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { DomSanitizer, SafeResourceUrl } from '@angular/platform-browser';

export interface CampaignGuideDialogData {
  docId: string;
  editUrl: string;
  folderName: string;
}

@Component({
  selector: 'app-campaign-guide-dialog',
  standalone: true,
  imports: [CommonModule, MatDialogModule, MatButtonModule, MatIconModule],
  template: `
    <h2 mat-dialog-title>
      <mat-icon class="title-icon">description</mat-icon>
      Guia da campanha
    </h2>

    <mat-dialog-content class="dialog-content">
      <p class="folder-name">{{ data.folderName }}</p>

      <div class="preview-frame">
        <iframe
          [src]="previewUrl"
          title="Guia da campanha (Google Docs)"
          class="guide-iframe"
        ></iframe>
      </div>

      <p class="edit-hint">
        O preview é somente leitura. Para editar e salvar no Drive, abra o
        documento no Google Docs.
      </p>
    </mat-dialog-content>

    <mat-dialog-actions align="end">
      <button mat-button (click)="close()">Fechar</button>
      <a
        mat-raised-button
        color="primary"
        [href]="data.editUrl"
        target="_blank"
        rel="noopener noreferrer"
      >
        <mat-icon>edit_document</mat-icon>
        Editar no Google Docs
      </a>
    </mat-dialog-actions>
  `,
  styles: [
    `
      .title-icon {
        vertical-align: -4px;
        margin-right: 4px;
        opacity: 0.8;
      }
      .dialog-content {
        min-width: min(760px, 86vw);
      }
      .folder-name {
        margin: 0 0 12px;
        opacity: 0.65;
        font-size: 0.9rem;
      }
      .preview-frame {
        border: 1px solid rgba(255, 255, 255, 0.12);
        border-radius: 8px;
        overflow: hidden;
        background: #fff;
        height: min(62vh, 560px);
      }
      .guide-iframe {
        width: 100%;
        height: 100%;
        border: none;
        display: block;
      }
      .edit-hint {
        margin: 12px 0 0;
        font-size: 0.8rem;
        opacity: 0.6;
      }
    `,
  ],
})
export class CampaignGuideDialogComponent {
  readonly previewUrl: SafeResourceUrl;

  constructor(
    public readonly dialogRef: MatDialogRef<CampaignGuideDialogComponent>,
    @Inject(MAT_DIALOG_DATA) public readonly data: CampaignGuideDialogData,
    sanitizer: DomSanitizer,
  ) {
    this.previewUrl = sanitizer.bypassSecurityTrustResourceUrl(
      `https://docs.google.com/document/d/${data.docId}/preview`,
    );
  }

  close() {
    this.dialogRef.close();
  }
}
