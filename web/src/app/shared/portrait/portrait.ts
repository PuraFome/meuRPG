import { Component, computed, effect, input, signal, untracked } from '@angular/core';

import { initialsOf } from '../../core/play/stage-view';

/**
 * An NPC's portrait (MR-031, E8-08): the gallery image in a frame 5 wide by 6
 * tall, or, with no image, the NPC's initials in the same frame ("AL" for
 * Aldo, "CG" for Capitão Goblin). The initials also come when the image can't
 * load (a player's request for a portrait whose NPC left the stage is a 404):
 * the frame falls back to them and never shows a broken-image icon.
 *
 * ```html
 * <app-portrait [src]="npc.portraitUrl" [name]="npc.name" [size]="64" />
 * ```
 *
 * - `size` is the width in pixels (the height follows, 5:6), or any CSS
 *   length. Without it the frame fills the width it is given.
 * - `fit` is how the image sits in the frame: `contain` shows the whole
 *   picture (the editor and the enlarged view), `cover` fills the frame (the
 *   small ones in cards and lists).
 * - The frame is an image (`role="img"`, "Retrato de Mira") for a screen
 *   reader; a caller that names the NPC next to it passes `decorative`.
 *
 * The cut-out of the player's stage has no frame at all: it is drawn by
 * `StagePlayer`, which uses `initialsOf` and its own image.
 */
@Component({
  selector: 'app-portrait',
  template: `
    @if (src() && !failed()) {
      <img
        class="pt__img"
        [src]="src()"
        alt=""
        decoding="async"
        draggable="false"
        (error)="failed.set(true)"
      />
    } @else {
      <span class="pt__initials" aria-hidden="true">{{ initials() }}</span>
    }
  `,
  styleUrl: './portrait.scss',
  host: {
    '[attr.role]': 'decorative() ? null : "img"',
    '[attr.aria-label]': 'decorative() ? null : label()',
    '[attr.aria-hidden]': 'decorative() ? "true" : null',
    '[class.pt--cover]': 'fit() === "cover"',
    '[style.--pt-w]': 'width()',
  },
})
export class Portrait {
  /** The image URL ("/images/<id>" or its "/thumb"), or empty for none. */
  readonly src = input('');
  readonly name = input.required<string>();
  readonly size = input<number | string | null>(null);
  readonly fit = input<'contain' | 'cover'>('contain');
  readonly decorative = input(false);

  /** The image failed: the initials take its place until the URL changes. */
  protected readonly failed = signal(false);
  protected readonly initials = computed(() => initialsOf(this.name()));
  protected readonly label = computed(() => `Retrato de ${this.name()}`);
  protected readonly width = computed(() => {
    const size = this.size();
    return size === null ? null : typeof size === 'number' ? `${size}px` : size;
  });

  constructor() {
    effect(() => {
      this.src();
      untracked(() => this.failed.set(false));
    });
  }
}
