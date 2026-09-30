import { Component, ElementRef, computed, input, output, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { formatBytes } from '../../../core/images/image-format';
import { ACCEPT_ATTRIBUTE, DEFAULT_LIMITS } from '../../../core/images/upload-errors';
import { ImagePrivacyNote } from '../../../shared/gallery-picker/image-privacy-note';

/**
 * Where the master starts an upload (E5-20, E5-21, E5-22): the screen's one
 * filled button, "Enviar imagem", which opens the file picker (JPEG, PNG or
 * WebP, several at once), the formats line and the privacy reminder.
 *
 * - `bar`: from 768px, the dashed drop zone of E5-20; on a phone, just the
 *   full-width button with the lines under it (E5-21).
 * - `empty`: the empty gallery's panel (E5-22), which invites the first
 *   upload.
 *
 * The drop itself is handled by the page (anywhere on the page works); this
 * only shows it, with the accent border, while `dragging` is on.
 */
@Component({
  selector: 'app-upload-zone',
  imports: [ImagePrivacyNote, MatButtonModule, MatIconModule],
  templateUrl: './upload-zone.html',
  styleUrl: './upload-zone.scss',
})
export class UploadZone {
  readonly variant = input<'bar' | 'empty'>('bar');
  readonly dragging = input(false);
  /** The server's per-image limit, for the formats line. */
  readonly maxImageBytes = input<number>(DEFAULT_LIMITS.maxImageBytes);
  /** The files the master picked, in order. */
  readonly picked = output<File[]>();

  protected readonly accept = ACCEPT_ATTRIBUTE;
  protected readonly hint = computed(
    () => `JPEG, PNG ou WebP, até\u00a0${formatBytes(this.maxImageBytes())}.`,
  );

  private readonly fileInput = viewChild.required<ElementRef<HTMLInputElement>>('fileInput');
  // `read: ElementRef`: on a Material button the reference is the directive.
  private readonly button = viewChild.required('pickButton', {
    read: ElementRef<HTMLButtonElement>,
  });

  /** Opens the system's file picker. */
  open(): void {
    this.fileInput().nativeElement.click();
  }

  /** Puts focus on "Enviar imagem" (after the last image is deleted). */
  focus(): void {
    this.button().nativeElement.focus();
  }

  protected onChange(input: HTMLInputElement): void {
    const files = Array.from(input.files ?? []);
    // Clear it, so picking the same file again still fires `change`.
    input.value = '';
    if (files.length > 0) {
      this.picked.emit(files);
    }
  }
}
