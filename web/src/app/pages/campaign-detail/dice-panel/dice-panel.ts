import { Component, computed, input } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { DICE_MODE_OPTIONS, preferenceLabel } from '../../../core/campaigns/dice-labels';
import type { MemberRowVm } from '../campaign-detail.copy';

/**
 * The master's "Dados" summary on `/campanhas/:id` (E6-17, RN-18): how the campaign's players roll dice, and, while the
 * campaign lets each player choose, what each one chose. The mode itself is edited in one place only, "Regras da mesa"
 * (it is saved there with the rest of the rules, so a stale panel here can never overwrite it), and the link goes there.
 */
@Component({
  selector: 'app-dice-panel',
  imports: [MatButtonModule, MatIconModule, RouterLink],
  templateUrl: './dice-panel.html',
  styleUrl: './dice-panel.scss',
})
export class DicePanel {
  readonly campaignId = input.required<string>();
  /** The mode the server has now. */
  readonly savedMode = input.required<DiceMode>();
  /** The members, with their dice preference (the master gets it). */
  readonly members = input.required<readonly MemberRowVm[]>();

  protected readonly preferenceLabel = preferenceLabel;
  protected readonly DicePreference = DicePreference;

  protected readonly option = computed(() => DICE_MODE_OPTIONS.find((o) => o.value === this.savedMode()));
  protected readonly showChoices = computed(() => this.savedMode() === DiceMode.PLAYERS_CHOOSE);
  protected readonly players = computed(() => this.members().filter((m) => m.role === 'jogador'));
}
