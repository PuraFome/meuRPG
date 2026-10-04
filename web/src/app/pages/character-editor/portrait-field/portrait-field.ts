import {
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  input,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { toObservable, toSignal } from '@angular/core/rxjs-interop';
import type { FormControl } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { startWith, switchMap } from 'rxjs';

import { GalleryClient } from '../../../core/images/gallery-client';
import { openImagePicker } from '../../../shared/gallery-picker/image-picker-dialog/image-picker-dialog';
import { PHONE_QUERY, mediaQuery } from '../../../shared/map-view/media-query';
import { Portrait } from '../../../shared/portrait/portrait';

/**
 * "Retrato" in the NPC editor (MR-031, E8-08): the NPC's portrait, a gallery
 * image, in a 200px frame 5 wide by 6 tall. Without one the frame shows the
 * NPC's initials ("AL"; "CG" for two words) and the caption says so.
 *
 * - "Escolher retrato" / "Trocar retrato" opens the gallery picker (the one
 *   "Mostrar imagem" uses) with nothing chosen: its filled button is dashed
 *   until a tile is picked. Choosing only changes this field; the NPC saves
 *   with the form's "Salvar alterações".
 * - "Remover retrato" (a text button) asks in place: "Remover o retrato de
 *   Mira?", "A imagem continua na galeria.", then "Voltar" (first, and focused)
 *   and "Remover retrato", stacked and as wide as the box. Removing returns to
 *   the initials; it also only applies on save.
 * - The control holds the image's ID, as `portrait_image_id`: the form carries
 *   it through every rebuild, so saving never loses it.
 *
 * Focus: the question opens on "Voltar"; "Voltar" gives it back to "Remover
 * retrato"; removing sends it to "Escolher retrato". The picker's own dialog
 * returns it to the button that opened it.
 */
@Component({
  selector: 'app-portrait-field',
  imports: [MatButtonModule, MatIconModule, Portrait],
  templateUrl: './portrait-field.html',
  styleUrl: './portrait-field.scss',
})
export class PortraitField {
  private readonly dialog = inject(MatDialog);
  private readonly gallery = inject(GalleryClient);
  private readonly injector = inject(Injector);
  private readonly phone = mediaQuery(PHONE_QUERY);

  /** The form control that holds the image's ID (`''` for none). */
  readonly control = input.required<FormControl<string>>();
  /** The NPC's name control: the initials, the title and the question use it. */
  readonly nameControl = input.required<FormControl<string>>();
  readonly campaignId = input.required<string>();

  protected readonly imageId = toSignal(
    toObservable(this.control).pipe(switchMap((c) => c.valueChanges.pipe(startWith(c.value)))),
    { initialValue: '' },
  );
  private readonly typedName = toSignal(
    toObservable(this.nameControl).pipe(switchMap((c) => c.valueChanges.pipe(startWith(c.value)))),
    { initialValue: '' },
  );
  /** The name of the gallery image, for the caption: known after a pick, or read
   * from the gallery for a portrait the sheet already had. */
  private readonly imageName = signal('');
  protected readonly removing = signal(false);

  protected readonly name = computed(() => this.typedName().trim() || 'o NPC');
  protected readonly url = computed(() => (this.imageId() ? `/images/${this.imageId()}` : ''));
  protected readonly caption = computed(() => {
    if (!this.imageId()) {
      return 'Sem retrato: aparecem as iniciais.';
    }
    return this.imageName() ? `Imagem da galeria: “${this.imageName()}”` : 'Imagem da galeria';
  });

  private readonly pickButton = viewChild('pickBtn', { read: ElementRef<HTMLElement> });
  private readonly removeButton = viewChild('removeBtn', { read: ElementRef<HTMLElement> });
  private readonly question = viewChild<ElementRef<HTMLElement>>('question');
  private readonly backButton = viewChild('backBtn', { read: ElementRef<HTMLElement> });

  constructor() {
    effect(() => {
      const id = this.imageId();
      untracked(() => void this.lookUpName(id));
    });
  }

  private async lookUpName(id: string): Promise<void> {
    if (!id || this.imageName()) {
      return;
    }
    try {
      const { images } = await this.gallery.list(this.campaignId());
      if (this.imageId() === id) {
        this.imageName.set(images.find((i) => i.id === id)?.name ?? '');
      }
    } catch {
      // The caption keeps its short form.
    }
  }

  protected choose(): void {
    openImagePicker(
      this.dialog,
      {
        campaignId: this.campaignId(),
        title: `Escolher o retrato de ${this.name()}`,
        lead: 'Toque numa imagem da galeria da campanha.',
        confirmLabel: 'Usar este retrato',
        current: null,
        currentTag: '',
        currentNote: '',
        // One hint only: the dashed button and "Escolha uma imagem." already say it.
        note: () => '',
        hiddenMapImages: new Map(),
        emptyError: 'Escolha uma imagem.',
        dashedUntilPicked: true,
        submit: async (image) => {
          this.control().setValue(image.id);
          this.control().markAsDirty();
          this.imageName.set(image.name);
        },
        errorMessage: () => 'Não deu para usar essa imagem. Tente de novo.',
      },
      this.injector,
      this.phone(),
    );
  }

  protected ask(): void {
    this.removing.set(true);
    afterNextRender(
      () => {
        this.question()?.nativeElement.scrollIntoView?.({ block: 'nearest' });
        this.backButton()?.nativeElement.focus({ preventScroll: true });
      },
      { injector: this.injector },
    );
  }

  protected back(): void {
    this.removing.set(false);
    afterNextRender(() => this.removeButton()?.nativeElement.focus(), { injector: this.injector });
  }

  protected remove(): void {
    this.control().setValue('');
    this.control().markAsDirty();
    this.imageName.set('');
    this.removing.set(false);
    afterNextRender(() => this.pickButton()?.nativeElement.focus(), { injector: this.injector });
  }
}
