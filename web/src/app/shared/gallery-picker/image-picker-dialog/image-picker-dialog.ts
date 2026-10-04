import { Component, ElementRef, Injector, afterNextRender, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialog, MatDialogRef } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import type { GalleryImage } from '../../../../gen/meurpg/maps/v1/gallery_pb';
import { GalleryPicker, PickerTag } from '../gallery-picker';

export interface ImagePickerData {
  readonly campaignId: string;
  readonly title: string;
  /** The filled button: "Mostrar aos jogadores", "Usar esta imagem". */
  readonly confirmLabel: string;
  /** A Material icon on the filled button ('cast' for showing an image). */
  readonly confirmIcon?: string;
  /** The image already in use (shown, or the map's): it can't be chosen
   * again, and its tile says so. */
  readonly current: { id: string; name: string } | null;
  /** The tag on the current image's tile ("À mostra agora"). */
  readonly currentTag: string;
  /** What to say when the current image is checked ("Essa imagem já está à mostra."). */
  readonly currentNote: string;
  /** What to say under the grid for any other image, given the one checked. */
  readonly note: (image: GalleryImage | null, current: { id: string; name: string } | null) => string;
  /** Images that are the background of a hidden map, with the map's name. */
  readonly hiddenMapImages: ReadonlyMap<string, string>;
  readonly emptyError: string;
  /** A line under the title ("Toque numa imagem da galeria da campanha."). */
  readonly lead?: string;
  /** The icon of the line under the grid (default: the eye). */
  readonly noteIcon?: string;
  /** With nothing chosen the filled button is the dashed, off one (`aria-disabled`,
   * a ⊘) and `emptyError` is a plain line above the actions instead of an error
   * after a press; "Cancelar" is outlined. The portrait picker (E8-08). */
  readonly dashedUntilPicked?: boolean;
  /** Does the work; a rejection becomes `errorMessage` in the dialog. */
  readonly submit: (image: GalleryImage) => Promise<void>;
  readonly errorMessage: (err: unknown) => string;
}

/**
 * The picker of a gallery image in a dialog (E5-10, E5-11): "Mostrar uma
 * imagem aos jogadores" on the session page, and "Trocar imagem" on a map.
 * A 720px `MatDialog` from a tablet up, full screen on a phone; the header
 * and the actions stay put and the grid scrolls. Esc and "Fechar" close it.
 *
 * - The tiles are the gallery picker's radiogroup (arrows move), 3 columns
 *   (2 on a phone), with the "Enviar imagem" tile last.
 * - A tile whose image is the background of a hidden map says "Fundo de
 *   mapa escondido" with the eye-off icon; the image can still be chosen,
 *   and when it is, the line under the grid says it does not reveal the map.
 * - The filled button is enabled with nothing checked: pressing it says
 *   "Escolha uma imagem…" (the form pattern); with the current image
 *   checked it is disabled and the line says why. With `dashedUntilPicked`
 *   it is the dashed, off button with the phrase above it until a tile is
 *   checked.
 * - The dialog closes with `true` once `submit` worked.
 */
@Component({
  selector: 'app-image-picker-dialog',
  imports: [GalleryPicker, MatButtonModule, MatIconModule],
  templateUrl: './image-picker-dialog.html',
  styleUrl: './image-picker-dialog.scss',
})
export class ImagePickerDialog {
  protected readonly data = inject<ImagePickerData>(MAT_DIALOG_DATA);
  private readonly ref = inject<MatDialogRef<ImagePickerDialog, boolean>>(MatDialogRef);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  protected readonly selectedId = signal<string | null>(this.data.current?.id ?? null);
  protected readonly picked = signal<GalleryImage | null>(null);
  protected readonly error = signal<string | null>(null);
  protected readonly busy = signal(false);

  protected readonly tags = computed(() => {
    const tags = new Map<string, PickerTag>();
    for (const id of this.data.hiddenMapImages.keys()) {
      tags.set(id, { icon: 'visibility_off', text: 'Fundo de mapa escondido' });
    }
    if (this.data.current) {
      tags.set(this.data.current.id, { icon: 'cast', text: this.data.currentTag });
    }
    return tags;
  });

  protected readonly isCurrent = computed(
    () => this.data.current !== null && this.selectedId() === this.data.current.id,
  );

  /** The line under the grid (eye icon). */
  protected readonly note = computed(() => {
    const image = this.picked();
    if (this.isCurrent()) {
      return this.data.currentNote;
    }
    const base = this.data.note(image, this.data.current);
    const map = image ? this.data.hiddenMapImages.get(image.id) : undefined;
    return map ? `${base} Mostrar essa imagem não revela o mapa ${map}.` : base;
  });

  /** The dashed variant, with nothing chosen yet. */
  protected readonly waiting = computed(() => !!this.data.dashedUntilPicked && !this.picked());

  protected choose(image: GalleryImage): void {
    this.picked.set(image);
    this.error.set(null);
  }

  /** The gallery arrived: focus goes to the checked tile, or the first. */
  protected onLoaded(): void {
    afterNextRender(
      () =>
        this.host.nativeElement.querySelector<HTMLElement>('[role="radio"][tabindex="0"]')?.focus(),
      { injector: this.injector },
    );
  }

  protected async confirm(): Promise<void> {
    const image = this.picked();
    if (!image) {
      if (!this.data.dashedUntilPicked) {
        this.error.set(this.data.emptyError);
      }
      return;
    }
    if (this.isCurrent() || this.busy()) {
      return;
    }
    this.busy.set(true);
    try {
      await this.data.submit(image);
      this.ref.close(true);
    } catch (err) {
      this.busy.set(false);
      this.error.set(this.data.errorMessage(err));
    }
  }

  protected close(): void {
    this.ref.close(false);
  }
}

/** Opens the picker: full screen on a phone, 720px from a tablet up. */
export function openImagePicker(
  dialog: MatDialog,
  data: ImagePickerData,
  injector: Injector,
  phone: boolean,
  restoreFocus: boolean | HTMLElement = true,
) {
  return dialog.open<ImagePickerDialog, ImagePickerData, boolean>(ImagePickerDialog, {
    data,
    injector,
    width: phone ? '100vw' : '720px',
    maxWidth: phone ? '100vw' : 'calc(100vw - 32px)',
    height: phone ? '100dvh' : undefined,
    maxHeight: phone ? '100dvh' : '90dvh',
    panelClass: phone ? 'image-picker--full' : 'image-picker',
    ariaLabelledBy: 'image-picker-title',
    autoFocus: false,
    restoreFocus,
  });
}
