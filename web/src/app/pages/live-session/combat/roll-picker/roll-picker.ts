import { NgTemplateOutlet } from '@angular/common';
import { Component, computed, input, model, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { parseSum, typedTotal } from '../../../../core/combat/combat-dice';
import { tight } from '../../../../core/combat/combat-grid';

let nextId = 0;

/**
 * The two ways to roll one die, inside the attack sheet, the master's card
 * and the damage step (E6-07, E6-08, RN-18): "Rolar no app" and "Digitar o
 * resultado". The player's saved choice is the filled button and the other a
 * text link; a forced mode (`canApp` or `canType` false) hides the other.
 * Typing is the physical roll: a field of 64px, the live total in a status
 * region, and "Confirmar 16" until the number is valid (a d20 takes 1 to 20,
 * a damage the sum of its dice, N to N times the faces: Q38). The page picks
 * the words with `label` and `hint`; the sheet swaps its title while the
 * person types (`typing`).
 */
@Component({
  selector: 'app-roll-picker',
  imports: [MatButtonModule, MatIconModule, NgTemplateOutlet],
  templateUrl: './roll-picker.html',
  styleUrl: './roll-picker.scss',
})
export class RollPicker {
  /** "Rolar no app" is allowed (the campaign did not force physical dice). */
  readonly canApp = input(true);
  /** "Digitar o resultado" is allowed (the campaign did not force the app). */
  readonly canType = input(true);
  /** The player's choice: the filled button is the app's, or the typed one. */
  readonly preferApp = input(true);
  /** The face range of what is typed: 1 to 20 for a d20, N to N×faces for damage. */
  readonly min = input(1);
  readonly max = input(20);
  /** Added to the typed number: the attack bonus, or the damage modifier. */
  readonly modifier = input(0);
  /** The field's label ("Role 1d20 para o Machado de batalha (+5)"). */
  readonly label = input.required<string>();
  /** The line under it ("Role o seu dado e digite o número que saiu (1 a 20)."). */
  readonly hint = input('');
  /** The caption of the total box ("Total do ataque"). */
  readonly totalNote = input('Total');
  /** What the buttons are called when the way is a damage roll. */
  readonly appLabel = input('Rolar no app');
  /** The master's NPC card has no filled button of its own ("Próximo turno" owns it). */
  readonly outlined = input(false);
  readonly busy = input(false);
  /** The buttons stick to the bottom of the sheet that scrolls (the attack sheet). */
  readonly sticky = input(false);
  /** Typing mode, two-way: the sheet reads it to change its title. */
  readonly typing = model(false);

  readonly app = output<void>();
  readonly typed = output<number>();

  protected readonly id = `roll-picker-${nextId++}`;
  protected readonly text = signal('');
  protected readonly value = computed(() => parseSum(this.text(), this.min(), this.max()));
  protected readonly invalid = computed(() => this.text().trim() !== '' && this.value() === null);
  protected readonly showTyping = computed(() => this.typing() || !this.canApp());
  protected readonly errorText = computed(() => tight(`Digite um número de ${this.min()} a ${this.max()}`));
  protected readonly hintText = computed(() => tight(this.hint()));
  /** What a screen reader hears while the well shows only "—". */
  protected readonly waitText = computed(() => tight(`O total aparece quando o número for de ${this.min()} a ${this.max()}.`));
  protected readonly total = computed(() => {
    const v = this.value();
    return v === null ? null : { sum: v + this.modifier(), text: typedTotal(v, this.modifier()) };
  });
  protected readonly bonusText = computed(() => {
    const m = this.modifier();
    return m === 0 ? '' : `${m < 0 ? '−' : '+'} ${Math.abs(m)} de ${this.max() === 20 ? 'bônus' : 'modificador'}`;
  });

  /** Back to the two buttons, with the field empty (after a roll went through). */
  reset(): void {
    this.typing.set(false);
    this.text.set('');
  }

  protected onType(event: Event): void {
    this.text.set((event.target as HTMLInputElement).value);
  }

  protected confirm(): void {
    const v = this.value();
    if (v !== null && !this.busy()) {
      this.typed.emit(v);
    }
  }
}
