import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import type { GalleryImage } from '../../../gen/meurpg/maps/v1/gallery_pb';
import type { ImageEdit, ImageGenerationStatus } from '../../../gen/meurpg/maps/v1/imagegen_pb';
import { EDIT_PRIVACY, remainingText } from '../../core/images/imagegen-copy';

/** The image on screen with what the dialog knows about how it was made. */
export interface ResultView {
  readonly image: GalleryImage;
  /** "Imagem 1": the request's number, 0 without one. */
  readonly number: number;
  /** "Gerada a partir do mapa · Pintura a óleo · 2 NPCs", empty when the dialog opened on an older image. */
  readonly caption: string;
  /** The grid goes over it (it is a textured map, or an edit of one, of the map the dialog was opened from). */
  readonly texture: boolean;
  /** It shows the whole map, the rooms the players have not found included (`GalleryImage.shows_whole_map`, RN-10): never one tap from the players. */
  readonly wholeMap: boolean;
}

/** Where "Usar como imagem do mapa" is: not asked, asked in place, working, or done. */
export type UseStage = 'idle' | 'asking' | 'busy' | 'done';

/** Where "Mostrar aos jogadores" of a picture that shows the whole map is: the question asked in place first (E10-07, RN-10). */
export type ShowStage = 'idle' | 'asking';

/**
 * What the master gets back (E10-07 5 and 6): the picture, "Guardada na galeria" (it is stored before it is shown; there is no "Guardar"),
 * and the actions.
 * - The scene art and the isometric view are hidden from the players until "Mostrar aos jogadores" (the showing of E8-05, one touch, no
 *   question: an image made from a map shows only what the players see); then the tag says "Mostrada aos jogadores".
 * - The textured map is drawn with the map's grid over it, so the master checks that it did not slip off a wall, and "Usar como imagem do
 *   mapa" asks in place (the grid, the layers and what the players saw stay; the players see the new image).
 * - "Pedir um ajuste" asks for a new picture that starts from this one (it costs one slot) and joins the chain, listed when there is more
 *   than one image ("Imagem 1 › Imagem 2", the current one in bold). What goes to Google is said under the field.
 * Presentational: the dialog runs the calls.
 */
@Component({
  selector: 'app-generate-result',
  imports: [MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './generate-result.html',
  styleUrl: './generate-result.scss',
})
export class GenerateResult {
  readonly view = input.required<ResultView>();
  readonly chain = input<readonly ImageEdit[]>([]);
  readonly shown = input(false);
  readonly showBusy = input(false);
  readonly showError = input<string | null>(null);
  /** The map's grid, for the lines over a textured map. */
  readonly gridColumns = input(0);
  readonly gridRows = input(0);
  /** A textured map made from a map of the campaign can become its image. */
  readonly canUse = input(false);
  readonly useStage = input<UseStage>('idle');
  readonly useError = input<string | null>(null);
  readonly instruction = input('');
  readonly editError = input<string | null>(null);
  readonly status = input<ImageGenerationStatus | null>(null);
  readonly showStage = input<ShowStage>('idle');
  /** The campaign has an open session: only then can an image be shown (without one the button is dashed and says why). */
  readonly sessionOpen = input(true);

  readonly instructionChange = output<string>();
  readonly pickChain = output<string>();

  protected readonly privacy = EDIT_PRIVACY;
  protected readonly ratio = computed(() =>
    this.view().image.height > 0 ? this.view().image.width / this.view().image.height : 1.78,
  );
  protected readonly texture = computed(() => this.view().texture);
  protected readonly wholeMap = computed(() => this.view().wholeMap);
  protected readonly asking = computed(
    () =>
      this.useStage() === 'asking' || this.useStage() === 'busy' || this.showStage() === 'asking',
  );
  protected readonly alt = computed(
    () => `${this.numbered()}${this.view().caption ? `: ${this.view().caption}` : ''}`,
  );
  protected readonly cost = computed(() => {
    const s = this.status();
    return s
      ? `O ajuste parte desta imagem e do seu pedido de antes. Gera uma imagem nova ao lado desta e gasta 1 das suas ${s.remaining}.`
      : 'O ajuste parte desta imagem e do seu pedido de antes. Gera uma imagem nova ao lado desta e gasta 1 imagem do mês.';
  });
  protected readonly remaining = computed(() => {
    const s = this.status();
    return s ? remainingText(s) : '';
  });

  /** The lines of the grid over a textured map, as one path in squares (the stroke does not scale). */
  protected readonly gridPath = computed(() => {
    const c = this.gridColumns();
    const r = this.gridRows();
    if (!this.texture() || c <= 0 || r <= 0) {
      return '';
    }
    let d = '';
    for (let x = 0; x <= c; x++) {
      d += `M${x} 0V${r}`;
    }
    for (let y = 0; y <= r; y++) {
      d += `M0 ${y}H${c}`;
    }
    return d;
  });

  protected numbered(): string {
    const n = this.view().number;
    return n > 0 ? `Imagem ${n}` : this.view().image.name;
  }

  protected setInstruction(event: Event): void {
    this.instructionChange.emit((event.target as HTMLTextAreaElement).value);
  }
}
