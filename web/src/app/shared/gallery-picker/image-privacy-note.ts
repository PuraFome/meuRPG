import { Component } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/** The reminder docs/privacidade.md asks for next to every image upload:
 * an image can be a photo of a real person. */
export const IMAGE_PRIVACY_REMINDER =
  'Use imagens do jogo. Não envie fotos de pessoas sem a autorização delas.';

/**
 * The privacy reminder next to every "Enviar imagem" (the gallery, its
 * empty state and the gallery picker), always with the same words, like
 * `FictionNotice` next to free text. A quiet line, not a notice: it sits
 * beside the upload, so it must not compete with an upload error.
 */
@Component({
  selector: 'app-image-privacy-note',
  imports: [MatIconModule],
  template: `<mat-icon aria-hidden="true">info</mat-icon><span>{{ text }}</span>`,
  host: { role: 'note', class: 'image-privacy-note' },
  // Icon and text side by side, the text wrapping in its own column (a
  // hanging indent), left-aligned wherever the note sits.
  styles: `
    :host {
      display: flex;
      align-items: flex-start;
      gap: 6px;
      max-width: 100%;
      margin: 0;
      color: var(--mr-ink-muted);
      font-size: 14px;
      line-height: 19px;
      text-align: left;
    }

    .mat-icon {
      flex: none;
      width: 18px;
      height: 18px;
      font-size: 18px;
    }
  `,
})
export class ImagePrivacyNote {
  protected readonly text = IMAGE_PRIVACY_REMINDER;
}
