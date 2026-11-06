import { ChangeDetectionStrategy, Component, computed, inject, input, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import type { CombatState } from '../../../../core/combat/combat-state';
import {
  type LegendaryResistanceCard,
  canConfirmResistance,
  resistanceIds,
} from '../../../../core/combat/legendary-resistance-card';
import { ActionKey } from '../../../../core/connect/idempotency';
import { type OfferFailure, type Sending, offerFailure } from './offer-answer';

let nextId = 0;

/**
 * The master's Legendary Resistance window (#557, boards 03, 08): one row per creature whose saving throw failed, each a
 * checkbox that starts unticked ("a magia espera; os dados já foram rolados"). "Deixar como rolou" comes first and sends
 * no creature; "Usar nas marcadas" sends exactly the ticked ones and stays off while none is ticked (it would be the
 * same as the first button: `canConfirmResistance`). The server spends one use per creature named. Only the master's
 * queue draws it; the players read "Esperando o mestre".
 */
@Component({
  selector: 'app-resistance-offer',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './resistance-offer.html',
  styleUrls: ['./offer.scss', './resistance-offer.scss'],
})
export class ResistanceOffer {
  private readonly api = inject(CombatClient);
  private readonly keys = new ActionKey();

  readonly card = input.required<LegendaryResistanceCard>();
  readonly encounter = input.required<Encounter>();
  readonly campaignId = input.required<string>();
  readonly state = input.required<CombatState>();
  readonly badge = input<'Agora' | 'Depois'>('Agora');
  readonly waiting = input('');

  protected readonly uid = `lr-${nextId++}-`;
  protected readonly ticked = signal<ReadonlySet<string>>(new Set());
  protected readonly sending = signal<Sending>(null);
  protected readonly failure = signal<OfferFailure | null>(null);
  protected readonly said = signal('');
  protected readonly canConfirm = computed(() => canConfirmResistance(this.card(), this.ticked()));
  /** The creatures a retry sends again: the same list, so the same key. */
  private last: readonly string[] | null = null;

  protected toggle(id: string, on: boolean): void {
    this.ticked.update((t) => {
      const next = new Set(t);
      if (on) {
        next.add(id);
      } else {
        next.delete(id);
      }
      return next;
    });
  }

  /** "Deixar como rolou": no creature. */
  protected decline(): Promise<void> {
    return this.answer([]);
  }

  /** "Usar nas marcadas": the ticked creatures, in the window's order. */
  protected confirm(): Promise<void> {
    return this.canConfirm()
      ? this.answer(resistanceIds(this.card(), this.ticked()))
      : Promise.resolve();
  }

  protected retry(): Promise<void> {
    return this.last === null ? Promise.resolve() : this.send(this.last);
  }

  private answer(ids: readonly string[]): Promise<void> {
    this.last = ids;
    return this.send(ids);
  }

  private async send(ids: readonly string[]): Promise<void> {
    if (this.sending()) {
      return;
    }
    this.sending.set(ids.length > 0 ? 'use' : 'pass');
    this.failure.set(null);
    try {
      const res = await this.api.attacks.answerLegendaryResistance(
        this.campaignId(),
        this.encounter().id,
        this.card().id,
        { creatureIds: ids },
        this.keys.keyFor({ window: this.card().id, ids }),
      );
      this.state().apply(res.encounter);
      this.said.set(
        ids.length > 0 ? 'Resistência Lendária usada.' : 'Os testes ficaram como rolados.',
      );
    } catch (err) {
      this.failure.set(offerFailure(err, 'responder à Resistência Lendária'));
    } finally {
      this.sending.set(null);
    }
  }
}
