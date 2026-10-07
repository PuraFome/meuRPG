import { Component, computed, effect, input, signal, untracked } from '@angular/core';

import { initialsOf } from '../../../../core/play/stage-view';

/**
 * One portrait of the players' stage, a cut-out (MR-031, E8-10): the image
 * stands directly on the stage, with **no box, no border, no radius and no
 * clipping behind it**, because the portraits are PNGs with a transparent
 * background (Q62). A line under the figure is its base: 1px for everyone,
 * 4px in the accent for the one who speaks, so the speaker never depends on a
 * silhouette that may match its stage.
 *
 * The one thing that keeps a tile is the initials placeholder: it is not an
 * image. It also takes the place of an image that fails to load (a portrait
 * whose NPC left the stage is a 404 for a player), so there is never a broken
 * icon. The picture sits at the bottom of a 5 by 6 box, so figures of any
 * proportion share one base line.
 *
 * The same piece draws the enlarged view, which gives it its own size.
 */
@Component({
  selector: 'app-stage-figure',
  template: `
    <span class="fig" [class.fig--speaking]="speaking()">
      @if (src() && !failed()) {
        <img class="fig__img" [src]="src()" alt="" decoding="async" draggable="false" (error)="failed.set(true)" />
      } @else {
        <span class="fig__tile" aria-hidden="true">{{ initials() }}</span>
      }
    </span>
  `,
  styleUrl: './stage-figure.scss',
  host: {
    role: 'img',
    '[attr.aria-label]': 'label()',
  },
})
export class StageFigure {
  readonly src = input('');
  readonly name = input.required<string>();
  readonly speaking = input(false);

  protected readonly failed = signal(false);
  protected readonly initials = computed(() => initialsOf(this.name()));
  protected readonly label = computed(() =>
    this.src() && !this.failed() ? `Retrato de ${this.name()}` : `Sem retrato: ${this.name()}`,
  );

  constructor() {
    effect(() => {
      this.src();
      untracked(() => this.failed.set(false));
    });
  }
}
