import { Component, computed, input, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { Fold } from '../../../../shared/fold/fold';
import { MarkdownView } from '../../../../shared/markdown/markdown-view';

/**
 * "Ganchos e anotações" in the open scene (E8-05, MR-029): the master's
 * private text on the scene, read here during the session (editing is in the
 * map editor). Folded at the start on every screen (C3-01), where the
 * screen is for the clues first; folded or not, the lock and "Só você vê" are
 * always on screen, so nobody turns the screen towards a player not knowing
 * there is a secret on it. It is an `app-fold`: the state is only this page's
 * (nothing is saved).
 */
@Component({
  selector: 'app-scene-hooks',
  imports: [Fold, MarkdownView, MatIconModule],
  templateUrl: './scene-hooks.html',
  styleUrl: './scene-hooks.scss',
})
export class SceneHooks {
  /** The hooks' Markdown. */
  readonly hooks = input('');
  /** A phone folds them at the start. */
  readonly phone = input(false);

  protected readonly choice = signal<boolean | null>(null);
  protected readonly open = computed(() => this.choice() ?? false);
  protected readonly empty = computed(() => this.hooks().trim() === '');
}
