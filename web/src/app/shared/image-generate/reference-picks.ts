import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { GalleryImage } from '../../../gen/meurpg/maps/v1/gallery_pb';
import { REFERENCES_PRIVACY } from '../../core/images/imagegen-copy';

/**
 * "Referências da galeria" (E10-07 1): the gallery images that go to Google as references, as small tiles with a ✕ to take one off, and
 * "Escolher da galeria", which opens the gallery's picker. The count is always in view, against the limits the server states (10 of objects
 * and places, 4 of characters); the characters are the portraits of the marked NPCs, named in the line. What goes to Google is said under
 * it. At the limit, "Escolher da galeria" is dashed and says so.
 */
@Component({
  selector: 'app-reference-picks',
  imports: [MatButtonModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <h3 class="title" id="refs-title" tabindex="-1" data-field="refs">Referências da galeria</h3>
    <div class="row">
      @if (images().length > 0) {
        <ul class="tiles" aria-labelledby="refs-title">
          @for (image of images(); track image.id) {
            <li class="tile">
              <span class="tile__thumb">
                <img [src]="image.thumbnailUrl" alt="" />
                <button type="button" class="tile__remove" [attr.aria-label]="'Tirar a referência ' + image.name" (click)="removed.emit(image.id)">
                  <mat-icon aria-hidden="true">close</mat-icon>
                </button>
              </span>
              <span class="tile__name">{{ image.name }}</span>
            </li>
          }
        </ul>
      }
      <button
        matButton="outlined"
        type="button"
        class="choose"
        [class.mr-button--off]="atLimit()"
        [disabled]="atLimit()"
        disabledInteractive
        [attr.aria-describedby]="atLimit() ? 'refs-limit' : null"
        (click)="add.emit()"
      >
        <mat-icon aria-hidden="true">image</mat-icon>Escolher da galeria
      </button>
    </div>
    @if (atLimit()) {
      <p class="note" id="refs-limit">Você já escolheu as {{ maxObjects() }} imagens que cabem. Tire uma para escolher outra.</p>
    }
    <p class="count">
      Objetos: <b>{{ images().length }}</b> de {{ maxObjects() }} · Personagens: <b>{{ characterNames().length }}</b> de {{ maxCharacters() }}{{ characterTail() }}
    </p>
    <p class="note">{{ privacy }}</p>
  `,
  styleUrl: './reference-picks.scss',
})
export class ReferencePicks {
  readonly images = input.required<readonly GalleryImage[]>();
  readonly maxObjects = input(10);
  readonly maxCharacters = input(4);
  /** The marked NPCs whose portraits go as character references. */
  readonly characterNames = input<readonly string[]>([]);
  readonly add = output<void>();
  readonly removed = output<string>();

  protected readonly privacy = REFERENCES_PRIVACY;
  protected readonly atLimit = computed(() => this.images().length >= this.maxObjects());
  protected readonly characterTail = computed(() => {
    const names = this.characterNames();
    return names.length === 0 ? '.' : ` (${names.join(', ')}, pelo marcado acima).`;
  });
}
