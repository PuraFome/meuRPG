import { Component, computed, inject, input, model } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import { formatModifier } from '../../../core/characters/character-labels';
import { ROLL_DIE } from '../../../core/dice/dice';
import { hitPointsPreview, validRoll } from '../hit-points-preview';

/**
 * The level-by-level rolls of "Pontos de vida" → "Rolado" (E6-21): a row per
 * level from the 2nd with its field ("Nível 2 (1d12)"), a "Rolar" button that
 * fills it (typing a roll made at the table is still allowed), the formula
 * the row adds, and a box with the total so far.
 *
 * The box is a preview. The server computes the real maximum when the sheet
 * is saved (`rules.Derive`), with the same arithmetic; this only shows the
 * player where the number comes from while they roll. `constitution` is the
 * final score (base, race and manual bonuses).
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
  readonly level = input.required<number>();
  readonly constitution = input.required<number>();
  /** One entry per level after the first, index 0 = level 2; 0 means "not rolled yet". */
  readonly rolls = model.required<readonly number[]>();

  protected readonly preview = computed(() =>
    hitPointsPreview(this.hitDie(), this.level(), this.constitution(), this.rolls()),
  );
  protected readonly modifier = computed(() => formatModifier(this.preview().constitutionModifier));
  /** "+ 3" or "− 1", the way the formula line writes it. */
  protected readonly modifierWords = computed(() => {
    const mod = this.preview().constitutionModifier;
    return mod < 0 ? `− ${-mod}` : `+ ${mod}`;
  });

  protected rollOf(level: number): number | null {
    return validRoll(this.rolls()[level - 2], this.hitDie());
  }

  protected typed(level: number, value: number): void {
    this.set(level, Number.isNaN(value) ? 0 : value);
  }

  protected roll(level: number): void {
    this.set(level, this.rollDie(this.hitDie()));
  }

  /** Rolls only the rows that are still empty. */
  protected rollMissing(): void {
    const next = Array.from({ length: Math.max(this.level() - 1, 0) }, (_, i) =>
      validRoll(this.rolls()[i], this.hitDie()) === null
        ? this.rollDie(this.hitDie())
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
