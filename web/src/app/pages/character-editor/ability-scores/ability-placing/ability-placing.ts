import { Component, computed, effect, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import { abilityLabel } from '../../../../core/characters/character-labels';
import { ABILITY_KEYS, AbilityKey } from '../../../../core/characters/characters.types';
import { AbilityResult, Placement, freeCount, holderOf } from '../../../../core/dice/dice';
import { PHONE_QUERY, mediaQuery } from '../../../../shared/map-view/media-query';
import { DiceResult, DiceResultState } from '../dice-result/dice-result';

/**
 * Where the rolled (or standard-array) results go (E6-20, E6-20b): the
 * results as chips, best first, and the six abilities to put them on.
 *
 * - From 768px each ability is a select of the results; picking one that
 *   another ability holds swaps the two.
 * - On a phone the player taps a result ("Escolhido"), then an ability;
 *   tapping an ability that already has a result picks that result up to
 *   move it, and the tap on the other ability swaps them. Focus stays on
 *   the ability just tapped.
 *
 * The placing itself (`Placement`, in `core/dice`) lives in the parent: this
 * only shows it and reports what the person did.
 */
@Component({
  selector: 'app-ability-placing',
  imports: [DiceResult, MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule],
  templateUrl: './ability-placing.html',
  styleUrl: './ability-placing.scss',
})
export class AbilityPlacing {
  readonly results = input.required<readonly AbilityResult[]>();
  readonly placement = input.required<Placement>();
  /** "Rolar de novo" exists only for rolled results (not the standard array). */
  readonly canReroll = input(false);

  readonly place = output<{ index: number; ability: AbilityKey }>();
  readonly reroll = output<void>();

  protected readonly keys = ABILITY_KEYS;
  protected readonly abilityLabel = abilityLabel;
  protected readonly onPhone = mediaQuery(PHONE_QUERY);

  /** Phone only: the result waiting for an ability. */
  protected readonly picked = signal<number | null>(null);
  /** The words the live region says after each tap. */
  protected readonly status = signal('');

  /** The result indexes best first (they already are, but a re-roll is not trusted to stay so). */
  protected readonly order = computed(() =>
    this.results()
      .map((_, i) => i)
      .sort((a, b) => this.results()[b].total - this.results()[a].total),
  );

  protected readonly free = computed(() => freeCount(this.placement()));
  protected readonly pickedTotal = computed(() => {
    const i = this.picked();
    return i === null ? null : this.results()[i].total;
  });

  /** "Resultados: 17, 15, 14, 12, 11 e 9", read out after a roll. */
  protected readonly announcement = computed(() => {
    const totals = this.order().map((i) => this.results()[i].total);
    return `Resultados: ${totals.slice(0, -1).join(', ')} e ${totals[totals.length - 1]}`;
  });

  constructor() {
    // A new list of results (a re-roll) drops whatever was picked.
    effect(() => {
      this.results();
      this.picked.set(null);
      this.status.set('');
    });
  }

  protected stateOf(index: number): DiceResultState {
    if (this.picked() === index) {
      return 'chosen';
    }
    return holderOf(this.placement(), index) === null ? 'free' : 'placed';
  }

  protected holderLabel(index: number): string | null {
    const holder = holderOf(this.placement(), index);
    return holder === null ? null : abilityLabel(holder);
  }

  /** Desktop: the total, plus where it already sits if on another ability.
   * The select shows the chosen option's text in big type, so the dice stay
   * on the chips and out of here. */
  protected optionText(index: number, forAbility: AbilityKey): string {
    const holder = holderOf(this.placement(), index);
    const elsewhere =
      holder !== null && holder !== forAbility ? ` (em ${abilityLabel(holder)})` : '';
    return `${this.results()[index].total}${elsewhere}`;
  }

  protected selectChanged(ability: AbilityKey, value: string): void {
    if (value !== '') {
      this.place.emit({ index: Number(value), ability });
    }
  }

  /** Phone: tap a result (again to put it down). */
  protected tapResult(index: number): void {
    const next = this.picked() === index ? null : index;
    this.picked.set(next);
    this.status.set(
      next === null
        ? ''
        : `${this.results()[next].total} escolhido. Toque no atributo que vai receber o ${this.results()[next].total}.`,
    );
  }

  /** Phone: tap an ability. With a result picked it goes there; with none, a filled ability picks its result up. */
  protected tapAbility(ability: AbilityKey): void {
    const picked = this.picked();
    const held = this.placement()[ability];
    if (picked === null) {
      if (held !== null) {
        this.tapResult(held);
      }
      return;
    }
    this.place.emit({ index: picked, ability });
    this.status.set(`${this.results()[picked].total} em ${abilityLabel(ability)}.`);
    this.picked.set(null);
  }

  protected abilityButtonLabel(ability: AbilityKey): string {
    const held = this.placement()[ability];
    const name = abilityLabel(ability);
    if (held !== null) {
      return `${name}: ${this.results()[held].total}`;
    }
    const total = this.pickedTotal();
    return total === null ? `${name}: livre` : `${name}: colocar o ${total}`;
  }
}
