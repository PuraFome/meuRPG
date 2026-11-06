import { ChangeDetectionStrategy, Component, inject, input, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import type { CombatState } from '../../../../core/combat/combat-state';
import type { ParryCard } from '../../../../core/combat/parry-card';
import { ActionKey } from '../../../../core/connect/idempotency';
import { type OfferFailure, type Sending, offerFailure } from './offer-answer';

/**
 * The master's Parry window (#557, boards 01, 08, 10, 12, 13): "Aparar o golpe de Tor?". It exists only when the master
 * set the creature to ask; the server decides Parry otherwise. "Deixar passar" comes first. The answer carries its own
 * key, kept across a retry, so a lost answer sent again never answers twice. Only the master's queue draws it.
 */
@Component({
  selector: 'app-parry-offer',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './parry-offer.html',
  styleUrl: './offer.scss',
})
export class ParryOffer {
  private readonly api = inject(CombatClient);
  private readonly keys = new ActionKey();

  readonly card = input.required<ParryCard>();
  readonly encounter = input.required<Encounter>();
  readonly campaignId = input.required<string>();
  readonly state = input.required<CombatState>();
  /** "Agora" for the window to answer, "Depois" for the ones that wait. */
  readonly badge = input<'Agora' | 'Depois'>('Agora');
  /** "Depois da janela acima.", or `''`. */
  readonly waiting = input('');

  protected readonly sending = signal<Sending>(null);
  protected readonly failure = signal<OfferFailure | null>(null);
  protected readonly said = signal('');
  /** The answer a retry sends again: the same value, so the same key. */
  private last: boolean | null = null;

  protected answer(use: boolean): Promise<void> {
    this.last = use;
    return this.send(use);
  }

  protected retry(): Promise<void> {
    return this.last === null ? Promise.resolve() : this.send(this.last);
  }

  private async send(use: boolean): Promise<void> {
    if (this.sending()) {
      return;
    }
    this.sending.set(use ? 'use' : 'pass');
    this.failure.set(null);
    try {
      const res = await this.api.attacks.answerParry(
        this.campaignId(),
        this.encounter().id,
        this.card().id,
        use,
        this.keys.keyFor({ window: this.card().id, use }),
      );
      this.state().apply(res.encounter);
      this.said.set(use ? 'Golpe aparado.' : 'O golpe passou.');
    } catch (err) {
      this.failure.set(offerFailure(err, 'responder a reação'));
    } finally {
      this.sending.set(null);
    }
  }
}
