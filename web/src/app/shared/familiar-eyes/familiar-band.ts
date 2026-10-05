import { Component, computed, effect, inject, input, output, signal, untracked } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { FamiliarEyesClient, familiarName, familiarSightMessage } from '../../core/play/familiar-eyes';
import { newKey } from '../../core/connect/idempotency';

/**
 * The band that stays at the top of the page while the player looks through their
 * familiar's eyes (MR-036, E9-04 states 3 and 4): amber, with an icon and words —
 * "Você está vendo pelos olhos do Nanquim", then "Pensantus está cego e surdo" (and,
 * in a combat, "Até o começo da sua próxima vez, a rodada 3") — and the screen's one
 * filled button, "Voltar aos seus olhos". `role="status"`, so it is read when it
 * appears. The other players and the master see no band: only the conditions, in
 * words, in the combat.
 */
@Component({
  selector: 'app-familiar-band',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './familiar-band.html',
  styleUrl: './familiar-band.scss',
})
export class FamiliarBand {
  private readonly api = inject(FamiliarEyesClient);

  readonly campaignId = input.required<string>();
  readonly characterId = input.required<string>();
  readonly characterName = input.required<string>();
  /** The familiar (a creature ID): its name is read once. */
  readonly creatureId = input.required<string>();
  /** Started in a combat: it ends by itself at the start of the next turn. */
  readonly inCombat = input(false);
  /** The round the sight ends in (the combat's round + 1), when in a combat. */
  readonly endsInRound = input<number | null>(null);

  /** "Voltar aos seus olhos" worked. */
  readonly stopped = output<void>();

  private readonly name = signal<string | null>(null);
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  private readonly key = newKey();

  protected readonly who = computed(() => familiarName(this.name()));
  protected readonly title = computed(() => `Você está vendo pelos olhos do ${this.name()?.trim() || 'familiar'}.`);

  constructor() {
    effect(() => {
      const campaignId = this.campaignId();
      const characterId = this.characterId();
      const creatureId = this.creatureId();
      untracked(() => void this.api.name(campaignId, characterId, creatureId).then((n) => this.name.set(n)));
    });
  }

  protected async back(): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      await this.api.stop(this.campaignId(), this.characterId(), this.key);
      this.stopped.emit();
    } catch (err) {
      this.error.set(familiarSightMessage(err, this.who()));
    } finally {
      this.busy.set(false);
    }
  }
}
