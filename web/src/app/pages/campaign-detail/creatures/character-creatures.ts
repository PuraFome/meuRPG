import { Component, ElementRef, Injector, afterNextRender, effect, inject, input, output, signal, untracked } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import type { CharacterCreature } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { CreaturesClient } from '../../../core/creatures/creatures-client';
import { sourceShort } from '../../../core/creatures/creature-format';
import { CreatureArt } from '../../../shared/creatures/creature-art';
import { CreatureEdit } from '../../character-sheet/creatures-panel/creature-edit';
import { openSheet } from '../../live-session/combat/sheet-host';
import { GiveCreatureSheet, type GiveCreatureData, type GiveCreatureResult } from './give-creature-sheet';

/**
 * The master's "Criaturas" of one character in the campaign's list (E9-10,
 * quadros 4 and 6): a line for each creature (the small picture, the name, "Corvo
 * · familiar", and "Dispensar Nanquim", a text action that asks in place), or
 * "Nenhuma criatura", and "Dar uma criatura" (44 px, outlined, the character's
 * name in its `aria-label`), which opens the SRD search. A character can have
 * several creatures, so the button stays. On a phone the master reads the list
 * and gives none (the button is hidden there: the dialog is a computer's).
 *
 * It reads the character's list itself (the master reads any), and again when
 * the page says so (`reload`, the stream's `creatures_changed`), after a gift
 * and after a dismissal. Only the master gets this block.
 */
@Component({
  selector: 'app-character-creatures',
  imports: [CreatureArt, CreatureEdit, MatButtonModule, MatIconModule],
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

  protected readonly creatures = signal<readonly CharacterCreature[] | null>(null);
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
    try {
      this.creatures.set(await this.client.list(campaignId, characterId));
    } catch {
      // Keep what is shown; a read that fails shows no creatures rather than a wrong list.
      this.creatures.update((c) => c ?? []);
    }
  }

  protected give(): void {
    openSheet<GiveCreatureSheet, GiveCreatureData, GiveCreatureResult>(this.dialog, this.bottomSheet, GiveCreatureSheet, {
      data: { campaignId: this.campaignId(), characterId: this.characterId(), characterName: this.characterName() },
      ariaLabel: `Dar uma criatura a ${this.characterName()}`,
      labelledBy: 'give-t',
      width: '720px',
      focus: 'input[type=search]',
    }).subscribe((result) => {
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
      void this.load(this.campaignId(), this.characterId());
      return;
    }
    afterNextRender(() => this.host.nativeElement.querySelector<HTMLElement>(`#dismiss-${id}`)?.focus(), { injector: this.injector });
  }
}
