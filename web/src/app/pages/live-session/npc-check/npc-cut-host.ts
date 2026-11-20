import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  input,
  signal,
  untracked,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { effectivePreference } from '../../../core/campaigns/dice-labels';
import { CuttingFlow } from '../../../core/npc-check/cutting-flow';
import { NpcCheckClient } from '../../../core/npc-check/npc-check-client';
import { ResourceClient } from '../../../core/resources/resources-client';
import { openNpcCut } from './npc-cut-sheet';

/** How long "Você deixou passar. Nada foi gasto." stays on the page after the sheet closed. */
export const PASS_NOTE_MS = 6000;
export const PASS_NOTE_TEXT = 'Você deixou passar. Nada foi gasto.';

/**
 * The bard's host of the question of an NPC check outside a combat: it reads the held rolls on every hint, opens the sheet when one
 * of them is a cutting-words question for this player (over the board or a puzzle), keeps it up to date (the master going on without
 * the answer shows in the open sheet) and, once the bard let it pass, leaves one polite line on the page for a moment.
 */
@Component({
  selector: 'app-npc-cut-host',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  templateUrl: './npc-cut-host.html',
  styleUrl: './npc-cut-host.scss',
})
export class NpcCutHost {
  private readonly resources = inject(ResourceClient);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  readonly campaignId = input.required<string>();
  /** Moves when the stream says the held rolls changed, and after a reconnection. */
  readonly tick = input(0);
  /** Moves when the map, the vision, the scene or the stage changed: read again only while a question is asked. */
  readonly recheck = input(0);
  readonly diceMode = input<DiceMode>(DiceMode.PLAYERS_CHOOSE);
  readonly preference = input<DicePreference>(DicePreference.APP);
  protected readonly flow = new CuttingFlow(inject(NpcCheckClient), () => this.campaignId());
  protected readonly note = signal('');
  private readonly typed = computed(
    () => effectivePreference(this.diceMode(), this.preference()) === DicePreference.PHYSICAL,
  );
  private reads = 0;
  private open = false;
  private noteTimer: ReturnType<typeof setTimeout> | undefined;

  constructor() {
    effect(() => {
      this.tick();
      this.campaignId();
      untracked(() => void this.load());
    });
    effect(() => {
      this.recheck();
      untracked(() => {
        if (this.flow.offer() !== null && this.flow.phase() === 'asking') {
          void this.load();
        }
      });
    });
    effect(() => {
      if (this.flow.offer() !== null) {
        untracked(() => this.show());
      }
    });
    inject(DestroyRef).onDestroy(() => {
      this.reads++;
      clearTimeout(this.noteTimer);
    });
  }

  private show(): void {
    if (this.open) {
      return;
    }
    this.open = true;
    openNpcCut(this.dialog, this.bottomSheet, { flow: this.flow, typed: this.typed() }).subscribe({
      next: (passed) => this.closed(passed === true),
      error: () => this.closed(false),
      complete: () => (this.open = false),
    });
  }

  private closed(passed: boolean): void {
    this.open = false;
    this.flow.acknowledge();
    // The question that waited behind the one that closed comes in now.
    void this.load();
    if (passed) {
      this.note.set(PASS_NOTE_TEXT);
      clearTimeout(this.noteTimer);
      this.noteTimer = setTimeout(() => this.note.set(''), PASS_NOTE_MS);
    }
  }

  private async load(): Promise<void> {
    const read = ++this.reads;
    try {
      const res = await this.resources.listHeldRolls(this.campaignId());
      if (read === this.reads) {
        this.flow.sync(res.offers);
      }
    } catch {
      // A hint that did not come back: the question stays as it is and the next hint reads again.
    }
  }
}
