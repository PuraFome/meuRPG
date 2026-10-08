import { Component, computed, inject, input, model } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import { formatModifier } from '../../../core/characters/character-labels';
import { ROLL_DIE } from '../../../core/dice/dice';
import { hitPointsPreview, validRoll } from '../hit-points-preview';

/** "2 e 3", "2, 3 e 4": the levels still to roll, written as a list. */
const LEVEL_LIST = new Intl.ListFormat('pt-BR', { type: 'conjunction' });

/**
 * The level-by-level rolls of "Pontos de vida" → "Rolado" (E6-21): a row per
 * level from the 2nd with its field ("Nível 2 (1d12)"), a "Rolar" button that
 * fills it (typing a roll made at the table is still allowed), the formula
 * the row adds, and a box with the total so far.
 *
 * The rows are the dice and the Constitution modifier, so the player sees where
 * the number comes from while they roll. What the effects add (Dwarven Toughness,
 * a table's own effect) is the server's number, `fromEffects`, derived for the
 * draft by `PreviewCharacter`: the browser has no effect maths. Without it the
 * box is only a preview of the dice and says so. `constitution` is the final
 * score (base, race and manual bonuses).
 */
@Component({
  selector: 'app-hit-points-rolls',
  imports: [MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule],
  templateUrl: './hit-points-rolls.html',
  styleUrl: './hit-points-rolls.scss',
})
export class HitPointsRolls {
  private readonly rollDie = inject(ROLL_DIE);
  /** Faces of the class's hit die. */
  readonly hitDie = input.required<number>();
  /** The die of each level after the first when the classes differ (a multiclass); index 0 = level 2. */
  readonly levelDice = input<readonly number[] | null>(null);
  readonly level = input.required<number>();
  readonly constitution = input.required<number>();
  /**
   * What the effects of the race, the class and the features add to the hit points (`DerivedSheet.hit_points_from_effects`),
   * as the server derived it for this draft; `null` while there is none, and the box then adds up the dice and the
   * Constitution modifier alone, with its note.
   */
  readonly fromEffects = input<number | null>(null);
  /** One entry per level after the first, index 0 = level 2; 0 means "not rolled yet". */
  readonly rolls = model.required<readonly number[]>();

  /** The rows and the arithmetic of the dice; the effects' share is added in `total`, `min` and `max`. */
  protected readonly preview = computed(() =>
    hitPointsPreview(
      this.hitDie(),
      this.level(),
      this.constitution(),
      this.rolls(),
      this.levelDice(),
    ),
  );
  private readonly effects = computed(() => this.fromEffects() ?? 0);
  protected readonly total = computed(() => this.preview().total + this.effects());
  protected readonly min = computed(() => this.preview().min + this.effects());
  protected readonly max = computed(() => this.preview().max + this.effects());
  /** "+1" or "−1" for the line that names what the effects add; empty when the server has said nothing or they add nothing. */
  protected readonly effectsWords = computed(() => {
    const n = this.fromEffects();
    if (!n) {
      return '';
    }
    return n < 0 ? `−${-n}` : `+${n}`;
  });
  protected readonly modifier = computed(() => formatModifier(this.preview().constitutionModifier));
  /** "+ 3" or "− 1", the way the formula line writes it. */
  protected readonly modifierWords = computed(() => {
    const mod = this.preview().constitutionModifier;
    return mod < 0 ? `− ${-mod}` : `+ ${mod}`;
  });
  /** The levels still to roll as Portuguese writes a list: "2 e 3", "2, 3 e 4". */
  protected readonly missingWords = computed(() =>
    LEVEL_LIST.format(this.preview().missing.map(String)),
  );

  protected rollOf(level: number): number | null {
    return validRoll(this.rolls()[level - 2], this.dieOf(level));
  }

  protected dieOf(level: number): number {
    return this.levelDice()?.[level - 2] || this.hitDie();
  }

  protected typed(level: number, value: number): void {
    this.set(level, Number.isNaN(value) ? 0 : value);
  }

  protected roll(level: number): void {
    this.set(level, this.rollDie(this.dieOf(level)));
  }

  /** Rolls only the rows that are still empty. */
  protected rollMissing(): void {
    const next = Array.from({ length: Math.max(this.level() - 1, 0) }, (_, i) =>
      validRoll(this.rolls()[i], this.dieOf(i + 2)) === null
        ? this.rollDie(this.dieOf(i + 2))
        : this.rolls()[i],
    );
    this.rolls.set(next);
  }

  private set(level: number, value: number): void {
    const next = Array.from(
      { length: Math.max(this.level() - 1, 0) },
      (_, i) => this.rolls()[i] ?? 0,
    );
    next[level - 2] = value;
    this.rolls.set(next);
  }
}
