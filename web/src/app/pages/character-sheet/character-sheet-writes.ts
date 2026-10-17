import { signal } from '@angular/core';

import {
  describeCharacterError,
  livingRefusal,
  type LivingRefusal,
} from '../../core/characters/character-errors';
import { ActionKey } from '../../core/connect/idempotency';
import { IDLE_SAVING, type SavingState } from '../../core/connect/saving-state';
import { type CharacterSheetSource, type CharacterSheetVm } from './character-sheet.types';

/** Who the page calls the player: the display name, or "o jogador". */
export function playerNameOf(vm: CharacterSheetVm): string {
  return vm.playerDisplayName ?? 'o jogador';
}

/** What the writes need of the page that owns them. */
export interface SheetWritesHost {
  readonly source: CharacterSheetSource;
  readonly navigate: (commands: string[]) => Promise<boolean>;
  /** The page is still on this character (and not gone). */
  readonly stillOn: (characterId: string) => boolean;
  /** Shows the answer of a write as the newest word on its character; false when the page left it. */
  readonly apply: (characterId: string, vm: CharacterSheetVm) => boolean;
  /** Reads the character again without the loading state. */
  readonly reload: () => void;
  /** A confirmation replaces the button that asked for it: the focus goes to its replacement. */
  readonly focus: (selector: string) => void;
}

/**
 * What the master and the owner do to a character from its page: mark it dead, bring it back, approve it, reject
 * it, ask for changes, send it again, and let the player edit the story. A plain class the page builds: each write
 * has its own saving state, its own confirmation question and (where a retry must not repeat it) its own key, and
 * the page keeps the sheet itself.
 */
export class SheetWrites {
  readonly markDeadState = signal<SavingState>(IDLE_SAVING);
  /** A death can't be undone, so it takes a second click ("Confirmar morte") after "Marcar como morto". */
  readonly confirmingDeath = signal(false);
  readonly storyToggleState = signal<SavingState>(IDLE_SAVING);
  /** "Reviver" (master, a dead player character): the question is open in place of the button. */
  readonly confirmingRevive = signal(false);
  readonly reviveState = signal<SavingState>(IDLE_SAVING);
  /** The player's other living character that refused the revival: the card says so, with the way to it. */
  readonly reviveRefused = signal<LivingRefusal | null>(null);
  /** The words a screen reader gets once the character lives again. */
  readonly reviveAnnouncement = signal('');
  /** "Aprovar personagem" / "Recusar personagem" (MR-024). */
  readonly approvalState = signal<SavingState>(IDLE_SAVING);
  /** Rejecting deletes the character for good, so it takes a second click ("Confirmar recusa"). */
  readonly confirmingReject = signal(false);
  /** "Pedir ajustes" (master): the form is open in place of the approval notice. */
  readonly requestingChanges = signal(false);
  readonly requestChangesState = signal<SavingState>(IDLE_SAVING);
  /** "Enviar de novo" (the owning player). */
  readonly resubmitState = signal<SavingState>(IDLE_SAVING);
  /** The player just sent the sheet again: the waiting notice says so, until the page is left. */
  readonly resubmitted = signal(false);
  /** The words a screen reader gets once the master's request went out. */
  readonly announcement = signal('');
  private readonly reviveKey = new ActionKey();
  private readonly requestKey = new ActionKey();
  private readonly resubmitKey = new ActionKey();

  constructor(private readonly host: SheetWritesHost) {}

  /** A new character on the page: nothing of the previous one stays open. */
  reset(): void {
    this.confirmingDeath.set(false);
    this.confirmingReject.set(false);
    this.requestingChanges.set(false);
    this.requestChangesState.set(IDLE_SAVING);
    this.resubmitState.set(IDLE_SAVING);
    this.resubmitted.set(false);
    this.announcement.set('');
    this.markDeadState.set(IDLE_SAVING);
    this.confirmingRevive.set(false);
    this.reviveState.set(IDLE_SAVING);
    this.reviveRefused.set(null);
    this.reviveAnnouncement.set('');
    this.storyToggleState.set(IDLE_SAVING);
    this.approvalState.set(IDLE_SAVING);
  }

  readonly askToConfirmDeath = (): void => {
    this.confirmingDeath.set(true);
    this.host.focus('.js-confirm-death');
  };

  readonly cancelDeath = (): void => {
    this.confirmingDeath.set(false);
    this.host.focus('.js-mark-dead');
  };

  readonly askToRevive = (): void => {
    this.reviveState.set(IDLE_SAVING);
    this.reviveRefused.set(null);
    this.confirmingRevive.set(true);
  };

  readonly cancelRevive = (): void => {
    this.confirmingRevive.set(false);
    this.reviveRefused.set(null);
    this.host.focus('.js-revive');
  };

  readonly askToConfirmReject = (): void => {
    this.confirmingReject.set(true);
    this.host.focus('.js-cancel-reject');
  };

  readonly cancelReject = (): void => {
    this.confirmingReject.set(false);
    this.host.focus('.js-reject');
  };

  readonly askForChanges = (): void => {
    this.confirmingReject.set(false);
    this.announcement.set('');
    this.requestChangesState.set(IDLE_SAVING);
    this.requestingChanges.set(true);
  };

  readonly cancelRequestChanges = (): void => {
    this.requestingChanges.set(false);
    this.host.focus('.js-ask-changes');
  };

