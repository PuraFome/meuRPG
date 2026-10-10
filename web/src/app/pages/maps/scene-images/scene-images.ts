import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import type { SceneImage } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { tight } from '../../../core/format/text';
import type { GenerateOrigin } from '../../../core/images/imagegen-copy';
import { sceneImagesErrorMessage } from '../../../core/maps/map-errors';
import { MapsClient } from '../../../core/maps/maps-client';
import { openImagePicker } from '../../../shared/gallery-picker/image-picker-dialog/image-picker-dialog';
import type { GenerateOutcome } from '../../../shared/image-generate/image-generate-dialog';
import { GenerateImageButton } from '../../../shared/image-generate/generate-image-button';
import { PHONE_QUERY, mediaQuery } from '../../../shared/map-view/media-query';
import { RetryImage } from '../../../shared/retry-image/retry-image';

/** How many images a scene holds (maps.proto, `SetSceneImages`). */
export const SCENE_IMAGE_LIMIT = 8;

type Control = 'up' | 'down' | 'remove';

/**
 * "Imagens da cena" in the point panel of a SCENE point (MR-015): the gallery
 * images the master prepared for the scene, in order, up to 8. The players never
 * see this list: an image reaches them only when the master shows it in the
 * session. Every change saves at once (the whole list is one call), like the
 * clues; one write at a time. Taking an image off the scene keeps it in the
 * gallery, so it asks nothing.
 *
 * "Escolher da galeria" opens the shared gallery picker; "Gerar imagem com IA"
 * opens the generate dialog (MR-039), and the picture the master ends on joins
 * the list by itself.
 *
 * Focus: after ↑ or ↓ it stays on the same button of the moved row; after a
 * removal it goes to the next row's "Remover", or to "Escolher da galeria".
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-scene-images',
  imports: [GenerateImageButton, MatButtonModule, MatIconModule, RetryImage],
  templateUrl: './scene-images.html',
  styleUrl: './scene-images.scss',
})
export class SceneImages {
  private readonly api = inject(MapsClient);
  private readonly dialog = inject(MatDialog);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly campaignId = input.required<string>();
  readonly mapId = input.required<string>();
  readonly pointId = input.required<string>();
  readonly images = input.required<readonly SceneImage[]>();
  /** The scene's name and whether its point is revealed: only then the name goes to the picture's text (RN-10). */
  readonly name = input.required<string>();
  readonly revealed = input(false);
  /** The point's images as the server has them now (after each write). */
  readonly imagesChange = output<readonly SceneImage[]>();

  protected readonly phone = mediaQuery(PHONE_QUERY);
  protected readonly limit = SCENE_IMAGE_LIMIT;
  protected readonly count = computed(() =>
    tight(`${this.images().length} de ${SCENE_IMAGE_LIMIT}`),
  );
  protected readonly full = computed(() => this.images().length >= SCENE_IMAGE_LIMIT);
  protected readonly origin = computed<GenerateOrigin>(() => ({
    kind: 'scene',
    name: this.name(),
    revealed: this.revealed(),
  }));
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  /** What a screen reader hears after a change. */
  protected readonly status = signal('');

  protected thumb(image: SceneImage): string {
    return `/images/${image.id}/thumb`;
  }

  protected open(): void {
    if (this.full() || this.busy()) {
      return;
    }
    openImagePicker(
      this.dialog,
      {
        campaignId: this.campaignId(),
        title: 'Escolher da galeria',
        lead: 'A imagem fica nesta cena. Os jogadores só a veem quando você a mostra na sessão.',
        confirmLabel: 'Pôr na cena',
        confirmIcon: 'add_photo_alternate',
        current: null,
        currentTag: '',
        currentNote: '',
        note: () => 'A imagem continua na galeria.',
        noteIcon: 'info',
        hiddenMapImages: new Map(),
        excluded: new Set(this.images().map((i) => i.id)),
        emptyError: 'Escolha uma imagem para pôr na cena.',
        submit: async (image) => {
          await this.save(
            [...this.images().map((i) => i.id), image.id],
            `${image.name} entrou na cena.`,
          );
        },
        errorMessage: (err) => sceneImagesErrorMessage(err, 'pôr a imagem na cena'),
      },
      this.injector,
      this.phone(),
      this.host.nativeElement.querySelector<HTMLElement>('[data-control="pick"]') ?? true,
    );
  }

  /** The picture made in the generate dialog joins the list; a full scene keeps it only in the gallery. */
  protected async generated(outcome: GenerateOutcome): Promise<void> {
    const id = outcome.lastImageId;
    if (!id || this.images().some((i) => i.id === id)) {
      return;
    }
    if (this.full()) {
      this.error.set(
        `A imagem foi para a galeria, mas a cena já tem ${SCENE_IMAGE_LIMIT}. Tire uma para pôr esta.`,
      );
      return;
    }
    try {
      await this.save([...this.images().map((i) => i.id), id], 'A imagem gerada entrou na cena.');
    } catch (err) {
      this.error.set(sceneImagesErrorMessage(err, 'pôr a imagem gerada na cena'));
    }
  }

  protected async move(image: SceneImage, direction: 'up' | 'down'): Promise<void> {
    const ids = this.images().map((i) => i.id);
    const index = ids.indexOf(image.id);
    const to = direction === 'up' ? index - 1 : index + 1;
    if (this.busy() || to < 0 || to >= ids.length) {
      return;
    }
    [ids[index], ids[to]] = [ids[to], ids[index]];
    try {
      await this.save(
        ids,
        `Imagem ${index + 1} ${direction === 'up' ? 'subiu' : 'desceu'} para a posição ${to + 1}.`,
      );
      this.focusRow(image.id, direction);
    } catch (err) {
      this.error.set(sceneImagesErrorMessage(err, 'mudar a ordem das imagens'));
    }
  }

  protected async remove(image: SceneImage): Promise<void> {
    if (this.busy()) {
      return;
    }
    const ids = this.images().map((i) => i.id);
    const index = ids.indexOf(image.id);
    const rest = ids.filter((id) => id !== image.id);
    try {
      await this.save(rest, `${image.name} saiu da cena. Continua na galeria.`);
      const next = rest[index];
      if (next) {
        this.focusRow(next, 'remove');
      } else {
        this.focusPick();
      }
    } catch (err) {
      this.error.set(sceneImagesErrorMessage(err, 'tirar a imagem da cena'));
    }
  }

  /** One write at a time. Rejects (and leaves the list as it was) so each caller says what failed. */
  private async save(ids: readonly string[], announce: string): Promise<void> {
    this.busy.set(true);
    this.error.set('');
    try {
      const images = await this.api.setSceneImages(
        this.campaignId(),
        this.mapId(),
        this.pointId(),
        ids,
      );
      this.imagesChange.emit(images);
      this.status.set(announce);
    } finally {
      this.busy.set(false);
    }
  }

  private focusRow(imageId: string, control: Control): void {
    afterNextRender(
      () =>
        this.host.nativeElement
          .querySelector<HTMLElement>(`[data-image="${imageId}"][data-control="${control}"]`)
          ?.focus(),
      { injector: this.injector },
    );
  }

  private focusPick(): void {
    afterNextRender(
      () => this.host.nativeElement.querySelector<HTMLElement>('[data-control="pick"]')?.focus(),
      { injector: this.injector },
    );
  }
}
