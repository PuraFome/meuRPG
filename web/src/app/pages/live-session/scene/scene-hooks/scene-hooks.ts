import { Component, computed, input, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { MarkdownView } from '../../../../shared/markdown/markdown-view';

let nextId = 0;

/**
 * "Ganchos e anotações" in the open scene (E8-05, MR-029): the master's
 * private text on the scene, read here during the session (editing is in the
 * map editor). It is open on a computer and folded on a phone, where the
 * screen is for the clues first; folded or not, the lock and "Só você vê" are
 * always on screen, so nobody turns the screen towards a player not knowing
 * there is a secret on it. The button that folds it is 48px with
 * `aria-expanded` and the section's name; the state is only this page's
 * (nothing is saved).
 */
@Component({
  selector: 'app-scene-hooks',
  imports: [MarkdownView, MatIconModule],
  templateUrl: './scene-hooks.html',
  styleUrl: './scene-hooks.scss',
})
export class SceneHooks {
  /** The hooks' Markdown. */
  readonly hooks = input('');
  /** A phone folds them at the start. */
  readonly phone = input(false);

  protected readonly id = `sh-${nextId++}`;
  private readonly choice = signal<boolean | null>(null);
  protected readonly open = computed(() => this.choice() ?? !this.phone());
  protected readonly empty = computed(() => this.hooks().trim() === '');

  protected toggle(): void {
    this.choice.set(!this.open());
  }
}