  /** Master only: the character lives again; the page shows it as it was before the death. */
  readonly revive = async (
    campaignId: string,
    characterId: string,
    name: string,
  ): Promise<void> => {
    this.reviveState.set({ status: 'saving' });
    try {
      const key = this.reviveKey.keyFor({ campaignId, characterId });
      const vm = await this.host.source.reviveCharacter(campaignId, characterId, key);
      if (!this.host.apply(characterId, vm)) {
        return;
      }
      this.reviveKey.renew();
      this.confirmingRevive.set(false);
      this.reviveState.set(IDLE_SAVING);
      this.reviveAnnouncement.set(`${name} voltou à vida`);
    } catch (err) {
      this.confirmingRevive.set(false);
      const living = livingRefusal(err);
      if (living) {
        this.reviveRefused.set(living);
        this.host.focus('.js-open-living');
        return;
      }
      this.reviveState.set({ status: 'error', message: describeCharacterError(err) });
      // Someone else may have changed the character: read it again.
      this.host.reload();
    }
  };

  /** Master only (RN-15): the character goes back to its player with the reason and stays pending. */
  readonly requestChanges = async (
    campaignId: string,
    characterId: string,
    reason: string,
  ): Promise<void> => {
    this.requestChangesState.set({ status: 'saving' });
    try {
      const key = this.requestKey.keyFor({ campaignId, characterId, reason });
      const vm = await this.host.source.requestCharacterChanges(
        campaignId,
        characterId,
        reason,
        key,
      );
      if (!this.host.apply(characterId, vm)) {
        return;
      }
      this.requestKey.renew();
      this.requestChangesState.set(IDLE_SAVING);
      this.requestingChanges.set(false);
      this.announcement.set(`Pedido de ajustes enviado a ${playerNameOf(vm)}`);
      this.host.focus('.js-ask-changes');
    } catch (err) {
      this.requestChangesState.set({ status: 'error', message: describeCharacterError(err) });
    }
  };

  /** Owning player only (RN-15): the sheet goes to the master again. */
  readonly resubmit = async (campaignId: string, characterId: string): Promise<void> => {
    this.resubmitState.set({ status: 'saving' });
    try {
      const key = this.resubmitKey.keyFor({ campaignId, characterId });
      const vm = await this.host.source.resubmitCharacter(campaignId, characterId, key);
      if (!this.host.apply(characterId, vm)) {
        return;
      }
      this.resubmitKey.renew();
      this.resubmitState.set(IDLE_SAVING);
      this.resubmitted.set(true);
    } catch (err) {
      this.resubmitState.set({ status: 'error', message: describeCharacterError(err) });
    }
  };

  readonly markDead = async (campaignId: string, characterId: string): Promise<void> => {
    this.markDeadState.set({ status: 'saving' });
    try {
      const vm = await this.host.source.markCharacterDead(campaignId, characterId);
      if (!this.host.apply(characterId, vm)) {
        return;
      }
      this.confirmingDeath.set(false);
      this.markDeadState.set(IDLE_SAVING);
    } catch (err) {
      this.confirmingDeath.set(false);
      this.markDeadState.set({ status: 'error', message: describeCharacterError(err) });
    }
  };

  /** Master only: flips whether the player may currently edit the story
   * (integrator amendment to A3, 29/09/2026; `SetStoryEditing` takes no
   * revision and never changes one; see `CharacterSheetSource`'s doc
   * comment). Never changes `canEditStory` directly: the response is what
   * updates it, same as every other server-computed flag. */
  readonly toggleStoryEditingAllowed = async (
    campaignId: string,
    characterId: string,
    currentlyAllowed: boolean,
  ): Promise<void> => {
    this.storyToggleState.set({ status: 'saving' });
    try {
      const vm = await this.host.source.setStoryEditingAllowed(
        campaignId,
        characterId,
        !currentlyAllowed,
      );
      if (!this.host.apply(characterId, vm)) {
        return;
      }
      this.storyToggleState.set(IDLE_SAVING);
    } catch (err) {
      this.storyToggleState.set({ status: 'error', message: describeCharacterError(err) });
    }
  };

  /** Master only (MR-024): the character joins the campaign as a draft, and
   * its player as a member. The response is the approved character, so the
   * page simply shows it. */
  readonly approve = async (campaignId: string, characterId: string): Promise<void> => {
    this.approvalState.set({ status: 'saving' });
    try {
      const vm = await this.host.source.approveCharacter(campaignId, characterId);
      if (!this.host.apply(characterId, vm)) {
        return;
      }
      this.approvalState.set(IDLE_SAVING);
    } catch (err) {
      this.approvalState.set({ status: 'error', message: describeCharacterError(err) });
    }
  };

  /** Master only (MR-024), after "Confirmar recusa": the character is gone,
   * so the master goes back to the campaign's page. */
  readonly reject = async (campaignId: string, characterId: string): Promise<void> => {
    this.approvalState.set({ status: 'saving' });
    try {
      await this.host.source.rejectCharacter(campaignId, characterId);
      if (!this.host.stillOn(characterId)) {
        return;
      }
      this.approvalState.set(IDLE_SAVING);
      await this.host.navigate(['/campaigns', campaignId]);
    } catch (err) {
      this.confirmingReject.set(false);
      this.approvalState.set({ status: 'error', message: describeCharacterError(err) });
    }
  };
}
