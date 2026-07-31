import {
  Component,
  EventEmitter,
  Input,
  Output,
  signal,
} from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';

@Component({
  selector: 'app-file-upload',
  standalone: true,
  imports: [MatIconModule, MatButtonModule],
  template: `
    <div
      class="dropzone"
      [class.drag-over]="isDragOver()"
      (dragenter)="onDragEnter($event)"
      (dragover)="onDragOver($event)"
      (dragleave)="onDragLeave($event)"
      (drop)="onDrop($event)"
      (click)="fileInput.click()"
      role="button"
      tabindex="0"
      (keydown.enter)="fileInput.click()"
      (keydown.space)="fileInput.click(); $event.preventDefault()"
    >
      <input
        #fileInput
        type="file"
        hidden
        (change)="onFileSelected($event)"
        [accept]="acceptString()"
      />
      <mat-icon class="upload-icon">cloud_upload</mat-icon>
      <p class="upload-text">
        Arraste arquivos aqui ou clique para selecionar
      </p>
      <span class="upload-hint"
        >{{ acceptedTypes.join(', ') || 'Todos os tipos' }} •
        Máx {{ maxSize / (1024 * 1024) }} MB</span
      >
    </div>

    @if (previewUrl()) {
      <div class="preview-wrapper">
        <img [src]="previewUrl()" class="preview" alt="Preview" />
        <button
          mat-icon-button
          class="remove-btn"
          (click)="clearFile(); $event.stopPropagation()"
          aria-label="Remover arquivo"
        >
          <mat-icon>close</mat-icon>
        </button>
      </div>
    }

    @if (errorMessage()) {
      <div class="error-message">
        <mat-icon class="error-icon">error_outline</mat-icon>
        {{ errorMessage() }}
      </div>
    }
  `,
  styles: [
    `
      .dropzone {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 32px 16px;
        border: 2px dashed rgba(255, 255, 255, 0.23);
        border-radius: 8px;
        cursor: pointer;
        transition: border-color 0.2s, background-color 0.2s;
        text-align: center;
        outline: none;
      }
      .dropzone:hover,
      .dropzone:focus-visible {
        border-color: rgba(255, 255, 255, 0.5);
        background-color: rgba(255, 255, 255, 0.03);
      }
      .dropzone.drag-over {
        border-color: #ce93d8;
        background-color: rgba(206, 147, 216, 0.08);
      }
      .upload-icon {
        font-size: 2.5rem;
        width: 2.5rem;
        height: 2.5rem;
        margin-bottom: 12px;
        opacity: 0.5;
      }
      .upload-text {
        margin: 0 0 4px;
        font-size: 0.95rem;
        opacity: 0.8;
      }
      .upload-hint {
        font-size: 0.8rem;
        opacity: 0.45;
      }
      .preview-wrapper {
        position: relative;
        margin-top: 12px;
        display: inline-block;
      }
      .preview {
        max-width: min(200px, 100%);
        max-height: min(200px, 50vh);
        object-fit: contain;
        border-radius: 8px;
        display: block;
      }
      .remove-btn {
        position: absolute;
        top: -8px;
        right: -8px;
        background: rgba(0, 0, 0, 0.6);
      }
      .error-message {
        display: flex;
        align-items: center;
        gap: 6px;
        margin-top: 8px;
        font-size: 0.85rem;
        color: #f44336;
      }
      .error-icon {
        font-size: 1.1rem;
        width: 1.1rem;
        height: 1.1rem;
      }
    `,
  ],
})
export class FileUploadComponent {
  @Input() acceptedTypes: string[] = [];
  @Input() maxSize = 10 * 1024 * 1024; // 10 MB
  @Output() fileChange = new EventEmitter<File>();

  readonly isDragOver = signal(false);
  readonly previewUrl = signal<string | null>(null);
  readonly errorMessage = signal<string | null>(null);

  private selectedFile: File | null = null;

  get acceptString(): () => string {
    const fn = () => this.acceptedTypes?.join(',') ?? '';
    return fn;
  }

  onDragEnter(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.isDragOver.set(true);
  }

  onDragOver(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.isDragOver.set(true);
  }

  onDragLeave(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.isDragOver.set(false);
  }

  onDrop(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.isDragOver.set(false);

    const files = event.dataTransfer?.files;
    if (files?.length) {
      this.handleFile(files[0]);
    }
  }

  onFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    if (input.files?.length) {
      this.handleFile(input.files[0]);
    }
    input.value = '';
  }

  private handleFile(file: File): void {
    this.errorMessage.set(null);

    // Validate type
    if (
      this.acceptedTypes?.length &&
      !this.acceptedTypes.some((t) => file.type.match(t))
    ) {
      this.errorMessage.set(
        `Tipo de arquivo não suportado: ${file.type || 'desconhecido'}`,
      );
      return;
    }

    // Validate size
    if (file.size > this.maxSize) {
      const maxMB = this.maxSize / (1024 * 1024);
      this.errorMessage.set(
        `Arquivo muito grande. Máximo permitido: ${maxMB} MB`,
      );
      return;
    }

    this.selectedFile = file;
    this.fileChange.emit(file);

    // Show preview for images
    if (file.type.startsWith('image/')) {
      const url = URL.createObjectURL(file);
      this.previewUrl.set(url);
    } else {
      this.previewUrl.set(null);
    }
  }

  clearFile(): void {
    if (this.previewUrl()) {
      URL.revokeObjectURL(this.previewUrl()!);
    }
    this.selectedFile = null;
    this.previewUrl.set(null);
    this.errorMessage.set(null);
  }
}
