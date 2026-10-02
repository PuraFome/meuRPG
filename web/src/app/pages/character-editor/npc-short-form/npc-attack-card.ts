import { Component, computed, input, output } from '@angular/core';
import { toObservable, toSignal } from '@angular/core/rxjs-interop';
import { ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { startWith, switchMap } from 'rxjs';

import {
  DAMAGE_TYPE_OPTIONS,
  damageTypeLabel,
  formatDamageDice,
} from '../../../core/characters/character-labels';
import { BasicAttackFormGroup, parseDice, parseSigned } from './basic-form';

/**
 * One attack of the NPC short form (E6-26): name, attack bonus, damage dice,
 * damage bonus and type, a remove button, and the "Na ficha:" line that
 * shows how the player will read it.
 */
@Component({
  selector: 'app-npc-attack-card',
  imports: [
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    ReactiveFormsModule,
  ],
  templateUrl: './npc-attack-card.html',
  styleUrl: './npc-attack-card.scss',
})
export class NpcAttackCard {
  readonly group = input.required<BasicAttackFormGroup>();
  /** 1-based, as the heading and the labels say it. */
  readonly number = input.required<number>();
  readonly remove = output<void>();

  protected readonly damageTypes = DAMAGE_TYPE_OPTIONS;

  /** The card's fields, again on every change (the summary reads them). */
  private readonly value = toSignal(
    toObservable(this.group).pipe(
      switchMap((group) => group.valueChanges.pipe(startWith(group.getRawValue()))),
    ),
    { initialValue: {} as Partial<ReturnType<BasicAttackFormGroup['getRawValue']>> },
  );

  /** "Cimitarra, +4 para acertar, 1d6 + 2 de dano cortante", or nothing
   * while a field is still invalid. */
  protected readonly summary = computed(() => {
    const v = this.value();
    const dice = parseDice(v.dice ?? '');
    const attackBonus = parseSigned(v.attackBonus ?? '');
    const damageBonus = parseSigned(v.damageBonus ?? '');
    if (!v.name?.trim() || !dice || attackBonus === null || damageBonus === null || !v.damageType) {
      return null;
    }
    const to = (n: number) => (n >= 0 ? `+${n}` : `−${Math.abs(n)}`);
    return (
      `${v.name.trim()}, ${to(attackBonus)} para acertar, ` +
      `${formatDamageDice(dice.count, dice.sides, damageBonus)} de dano ${damageTypeLabel(v.damageType)}`
    );
  });
}
