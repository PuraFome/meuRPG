import {
  ChangeDetectionStrategy,
  Component,
  effect,
  inject,
  input,
  untracked,
} from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { FavorState } from '../../../core/play/favor-state';
import type { VitalsVm } from '../../../core/play/live-session.types';

/**
 * "Inspiração" on the character sheet (RN-43, board 581-04): drawn only while the character has the master's Inspiração, for the
 * master and for the owner (nobody else reads the sheet of a character, RN-10). The owner reads what it is for: "Do mestre.
 * Vantagem num d20 seu: marque "Usar a Inspiração" antes de rolar."; the master reads the title alone, since those words are
 * the player's. There is no "Passar a…" yet: the pass is not built.
 *
 * The page draws it only while the campaign has an open session, and gives it the live numbers (`vitals`): each `vitals_changed`
 * hint is a new copy, and it is the only hint a grant tells, so the panel reads who has it again when the copy changes. It keeps
 * its own `FavorState`; a read that fails draws nothing.
 */
@Component({
  selector: 'app-favor-panel',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [FavorState],
  template: `
    @if (favor.has(characterId())) {
      <section class="mr-panel favor" aria-labelledby="favor-heading">
        <h2 class="favor__title" id="favor-heading">
          <mat-icon aria-hidden="true">star</mat-icon>Inspiração
        </h2>
        @if (viewer() === 'owner') {
          <p class="favor__text">
            Do mestre. Vantagem num d20 seu: marque "Usar a Inspiração" antes de rolar.
          </p>
        }
      </section>
    }
  `,
  styles: `
    :host {
      display: block;

      &:empty {
        display: none;
      }
    }

    .favor__title {
      display: flex;
      align-items: center;
      gap: var(--mr-space-2);
      margin: 0;
      font-family: var(--mr-font-display);
      font-size: var(--mr-font-21);
      line-height: 26px;
      font-weight: 700;
      color: var(--mr-ink);
    }

    .favor__text {
      margin: var(--mr-space-2) 0 0;
      font-size: var(--mr-font-14);
      line-height: 19px;
      color: var(--mr-ink-muted);
    }
  `,
})
export class FavorPanel {
  readonly campaignId = input.required<string>();
  readonly characterId = input.required<string>();
  /** Who reads the sheet: the owner reads what to do with it. */
  readonly viewer = input<'owner' | 'master'>('owner');
  /** The character's live numbers: a new copy is a `vitals_changed` hint, and the cue to read again. */
  readonly vitals = input<VitalsVm | null>(null);

  protected readonly favor = inject(FavorState);

  constructor() {
    effect(() => {
      const id = this.campaignId();
      untracked(() => this.favor.follow(id));
    });
    effect(() => {
      this.vitals();
      untracked(() => void this.favor.refresh());
    });
  }
}
