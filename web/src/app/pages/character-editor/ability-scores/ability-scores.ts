import { Component, computed, effect, inject, input, model, signal } from '@angular/core';
import { MatRadioModule } from '@angular/material/radio';

import { ABILITY_KEYS, AbilityKey } from '../../../core/characters/characters.types';
import {
  AbilityResult,
  Placement,
  emptyPlacement,
  freeCount,
  ROLL_DIE,
  place,
  rollAbilityScores,
  standardArrayResults,
} from '../../../core/dice/dice';
import { AbilityFields, AbilityFormGroup } from '../ability-fields/ability-fields';
import { AbilityPlacing } from './ability-placing/ability-placing';

export type AbilityMethod = 'type' | 'roll' | 'standard';

/** Results and where they went, for one of the two placing methods. */
interface Stage {
  readonly results: readonly AbilityResult[];
  readonly placement: Placement;
}

/**
 * The scores of the "Atributos" step (E6-20, E6-20b): how to set them
 * ("Como definir os valores") and then the six values. "Digitar" is today's
 * six fields. "Rolar 4d6" rolls six scores in the browser (nothing is
 * recorded, RN-18) and "Conjunto padrão" offers 15, 14, 13, 12, 10 and 8;
 * both end with the person placing each result on an ability.
 *
 * What lands on the sheet is only ever the number in the form group: the
 * placing writes it there as the person places. Until every result has an
 * ability the step is `incomplete`, and the page refuses to save, so a
 * half-placed roll never turns into six 10s.
 */
@Component({
  selector: 'app-ability-scores',
  imports: [AbilityFields, AbilityPlacing, MatRadioModule],
  templateUrl: './ability-scores.html',
  styleUrl: './ability-scores.scss',
})
export class AbilityScores {
  /** The form's six base scores (1 to 30). */
  readonly group = input.required<AbilityFormGroup>();
  /** True while a placing method has results with no ability yet. */
  readonly incomplete = model(false);

  private readonly rollDie = inject(ROLL_DIE);
  protected readonly method = signal<AbilityMethod>('type');
  private readonly rolled = signal<Stage | null>(null);
  private readonly standard = signal<Stage | null>(null);

  protected readonly stage = computed(() =>
    this.method() === 'roll'
      ? this.rolled()
      : this.method() === 'standard'
        ? this.standard()
        : null,
  );

  protected readonly methods: readonly { value: AbilityMethod; title: string; note: string }[] = [
    { value: 'type', title: 'Digitar', note: 'Você escolhe cada valor.' },
    { value: 'roll', title: 'Rolar 4d6', note: 'Rola 4 dados e descarta o menor.' },
    { value: 'standard', title: 'Conjunto padrão', note: '15, 14, 13, 12, 10 e 8.' },
  ];

  constructor() {
    effect(() => {
      const stage = this.stage();
      this.incomplete.set(stage !== null && freeCount(stage.placement) > 0);
    });
  }

  protected choose(method: AbilityMethod): void {
    this.method.set(method);
    // The first time a method is chosen it rolls (or lays out the array);
    // going back to it later finds the same results where they were left.
    if (method === 'roll' && this.rolled() === null) {
      this.reroll();
    } else if (method === 'standard' && this.standard() === null) {
      this.standard.set({ results: standardArrayResults(), placement: emptyPlacement() });
    }
  }

  protected reroll(): void {
    this.rolled.set({ results: rollAbilityScores(this.rollDie), placement: emptyPlacement() });
  }

  protected placeResult(index: number, ability: AbilityKey): void {
    const stage = this.stage();
    if (!stage) {
      return;
    }
    const next = { results: stage.results, placement: place(stage.placement, index, ability) };
    (this.method() === 'roll' ? this.rolled : this.standard).set(next);
    // The form gets the number of every ability that has one (a swap moves two).
    for (const key of ABILITY_KEYS) {
      const at = next.placement[key];
      if (at !== null) {
        this.group().controls[key].setValue(next.results[at].total);
      }
    }
  }
}
