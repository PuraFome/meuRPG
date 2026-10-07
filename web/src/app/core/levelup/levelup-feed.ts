import { Injectable, computed, inject, signal } from '@angular/core';

import type { LevelUp } from '../../../gen/meurpg/characters/v1/characters_pb';
import { LevelUpClient } from './levelup-client';

/** A level-up is "fresh" for the master for a day after the player confirmed it. */
export const FRESH_MS = 24 * 60 * 60 * 1000;

const PAGE_SIZE = 50;
/** A safety net: 500 level-ups are more than a table ever makes. */
const MAX_PAGES = 10;

/**
 * The master's view of the campaign's level-ups (MR-040): "Subiu para o nível N", the status line at
 * the top of the campaign page while one is fresh, and "O que mudou" in the characters' list. One
 * instance per campaign page, read through `ListLevelUps` (newest first; the master is told by the
 * session's `xp_changed`, so the page reads again then). There is no approval and no veto.
 *
 * How "fresh" ends without Web Storage: a level-up is fresh for 24 hours after it was confirmed.
 * Dismissing the status line hides it until the page is opened again (memory only).
 */
@Injectable()
export class LevelUpFeed {
  private readonly client = inject(LevelUpClient);

  private campaignId = '';
  private seq = 0;

  readonly items = signal<readonly LevelUp[]>([]);
  readonly loaded = signal(false);
  readonly dismissed = signal(false);
  /** The clock of the last read: "fresh" is judged against it, so nothing ticks. */
  private readonly now = signal(Date.now());

  /** Level-ups of the last 24 hours, newest first. */
  readonly fresh = computed(() => {
    const now = this.now();
    return this.items().filter(
      (l) => l.createdAt && now - Number(l.createdAt.seconds) * 1000 < FRESH_MS,
    );
  });

  /** Reads the campaign's level-ups, once per campaign; `refresh` reads again. */
  async load(campaignId: string): Promise<void> {
    if (campaignId === this.campaignId) {
      return;
    }
    this.campaignId = campaignId;
    this.loaded.set(false);
    this.items.set([]);
    await this.refresh();
  }

  async refresh(): Promise<void> {
    if (!this.campaignId) {
      return;
    }
    const seq = ++this.seq;
    try {
      // Every page, newest first: a campaign with more than one page of level-ups never loses the newest.
      const all: LevelUp[] = [];
      let token = '';
      for (let page = 0; page < MAX_PAGES; page++) {
        const res = await this.client.list(this.campaignId, PAGE_SIZE, token);
        all.push(...res.levelUps);
        token = res.nextPageToken;
        if (token === '') {
          break;
        }
      }
      if (seq === this.seq) {
        const before = this.items()[0]?.id;
        this.items.set(all);
        this.now.set(Date.now());
        // A newer level-up than the one the master had dismissed the line for: the line comes back.
        if (all[0] && all[0].id !== before) {
          this.dismissed.set(false);
        }
      }
    } catch {
      // The characters' list still works without it: nothing to show, nothing to break.
    } finally {
      if (seq === this.seq) {
        this.loaded.set(true);
      }
    }
  }

  /** Asks the characters' list to open the "O que mudou" of one character and bring it into view. */
  readonly reveal = signal<{ readonly characterId: string; readonly n: number } | null>(null);
  revealCharacter(characterId: string): void {
    this.reveal.update((r) => ({ characterId, n: (r?.n ?? 0) + 1 }));
  }

  /** The newest level-up of one character, if it has any. */
  latestOf(characterId: string): LevelUp | undefined {
    return this.items().find((l) => l.characterId === characterId);
  }

  /** Whether that newest level-up is still fresh. */
  isFresh(levelUp: LevelUp): boolean {
    return this.fresh().includes(levelUp);
  }

  dismiss(): void {
    this.dismissed.set(true);
  }
}
