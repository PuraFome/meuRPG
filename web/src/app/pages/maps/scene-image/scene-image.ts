import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import type { GenerateOrigin } from '../../../core/images/imagegen-copy';
import { GenerateImageButton } from '../../../shared/image-generate/generate-image-button';

/**
 * "Imagem da cena" in the panel of a scene point (MR-039, E10-07 2): a picture of the place made by the image service, from the scene's name
 * and what the master writes, to show the players. It opens the generate dialog without a map, so only "Arte da cena" is on; the picture goes
 * to the gallery, hidden, and the dialog's result offers "Mostrar aos jogadores".
 */
@Component({
  selector: 'app-scene-image',
  imports: [GenerateImageButton],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="si" aria-labelledby="si-title">
      <h3 class="si__title" id="si-title">Imagem da cena</h3>
      <p class="si__text">Uma arte do lugar para mostrar aos jogadores. O texto começa com o nome da cena.</p>
      <app-generate-image-button [campaignId]="campaignId()" [origin]="origin()" />
    </section>
  `,
  styles: `
    .si {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: var(--mr-space-2);
    }

    .si__title {
      margin: 0;
      font-family: var(--mr-font-display);
      font-size: 21px;
      font-weight: 700;
      line-height: 26px;
    }

    .si__text {
      margin: 0;
      color: var(--mr-ink-muted);
      font-size: 14px;
      line-height: 19px;
    }
  `,
})
export class SceneImage {
  readonly campaignId = input.required<string>();
  readonly name = input.required<string>();
  /** The scene's point is revealed to the players: only then its name names the picture (RN-10). */
  readonly revealed = input(false);

  protected readonly origin = computed<GenerateOrigin>(() => ({ kind: 'scene', name: this.name(), revealed: this.revealed() }));
}
