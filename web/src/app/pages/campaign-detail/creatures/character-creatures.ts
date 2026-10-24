import {
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import type { CharacterCreature } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { CreaturesClient } from '../../../core/creatures/creatures-client';
import { focusWithRing } from '../../../core/creatures/focus-ring';
import { sourceShort } from '../../../core/creatures/creature-format';
import { CreatureArt } from '../../../shared/creatures/creature-art';
import { CreatureEdit } from '../../../shared/creatures/creature-edit';
import { openSheet } from '../../../shared/sheet/sheet-host';
import {
  GiveCreatureSheet,
  type GiveCreatureData,
  type GiveCreatureResult,
} from './give-creature-sheet';

/**
 * The master's creatures of one character in the campaign's list (E9-10, quadros 4 and 6; board
 * C2-01): a line for each creature (the small picture, the name, "Corvo · familiar", and "Dispensar
 * Nanquim", a text action that asks in place), and nothing at all while the character has none.
 * "Dar uma criatura" is not here: it is an item of the character's "⋮" menu on the list, which calls
 * `give()` (it opens the SRD search; a character can have several creatures). On a phone the master
 * reads the list and gives none (the menu has no such item there: the dialog is a computer's).
 *
 * It reads the character's list itself (the master reads any), and again when the page says so
 * (`reload`, the stream's `creatures_changed`), after a gift and after a dismissal. Only the master
 * gets this block.
 */
@Component({
  selector: 'app-character-creatures',
  imports: [CreatureArt, CreatureEdit, MatIconModule],
  templateUrl: './character-creatures.html',
  styleUrl: './character-creatures.scss',
})
export class CharacterCreatures {
  private readonly client = inject(CreaturesClient);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly campaignId = input.required<string>();
  readonly characterId = input.required<string>();
  readonly characterName = input.required<string>();
  readonly reload = input(0);
  /** A creature was given: the list says it in its live region. */
  readonly given = output<string>();
  /** The sheet closed or a line was dismissed: the page puts the focus back on the character's "⋮" button. */
  readonly settled = output();

  protected readonly creatures = signal<readonly CharacterCreature[] | null>(null);
  /** The read failed and nothing was ever read: say so instead of "Nenhuma criatura". */
  protected readonly failed = signal(false);
  private seq = 0;
  /** The creature whose "Dispensar" question is open. */
  protected readonly asking = signal<string | null>(null);
  protected readonly sourceShort = sourceShort;

  constructor() {
    effect(() => {
      this.reload();
      const campaignId = this.campaignId();
      const characterId = this.characterId();
      untracked(() => void this.load(campaignId, characterId));
    });
  }

  private async load(campaignId: string, characterId: string): Promise<void> {
    const seq = ++this.seq;
    try {
      const list = await this.client.list(campaignId, characterId);
      // A newer read started meanwhile: its answer is the one that counts.
      if (seq === this.seq) {
        this.creatures.set(list);
        this.failed.set(false);
      }
    } catch {
      // Keep what is shown; with nothing shown yet, say the read failed.
      if (seq === this.seq && this.creatures() === null) {
        this.failed.set(true);
      }
    }
  }

  give(): void {
    openSheet<GiveCreatureSheet, GiveCreatureData, GiveCreatureResult>(
      this.dialog,
      this.bottomSheet,
      GiveCreatureSheet,
      {
        data: {
          campaignId: this.campaignId(),
          characterId: this.characterId(),
          characterName: this.characterName(),
        },
        ariaLabel: `Dar uma criatura a ${this.characterName()}`,
        labelledBy: 'give-t',
        width: '720px',
        focus: 'input[type=search]',
      },
    ).subscribe((result) => {
      // The dialog gives the focus back to the button that opened it; the page draws the ring.
      this.settled.emit();
      if (result) {
        void this.load(this.campaignId(), this.characterId());
        this.given.emit(result.name);
      }
    });
  }

  protected ask(id: string): void {
    this.asking.set(id);
  }

  protected closed(changed: boolean, id: string): void {
    this.asking.set(null);
    if (changed) {
      // The dismissed line is gone: the focus goes to the character's "⋮" button, which is always there.
      void this.load(this.campaignId(), this.characterId()).then(() => this.settled.emit());
      return;
    }
    afterNextRender(
      () => focusWithRing(this.host.nativeElement.querySelector<HTMLElement>(`#dismiss-${id}`)),
      { injector: this.injector },
    );
  }
}
