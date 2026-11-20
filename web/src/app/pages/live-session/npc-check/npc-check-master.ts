import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  Injector,
  afterNextRender,
  effect,
  inject,
  input,
  signal,
  untracked,
} from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { NpcCheckClient } from '../../../core/npc-check/npc-check-client';
import { COMBAT_OPEN_TEXT } from '../../../core/npc-check/npc-check-errors';
import { NpcCheckLauncher } from '../../../core/npc-check/npc-check-launcher';
import { NpcCheckHidden, NpcCheckState } from '../../../core/npc-check/npc-check-state';
import { NpcCheckCard } from './npc-check-card';
import { openNpcCheckSheet } from './npc-check-sheet';

/**
 * The master's side of "Teste de um NPC" outside a combat: the sheet it opens when the header button, the stage menu or its own
 * button asks (`NpcCheckLauncher`), and the card of the check the server holds or settled last (read again on every hint that the
 * held rolls changed, and after a reload). With `entry` it draws the button too (the page with no scene open, beside "Pedir teste em
 * grupo"). A combat that is open makes the entry dashed, with the reason beside it.
 */
@Component({
  selector: 'app-npc-check-master',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule, NpcCheckCard],
  templateUrl: './npc-check-master.html',
  styleUrl: './npc-check-master.scss',
})
export class NpcCheckMaster {
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  protected readonly launcher = inject(NpcCheckLauncher);
  readonly campaignId = input.required<string>();
  /** Bumped by the page on each `held_rolls_changed`. */
  readonly tick = input(0);
  /** Moves when the map, the vision, the scene or the stage changed: read again only while the check waits. */
  readonly recheck = input(0);
  readonly entry = input(false);
  protected readonly state = new NpcCheckState(
    inject(NpcCheckClient),
    undefined,
    inject(NpcCheckHidden).key,
  );
  protected readonly combatText = COMBAT_OPEN_TEXT;
  private readonly opening = signal(false);
  private readonly document = inject(DOCUMENT);
  private readonly injector = inject(Injector);

  constructor() {
    effect(() => {
      const id = this.campaignId();
      this.tick();
      untracked(() => void this.state.load(id));
    });
    effect(() => {
      const id = this.campaignId();
      this.recheck();
      untracked(() => {
        if (this.state.card()?.settled === false) {
          void this.state.load(id);
        }
      });
    });
    effect(() => {
      const request = this.launcher.request();
      if (request) {
        untracked(() => {
          this.launcher.take();
          this.open(request.characterId);
        });
      }
    });
    // A refusal about a combat belongs to this board: a later board starts clean.
    inject(DestroyRef).onDestroy(() => this.launcher.combatOpen.set(false));
  }

  /** The "X" hides the card, which held the focus: it goes to the button that rolls the next check. */
  protected dismiss(): void {
    this.state.dismiss();
    afterNextRender(
      () => {
        const buttons = Array.from(this.document.querySelectorAll('button'));
        buttons.find((b) => (b.textContent ?? '').trim().endsWith('Teste de um NPC'))?.focus();
      },
      { injector: this.injector },
    );
  }

  protected open(firstId = ''): void {
    if (this.opening() || this.launcher.blocked()) {
      return;
    }
    this.opening.set(true);
    openNpcCheckSheet(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      firstId,
      state: this.state,
    }).subscribe({
      complete: () => this.opening.set(false),
      error: () => this.opening.set(false),
    });
  }
}
