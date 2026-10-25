import {
  ChangeDetectionStrategy,
  Component,
  computed,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';

import type { SceneImage } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import type { SceneAudienceView } from '../../../../../gen/meurpg/play/v1/scene_pb';
import { tight } from '../../../../core/format/text';
import { RetryImage } from '../../../../shared/retry-image/retry-image';
import { openSheet } from '../../../../shared/sheet/sheet-host';
import {
  type ImageAudienceVm,
  LiveSessionSource,
  ShownImageVm,
} from '../../../../core/play/live-session.types';
import { imageAudienceErrorMessage } from '../../../../core/play/scene-errors';
import {
  choosablePlayers,
  imageChooserLine,
  shownAudienceLine,
} from '../../../../core/play/scene-audience';
import { joinNames } from '../../../../core/play/stage-view';
import { WhoChooser, type WhoChoice, type WhoData } from '../who-chooser/who-chooser';
import { showErrorMessage } from '../../shown-image-panel/shown-image-panel';

/**
 * "Imagens da cena" in the master's open scene (MR-015, MR-019): the images the
 * master prepared on the scene point, each with "Mostrar aos jogadores", which is
 * the gallery's own action (`SetShownImage`): it replaces the image on display
 * (or leaves it with the players when "Deixar com os jogadores" is on) and
 * follows the same rules. The one on display is marked "À mostra agora" and
 * offers "Parar de mostrar". An image that shows the whole map asks first, in
 * place, as the gallery picker does. The list is the master's; the players only
 * ever receive the image shown.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-scene-images-show',
  imports: [MatButtonModule, MatIconModule, MatMenuModule, RetryImage],
  templateUrl: './scene-images.html',
  styleUrl: './scene-images.scss',
})
export class SceneImagesShow {
  private readonly source = inject(LiveSessionSource);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);

  readonly campaignId = input.required<string>();
  readonly images = input.required<readonly SceneImage[]>();
  /** The image on the players' screens now, if any. */
  readonly shown = input<ShownImageVm | null>(null);
  /** The master's "Deixar com os jogadores" switch: the image replaced stays with the players. */
  readonly keep = input(false);
  /** Who is in the scene, as the master reads it: with some players only, "Mostrar" asks for whom (MR-028). */
  readonly audience = input<SceneAudienceView | null>(null);
  /** The server accepted a change: the page shows it at once. */
  readonly changed = output<ShownImageVm | null>();
  /** An image was left with the players: the page reads that list again. */
  readonly leftChanged = output<void>();

  /** The scene is open to only some players: an image may be for them, for the table or for those chosen. */
  protected readonly partial = computed(() => {
    const audience = this.audience();
    return !!audience && !audience.everyone;
  });
  protected readonly insideNames = computed(() =>
    joinNames(this.audience()?.inside.map((p) => p.characterName) ?? []),
  );
  protected readonly tableNames = computed(() => {
    const audience = this.audience();
    return joinNames(
      [...(audience?.inside ?? []), ...(audience?.outside ?? [])].map((p) => p.characterName),
    );
  });
  /** "Pensantus e Brisa veem": who sees the image on display. */
  protected readonly seeing = computed(() => {
    const shown = this.shown();
    return shown ? shownAudienceLine(shown.audience, this.audience()) : '';
  });
  /** "Mostrar a toda a mesa" is for an image some of the table does not see yet. */
  protected readonly canWiden = computed(() => {
    const shown = this.shown();
    return !!shown && this.partial() && (shown.audience?.kind ?? 'whole') !== 'whole';
  });
  protected readonly count = computed(() => tight(`${this.images().length} de 8`));
  protected readonly busyId = signal<string | null>(null);
  protected readonly askingId = signal<string | null>(null);
  protected readonly error = signal('');
  protected readonly status = signal('');
  /** The audience asked for before the whole-map question, kept for "Mostrar mesmo assim". */
  private askingAudience: ImageAudienceVm | undefined;

  protected thumb(image: SceneImage): string {
    return `/images/${image.id}/thumb`;
  }

  protected isShown(image: SceneImage): boolean {
    return this.shown()?.id === image.id;
  }

  protected async show(
    image: SceneImage,
    confirmed = false,
    audience?: ImageAudienceVm,
  ): Promise<void> {
    if (this.busyId() !== null) {
      return;
    }
    if (image.showsWholeMap && !confirmed) {
      this.error.set('');
      this.askingAudience = audience;
      this.askingId.set(image.id);
      return;
    }
    const leaving = this.keep() ? this.shown() : null;
    this.busyId.set(image.id);
    this.error.set('');
    try {
      const chosen = audience ?? this.askingAudience;
      const result = await (chosen
        ? this.source.setShownImage(this.campaignId(), image.id, undefined, chosen)
        : this.source.setShownImage(this.campaignId(), image.id));
      this.askingId.set(null);
      this.askingAudience = undefined;
      this.changed.emit(result);
      if (leaving) {
        this.leftChanged.emit();
      }
      this.status.set(`${image.name} está na tela dos jogadores.`);
    } catch (err) {
      this.askingId.set(null);
      this.askingAudience = undefined;
      this.error.set(audience ? imageAudienceErrorMessage(err) : showErrorMessage(err));
    } finally {
      this.busyId.set(null);
    }
  }

  protected cancelAsk(): void {
    this.askingId.set(null);
    this.askingAudience = undefined;
  }

  /** "Escolher quem vê…" on a thumbnail: the chooser, then the image is shown to those chosen. */
  protected chooseFor(image: SceneImage): void {
    this.choose(`Quem vê “${image.name}”?`, [], (audience) => this.show(image, false, audience));
  }

  /** "Mudar quem vê" on the image on display. */
  protected changeWho(): void {
    const shown = this.shown();
    if (!shown) {
      return;
    }
    const audience = this.audience();
    const selected =
      shown.audience?.kind === 'chosen'
        ? shown.audience.userIds
        : shown.audience?.kind === 'scene'
          ? (audience?.inside.map((p) => p.userId) ?? [])
          : choosablePlayers(audience).map((p) => p.userId);
    this.choose(`Quem vê “${shown.name}”?`, selected, (next) => this.setAudience(next));
  }

  /** "Mostrar a toda a mesa": the image on display, for everyone. */
  protected widen(): Promise<void> {
    return this.setAudience({ kind: 'whole', userIds: [] });
  }

  private choose(
    title: string,
    selected: readonly string[],
    then: (audience: ImageAudienceVm) => Promise<void>,
  ): void {
    const data: WhoData = {
      title,
      subtitle: 'Você sempre vê. Quem não for escolhido não vê a imagem.',
      players: choosablePlayers(this.audience()),
      selected,
      line: imageChooserLine,
    };
    openSheet<WhoChooser, WhoData, WhoChoice | undefined>(
      this.dialog,
      this.bottomSheet,
      WhoChooser,
      { data, ariaLabel: title, labelledBy: 'who-title' },
    ).subscribe((choice) => {
      if (choice) {
        void then(
          choice.everyone
            ? { kind: 'whole', userIds: [] }
            : { kind: 'chosen', userIds: choice.userIds },
        );
      }
    });
  }

  private async setAudience(audience: ImageAudienceVm): Promise<void> {
    if (this.busyId() !== null) {
      return;
    }
    this.busyId.set('audience');
    this.error.set('');
    try {
      this.changed.emit(await this.source.setShownImageAudience(this.campaignId(), audience));
      this.status.set('Mudou quem vê a imagem.');
    } catch (err) {
      this.error.set(imageAudienceErrorMessage(err));
    } finally {
      this.busyId.set(null);
    }
  }
}
