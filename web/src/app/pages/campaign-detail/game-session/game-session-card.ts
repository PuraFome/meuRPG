import { Component, OnInit, inject, input, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { Code } from '@connectrpc/connect';

import { describeConnectError } from '../../../core/connect/connect-errors';
import { GameSessionSource, GameSessionVm } from './game-session-card.types';

type CardState =
  | { status: 'loading' }
  | { status: 'ready'; session: GameSessionVm | null }
  | { status: 'error'; message: string };

type ActionState = { status: 'idle' } | { status: 'saving' } | { status: 'error'; message: string };

const MASTER_ONLY_MESSAGES = {
  [Code.PermissionDenied]: 'Só o mestre da campanha pode gerenciar sessões.',
  [Code.Unavailable]: 'Não foi possível falar com o servidor agora. Tente de novo em instantes.',
};

/**
 * The master's "Sessão" card on `/campanhas/:id` (RN-01 / MR-006): starting
 * a session locks every player's sheet in the campaign; ending it does not
 * unlock them (RN-01 — a locked sheet only unlocks a player back via the
 * master's own edit, never automatically). Only rendered by `CampaignDetail`
 * for the master, same as `CampaignInvites`.
 */
@Component({
  selector: 'app-game-session-card',
  imports: [MatButtonModule, MatCardModule],
  templateUrl: './game-session-card.html',
  styleUrl: './game-session-card.scss',
})
export class GameSessionCard implements OnInit {
  private readonly source = inject(GameSessionSource);

  readonly campaignId = input.required<string>();

  protected readonly state = signal<CardState>({ status: 'loading' });
  protected readonly actionState = signal<ActionState>({ status: 'idle' });
  /** The locked-sheet count from the last `StartGameSession` call in this
   * component's lifetime — cleared on reload, shown as "N fichas travadas"
   * right under the "em andamento" line (integrator amendment, 29/09/2026). */
  protected readonly lastLockedSheetCount = signal<number | null>(null);

  ngOnInit(): void {
    this.load();
  }

  private load(): void {
    this.state.set({ status: 'loading' });
    this.lastLockedSheetCount.set(null);
    this.source.getCurrentSession(this.campaignId()).then(
      (session) => this.state.set({ status: 'ready', session }),
      (err: unknown) => {
        this.state.set({ status: 'error', message: describeConnectError(err, MASTER_ONLY_MESSAGES) });
      },
    );
  }

  protected async startSession(): Promise<void> {
    this.actionState.set({ status: 'saving' });
    try {
      const result = await this.source.startGameSession(this.campaignId());
      this.state.set({ status: 'ready', session: result.session });
      this.lastLockedSheetCount.set(result.lockedSheetCount);
      this.actionState.set({ status: 'idle' });
    } catch (err) {
      this.actionState.set({
        status: 'error',
        message: describeConnectError(err, {
          ...MASTER_ONLY_MESSAGES,
          [Code.FailedPrecondition]: 'Já existe uma sessão em andamento nesta campanha.',
        }),
      });
    }
  }

  protected async endSession(gameSessionId: string): Promise<void> {
    this.actionState.set({ status: 'saving' });
    try {
      await this.source.endGameSession(this.campaignId(), gameSessionId);
      // A session that just ended is no longer "current" for this card.
      this.state.set({ status: 'ready', session: null });
      this.lastLockedSheetCount.set(null);
      this.actionState.set({ status: 'idle' });
    } catch (err) {
      this.actionState.set({
        status: 'error',
        message: describeConnectError(err, {
          ...MASTER_ONLY_MESSAGES,
          [Code.FailedPrecondition]: 'Não há nenhuma sessão em andamento nesta campanha.',
        }),
      });
    }
  }
}
