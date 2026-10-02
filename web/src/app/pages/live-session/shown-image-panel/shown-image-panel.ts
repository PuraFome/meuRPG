import {
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { Code, ConnectError } from '@connectrpc/connect';

import type { Map as MapMessage } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { describeConnectError } from '../../../core/connect/connect-errors';
import { GalleryClient } from '../../../core/images/gallery-client';
import { openImagePicker } from '../../../shared/gallery-picker/image-picker-dialog/image-picker-dialog';
import { PHONE_QUERY, mediaQuery } from '../../../shared/map-view/media-query';
import { LiveSessionSource, ShownImageVm } from '../live-session.types';

/** What the dialog's line says under the grid (E5-10). */
export function shownImageNote(
  image: { name: string } | null,
  current: { name: string } | null,
): string {
  return current && image
    ? `Os jogadores passam a ver ${image.name} no lugar de ${current.name}.`
    : 'Os jogadores veem a imagem na hora, com o nome dela como legenda. O mapa atual continua na tela deles.';
}

/** The images that are the background of a hidden map, with the map's
 * name: showing one reveals the image and its name, not the map. */
export function hiddenMapImages(maps: readonly MapMessage[]): Map<string, string> {
  const hidden = new Map<string, string>();
  for (const map of maps) {
    if (!map.revealed && map.image) {
      hidden.set(map.image.id, map.name);
    }
  }
  return hidden;
}

/**
 * The master's "Imagem para os jogadores" panel on the session page (E5-10,
 * E5-11, MR-028): show a gallery image as a handout, apart from the current
 * map.
 *
 * - Empty: "Nenhuma imagem à mostra." and an outlined "Mostrar imagem" that
 *   opens the picker. With an empty gallery the text says to upload one.
 * - Showing: an accent frame, the thumbnail, "Mostrando agora", the image's
 *   name and two outlined buttons: "Parar de mostrar" (at once, no confirm:
 *   showing it again undoes it) and "Trocar imagem".
 * - No filled button on the page: the filled "Mostrar aos jogadores" is the
 *   dialog's.
 * - Each change is announced (`role="status"`) and focus follows: to
 *   "Mostrar imagem" after stopping, to "Parar de mostrar" after showing
 *   (the button that opened the dialog no longer exists).
 */
@Component({
  selector: 'app-shown-image-panel',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './shown-image-panel.html',
  styleUrl: './shown-image-panel.scss',
})
export class ShownImagePanel {
  private readonly source = inject(LiveSessionSource);
  private readonly gallery = inject(GalleryClient);
  private readonly dialog = inject(MatDialog);
  private readonly injector = inject(Injector);

  readonly campaignId = input.required<string>();
  readonly shown = input<ShownImageVm | null>(null);
  /** The campaign's maps, to tag the images that back a hidden map. */
  readonly maps = input<readonly MapMessage[]>([]);
  /** The server accepted a change: the page shows it at once. */
  readonly changed = output<ShownImageVm | null>();

  protected readonly phone = mediaQuery(PHONE_QUERY);
  protected readonly status = signal('');
  protected readonly error = signal<string | null>(null);
  protected readonly stopping = signal(false);
  protected readonly galleryEmpty = signal(false);
  protected readonly thumb = computed(() => {
    const s = this.shown();
    return s ? `${s.url}/thumb` : '';
  });

  private readonly showButton = viewChild('showButton', { read: ElementRef<HTMLButtonElement> });
  private readonly stopButton = viewChild('stopButton', { read: ElementRef<HTMLButtonElement> });

  constructor() {
    afterNextRender(() => {
      this.gallery.list(this.campaignId()).then(
        ({ images }) => this.galleryEmpty.set(images.length === 0),
        () => undefined,
      );
    });
  }

  protected open(): void {
    const shown = this.shown();
    const ref = openImagePicker(
      this.dialog,
      {
        campaignId: this.campaignId(),
        title: 'Mostrar uma imagem aos jogadores',
        confirmLabel: 'Mostrar aos jogadores',
        confirmIcon: 'cast',
        current: shown ? { id: shown.id, name: shown.name } : null,
        currentTag: 'À mostra agora',
        currentNote: 'Essa imagem já está à mostra.',
        note: shownImageNote,
        hiddenMapImages: hiddenMapImages(this.maps()),
        emptyError: 'Escolha uma imagem para mostrar.',
        submit: async (image) => {
          const result = await this.source.setShownImage(this.campaignId(), image.id);
          this.changed.emit(result);
          this.status.set(`${image.name} está na tela dos jogadores.`);
        },
        errorMessage: showErrorMessage,
      },
      this.injector,
      this.phone(),
      // The button that opened the dialog may be gone when it closes.
      false,
    );
    ref.afterClosed().subscribe((done) => {
      afterNextRender(
        () => {
          if (done) {
            this.stopButton()?.nativeElement.focus();
          } else {
            (this.showButton() ?? this.stopButton())?.nativeElement.focus();
          }
        },
        { injector: this.injector },
      );
    });
  }

  protected async stop(): Promise<void> {
    this.stopping.set(true);
    this.error.set(null);
    try {
      await this.source.setShownImage(this.campaignId(), null);
      this.changed.emit(null);
      this.status.set('Imagem retirada da tela dos jogadores.');
      afterNextRender(() => this.showButton()?.nativeElement.focus(), { injector: this.injector });
    } catch (err) {
      this.error.set(showErrorMessage(err));
    } finally {
      this.stopping.set(false);
    }
  }
}

/** SetShownImage's errors, in Portuguese (play.proto). */
export function showErrorMessage(err: unknown): string {
  if (ConnectError.from(err).code === Code.FailedPrecondition) {
    return 'A sessão acabou: as imagens só são mostradas durante a sessão.';
  }
  return describeConnectError(err, {
    [Code.NotFound]: 'Essa imagem não está mais na galeria. Escolha outra.',
    [Code.PermissionDenied]: 'Só o mestre da campanha mostra imagens.',
  });
}
