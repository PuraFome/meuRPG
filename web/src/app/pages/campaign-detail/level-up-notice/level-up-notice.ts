import { Component, OnInit, computed, inject, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { LevelUpFeed } from '../../../core/levelup/levelup-feed';

/**
 * The master's status line at the top of the campaign page (MR-040, E8-15): "Pensantus subiu
 * para o nível 4. Veja o que foi escolhido em “O que mudou”." It shows while a level-up is
 * fresh (the last 24 hours, see `LevelUpFeed`) and until the master dismisses it, and it is a
 * polite live region, so a screen reader hears it when the page reads again after the stream's
 * `xp_changed`. There is nothing to approve: the line only tells.
 */
@Component({
  selector: 'app-level-up-notice',
  imports: [MatIconModule],
  template: `
    <div role="status" class="notice">
      @if (line(); as text) {
        <div class="mr-notice mr-notice--success">
          <mat-icon aria-hidden="true">arrow_upward</mat-icon>
          <p>
            <strong>{{ text }}</strong>
            <button type="button" class="notice__link" (click)="reveal()">Ver o que mudou</button>
          </p>
          <button type="button" class="notice__close" aria-label="Dispensar o aviso" (click)="feed.dismiss()">
            <mat-icon aria-hidden="true">close</mat-icon>
          </button>
        </div>
      }
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .notice:empty {
      display: none;
    }

    .mr-notice {
      align-items: flex-start;
      margin-bottom: var(--mr-space-4);
    }

    // The sentence, then the link under it on its own line, its words on the sentence's edge.
    p {
      flex: 1 1 auto;
      display: flex;
      flex-direction: column;
      align-items: flex-start;
    }

    // A text button: its words line up with the sentence, in a 44px target.
    .notice__link {
      display: inline-flex;
      align-items: center;
      min-height: 44px;
      margin: 0 0 -8px -8px;
      padding: 0 8px;
      border: 0;
      background: none;
      color: inherit;
      font: inherit;
      font-weight: 700;
      text-decoration: underline;
      text-underline-offset: 3px;
      cursor: pointer;
    }

    .notice__close {
      flex: none;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 44px;
      height: 44px;
      margin: -10px -10px -8px 0;
      padding: 0;
      border: 0;
      border-radius: 50%;
      background: none;
      color: inherit;
      cursor: pointer;
    }
  `,
})
export class LevelUpNotice implements OnInit {
  protected readonly feed = inject(LevelUpFeed);

  readonly campaignId = input.required<string>();

  /** "Pensantus subiu para o nível 4.", or "2 personagens subiram de nível.". */
  protected readonly line = computed(() => {
    const fresh = this.feed.fresh();
    if (fresh.length === 0 || this.feed.dismissed()) {
      return '';
    }
    return fresh.length === 1
      ? `${fresh[0].characterName} subiu para o nível ${fresh[0].toLevel}.`
      : `${fresh.length} personagens subiram de nível.`;
  });

  /** Scrolls to the newest fresh character's row and opens its "O que mudou". */
  protected reveal(): void {
    const newest = this.feed.fresh()[0];
    if (newest) {
      this.feed.revealCharacter(newest.characterId);
    }
  }

  ngOnInit(): void {
    void this.feed.load(this.campaignId());
  }
}
