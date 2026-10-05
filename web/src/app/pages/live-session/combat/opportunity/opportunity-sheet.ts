import { Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { OpportunityOffer } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { combatErrorMessage } from '../../../../core/combat/combat-errors';
import type { CombatState } from '../../../../core/combat/combat-state';
import { type ReactorAttack, attackLabel, playerQuestion } from '../../../../core/combat/opportunity';
import { SheetFrame } from '../sheet-frame/sheet-frame';
import { injectSheet } from '../sheet-host';

/** What the page hands the opportunity prompt. */
export interface OpportunitySheetData {
  readonly campaignId: string;
  readonly encounterId: string;
  readonly offer: OpportunityOffer;
  readonly round: number;
  /** The reactor's melee attacks with their numbers (the player's own options). */
  readonly attacks: readonly ReactorAttack[];
  readonly state: CombatState;
}

/** What the prompt closes with: the attack the player chose, or nothing for "Não atacar". */
export type OpportunityAnswer = { readonly attackKey: string } | null;

/**
 * "O Goblin 2 está saindo do seu alcance. Ataque de oportunidade?" (E9-13): the
 * player's `alertdialog`, like Escudo's. The server detected the exit and made
 * the offer (`Encounter.opportunity_offers`); the page opens this by itself and it
 * cannot be closed without an answer. Focus starts on the safe "Não atacar"; the
 * answers are the same width (stacked on a phone) and one "Atacar com <arma>" per
 * melee attack, which closes the prompt and hands over to the attack sheet, with
 * the mover as its target. It says what is spent (the reaction) and the weapon,
 * never the target's armor class. If the master answered or skipped meanwhile,
 * it says so and only closes.
 */
@Component({
  selector: 'app-opportunity-sheet',
  imports: [MatButtonModule, MatIconModule, SheetFrame],
  template: `
    <app-sheet-frame title="Ataque de oportunidade" [subtitle]="subtitle()" icon="swords" [phone]="inSheet" [closable]="false">
      @if (error()) {
        <div class="mr-notice mr-notice--danger" role="alert">
          <mat-icon aria-hidden="true">error</mat-icon>
          <p>{{ error() }}</p>
        </div>
      }
      @if (gone()) {
        <p class="what" role="status">O mestre respondeu por você: esse ataque de oportunidade não espera mais a sua resposta.</p>
      } @else {
        <p class="what">{{ question() }}</p>
        <p class="small">Gasta a sua reação.{{ weapons() ? ' ' + weapons() : '' }}</p>
      }
      <div foot>
        @if (gone()) {
          <button mat-stroked-button type="button" class="btn" data-initial-focus (click)="sheet.close(null)">Fechar</button>
        } @else {
          <div class="pair" [class.pair--many]="data.attacks.length > 1">
            <button mat-stroked-button type="button" class="btn" data-initial-focus [disabled]="busy()" (click)="decline()">
              Não atacar
            </button>
            @for (a of data.attacks; track a.key) {
              <button mat-flat-button type="button" class="btn" [disabled]="busy()" (click)="attack(a)">
                {{ label(a) }}
              </button>
            }
          </div>
        }
      </div>
    </app-sheet-frame>
  `,
  styleUrl: './opportunity-sheet.scss',
})
export class OpportunitySheet {
  private readonly api = inject(CombatClient);
  protected readonly sheet = injectSheet<OpportunitySheetData, OpportunityAnswer>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  protected readonly busy = signal(false);
  protected readonly error = signal('');

  protected readonly subtitle = computed(() => `${this.data.offer.moverLabel} · Rodada ${this.data.round}`);
  protected readonly question = computed(() => playerQuestion(this.data.offer));
  /** "Espada longa +5 · 1d8 + 3 cortante." for one attack; the names when there are several. */
  protected readonly weapons = computed(() => {
    const a = this.data.attacks;
    return a.length === 1 ? `${a[0].detail}.` : '';
  });
  /** The offer is no longer in the combat: the master (or the turn) answered it. */
  protected readonly gone = computed(
    () => !this.busy() && !(this.data.state.encounter()?.opportunityOffers ?? []).some((o) => o.id === this.data.offer.id),
  );

  protected label(a: ReactorAttack): string {
    return attackLabel(a);
  }

  protected async decline(): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      this.data.state.apply(await this.api.declineOpportunity(this.data.campaignId, this.data.encounterId, this.data.offer.id));
      this.sheet.close(null);
    } catch (err) {
      this.error.set(combatErrorMessage(err, 'responder'));
    } finally {
      this.busy.set(false);
    }
  }

  protected attack(a: ReactorAttack): void {
    this.sheet.close({ attackKey: a.key });
  }
}
