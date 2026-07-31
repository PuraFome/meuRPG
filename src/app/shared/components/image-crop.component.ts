import {
  Component,
  ElementRef,
  EventEmitter,
  Input,
  Output,
  ViewChild,
  AfterViewInit,
  OnDestroy,
  signal,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

interface CropRegion {
  x: number;
  y: number;
  width: number;
  height: number;
}

@Component({
  selector: 'app-image-crop',
  standalone: true,
  imports: [MatButtonModule, MatIconModule],
  template: `
    <div class="crop-container">
      @if (imageSrc) {
        <div
          class="image-wrapper"
          #wrapperEl
          (mousedown)="onMouseDown($event)"
          (mousemove)="onMouseMove($event)"
          (mouseup)="onMouseUp()"
          (mouseleave)="onMouseUp()"
        >
          <img
            #imgEl
            [src]="imageSrc"
            class="crop-image"
            (load)="onImageLoad()"
            alt="Imagem para recorte"
            draggable="false"
          />
          @if (cropRegion()) {
            <div
              class="crop-overlay"
              [style.left.px]="cropRegion()!.x"
              [style.top.px]="cropRegion()!.y"
              [style.width.px]="cropRegion()!.width"
              [style.height.px]="cropRegion()!.height"
            ></div>
          }
        </div>

        @if (cropRegion()) {
          <div class="actions">
            <button mat-raised-button color="primary" (click)="applyCrop()">
              <mat-icon>crop</mat-icon>
              Aplicar Corte
            </button>
            <button mat-button (click)="resetCrop()">Cancelar</button>
          </div>
        }

        @if (previewUrl()) {
          <div class="preview-section">
            <p class="preview-label">Pré-visualização</p>
            <img [src]="previewUrl()" class="preview" alt="Preview do recorte" />
          </div>
        }
      } @else {
        <div class="no-image">
          <mat-icon class="no-image-icon">image</mat-icon>
          <p>Nenhuma imagem selecionada</p>
        </div>
      }
    </div>
  `,
  styles: [
    `
      .crop-container {
        max-width: 100%;
      }
      .image-wrapper {
        position: relative;
        display: inline-block;
        cursor: crosshair;
        line-height: 0;
      }
      .crop-image {
        max-width: 100%;
        max-height: 60vh;
        user-select: none;
        -webkit-user-drag: none;
      }
      .crop-overlay {
        position: absolute;
        border: 2px dashed #ce93d8;
        background: rgba(206, 147, 216, 0.15);
        pointer-events: none;
        box-sizing: border-box;
      }
      .actions {
        display: flex;
        gap: 8px;
        margin-top: 12px;
        align-items: center;
      }
      .preview-section {
        margin-top: 16px;
      }
      .preview-label {
        font-size: 0.85rem;
        opacity: 0.6;
        margin: 0 0 8px;
      }
      .preview {
        max-width: 200px;
        max-height: 200px;
        border-radius: 8px;
        border: 1px solid rgba(255, 255, 255, 0.12);
      }
      .no-image {
        display: flex;
        flex-direction: column;
        align-items: center;
        padding: 48px;
        opacity: 0.5;
        text-align: center;
      }
      .no-image-icon {
        font-size: 3rem;
        width: 3rem;
        height: 3rem;
        margin-bottom: 8px;
      }
      .no-image p {
        margin: 0;
        font-size: 0.95rem;
      }
    `,
  ],
})
export class ImageCropComponent implements AfterViewInit, OnDestroy {
  @Input() imageSrc = '';
  @Input() aspectRatio?: number;
  @Output() cropComplete = new EventEmitter<Blob | string>();

  @ViewChild('imgEl') imgEl!: ElementRef<HTMLImageElement>;
  @ViewChild('wrapperEl') wrapperEl!: ElementRef<HTMLElement>;

  readonly cropRegion = signal<CropRegion | null>(null);
  readonly previewUrl = signal<string | null>(null);

  private image!: HTMLImageElement;
  private isDragging = false;
  private startX = 0;
  private startY = 0;
  private canvasEl = document.createElement('canvas');
  private ctx = this.canvasEl.getContext('2d')!;

  ngAfterViewInit(): void {
    // Canvas context is set up in constructor
  }

  ngOnDestroy(): void {
    const url = this.previewUrl();
    if (url) URL.revokeObjectURL(url);
  }

  onImageLoad(): void {
    this.image = this.imgEl.nativeElement;
    this.resetCrop();
  }

  onMouseDown(event: MouseEvent): void {
    if (!this.image) return;
    this.isDragging = true;
    const rect = this.wrapperEl.nativeElement.getBoundingClientRect();
    this.startX = event.clientX - rect.left;
    this.startY = event.clientY - rect.top;
    this.cropRegion.set({ x: this.startX, y: this.startY, width: 0, height: 0 });
  }

  onMouseMove(event: MouseEvent): void {
    if (!this.isDragging) return;
    const rect = this.wrapperEl.nativeElement.getBoundingClientRect();
    let x = this.startX;
    let y = this.startY;
    let w = event.clientX - rect.left - x;
    let h = event.clientY - rect.top - y;

    // Constrain aspect ratio if set
    if (this.aspectRatio && this.aspectRatio > 0) {
      const absW = Math.abs(w);
      const absH = Math.abs(h);
      if (absW / absH > this.aspectRatio) {
        const sign = h >= 0 ? 1 : -1;
        h = sign * (absW / this.aspectRatio);
      } else {
        const sign = w >= 0 ? 1 : -1;
        w = sign * (absH * this.aspectRatio);
      }
    }

    // Normalize negative dimensions
    if (w < 0) {
      x = x + w;
      w = -w;
    }
    if (h < 0) {
      y = y + h;
      h = -h;
    }

    this.cropRegion.set({ x, y, width: w, height: h });
  }

  onMouseUp(): void {
    this.isDragging = false;
  }

  applyCrop(): void {
    const region = this.cropRegion();
    if (!region || region.width < 1 || region.height < 1) return;

    const img = this.imgEl.nativeElement;
    const wrapper = this.wrapperEl.nativeElement;
    const wrapperRect = wrapper.getBoundingClientRect();
    const imgRect = img.getBoundingClientRect();

    // Scale factor between displayed size and natural size
    const scaleX = img.naturalWidth / imgRect.width;
    const scaleY = img.naturalHeight / imgRect.height;

    // Offset of image within wrapper
    const offsetX = imgRect.left - wrapperRect.left;
    const offsetY = imgRect.top - wrapperRect.top;

    const sx = (region.x - offsetX) * scaleX;
    const sy = (region.y - offsetY) * scaleY;
    const sw = region.width * scaleX;
    const sh = region.height * scaleY;

    // Clamp to image bounds
    const clampedSx = Math.max(0, sx);
    const clampedSy = Math.max(0, sy);
    const clampedSw = Math.min(sw, img.naturalWidth - clampedSx);
    const clampedSh = Math.min(sh, img.naturalHeight - clampedSy);

    if (clampedSw < 1 || clampedSh < 1) return;

    this.canvasEl.width = clampedSw;
    this.canvasEl.height = clampedSh;

    this.ctx.drawImage(
      img,
      clampedSx,
      clampedSy,
      clampedSw,
      clampedSh,
      0,
      0,
      clampedSw,
      clampedSh,
    );

    // Emit data URL
    const dataUrl = this.canvasEl.toDataURL('image/png');
    this.previewUrl.set(dataUrl);
    this.cropComplete.emit(dataUrl);
  }

  resetCrop(): void {
    this.cropRegion.set(null);
    this.previewUrl.set(null);
  }
}
