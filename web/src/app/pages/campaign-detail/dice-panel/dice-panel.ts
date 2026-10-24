import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { DICE_MODE_OPTIONS, preferenceLabel } from '../../../core/campaigns/dice-labels';
import type { MemberRowVm } from '../campaign-detail.copy';
import { PanelAction } from '../panel-action/panel-action';
import { PanelHead } from '../panel-head/panel-head';

/**
 * The master's "Dados" summary on `/campaigns/:id` (E6-17, RN-18): how the campaign's players roll dice, and, while the
 * campaign lets each player choose, what each one chose. The mode itself is edited in one place only, "Regras da mesa"
 * (it is saved there with the rest of the rules, so a stale panel here can never overwrite it), and the link goes there.
 */
@Component({
  selector: 'app-dice-panel',
  imports: [MatIconModule, PanelAction, PanelHead],
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

  protected readonly option = computed(() =>
    DICE_MODE_OPTIONS.find((o) => o.value === this.savedMode()),
  );
  protected readonly showChoices = computed(() => this.savedMode() === DiceMode.PLAYERS_CHOOSE);
  protected initial(m: MemberRowVm): string {
    return m.name.charAt(0).toUpperCase();
  }

  protected readonly players = computed(() => this.members().filter((m) => m.role === 'jogador'));
}
