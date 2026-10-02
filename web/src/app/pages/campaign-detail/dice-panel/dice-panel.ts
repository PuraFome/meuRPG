import { Component, computed, inject, input, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { Code } from '@connectrpc/connect';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CampaignsService } from '../../../core/campaigns/campaigns.service';
import { DICE_MODE_OPTIONS, preferenceLabel } from '../../../core/campaigns/dice-labels';
import { describeConnectError } from '../../../core/connect/connect-errors';
import { DiceChoice } from '../../../shared/dice-choice/dice-choice';
import type { MemberRowVm } from '../campaign-detail.copy';

/**
 * The master's "Dados" panel on `/campanhas/:id` (E6-17, RN-18): how the
 * campaign's players roll dice, and what each player chose. The list of
 * choices only shows while "Cada jogador escolhe" is selected; with the
 * other two modes the choices are kept on the server, but ignored. The
 * change applies from the next roll, which the note says.
 */
@Component({
  selector: 'app-dice-panel',
  imports: [DiceChoice, MatButtonModule, MatIconModule],
  templateUrl: './dice-panel.html',
  styleUrl: './dice-panel.scss',
})
export class DicePanel {
  private readonly campaigns = inject(CampaignsService);

  readonly campaignId = input.required<string>();
  /** The mode the server has now. */
  readonly savedMode = input.required<DiceMode>();
  /** The members, with their dice preference (the master gets it). */
  readonly members = input.required<readonly MemberRowVm[]>();

  protected readonly options = DICE_MODE_OPTIONS;
  protected readonly preferenceLabel = preferenceLabel;
  protected readonly DicePreference = DicePreference;

  /** What the master picked, before saving; null until they touch it. */
  private readonly picked = signal<DiceMode | null>(null);
  /** The mode the server confirmed after the last save, newer than the input. */
  private readonly confirmed = signal<DiceMode | null>(null);
  protected readonly saving = signal(false);
  protected readonly error = signal('');
  protected readonly saved = signal(false);

  protected readonly baseline = computed(() => this.confirmed() ?? this.savedMode());
  protected readonly selected = computed(() => this.picked() ?? this.baseline());
  protected readonly dirty = computed(() => this.selected() !== this.baseline());
  protected readonly showChoices = computed(() => this.selected() === DiceMode.PLAYERS_CHOOSE);
  protected readonly players = computed(() => this.members().filter((m) => m.role === 'jogador'));

  protected pick(mode: DiceMode): void {
    this.picked.set(mode);
    this.saved.set(false);
  }

  protected async save(): Promise<void> {
    this.saving.set(true);
    this.error.set('');
    try {
      const res = await this.campaigns.setDiceMode(this.campaignId(), this.selected());
      this.confirmed.set(res.mode);
      this.picked.set(null);
      this.saved.set(true);
    } catch (err) {
      this.error.set(
        describeConnectError(err, {
          [Code.PermissionDenied]: 'Só o mestre pode mudar os dados da campanha.',
          [Code.Unavailable]: 'Não foi possível salvar agora. Tente de novo em instantes.',
        }),
      );
    } finally {
      this.saving.set(false);
    }
  }
}
