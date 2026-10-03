import { Component, ElementRef, computed, inject, input, signal, viewChild } from '@angular/core';
import { toObservable, toSignal } from '@angular/core/rxjs-interop';
import { FormBuilder, ReactiveFormsModule } from '@angular/forms';
import { MatButton, MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { map, startWith, switchMap } from 'rxjs';

import { FictionNotice } from '../../../shared/fiction-notice/fiction-notice';
import type { ChallengeRatingVm } from '../character-editor.types';
import { DefeatXp } from '../defeat-xp/defeat-xp';
import { BasicSheetFormGroup, MAX_ATTACKS, createAttackGroup } from './basic-form';
import { NpcAttackCard } from './npc-attack-card';

export type { BasicSheetFormGroup } from './basic-form';

/**
 * The short form of a minion or story NPC (`BasicSheet`), as one compact
 * panel: the name, the combat numbers, up to three attack cards, and the
 * description with the fiction notice next to it. The form itself and its
 * submit stay in `CharacterEditor`.
 */
@Component({
  selector: 'app-npc-short-form',
  imports: [
    DefeatXp,
    FictionNotice,
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    NpcAttackCard,
    ReactiveFormsModule,
  ],
  templateUrl: './npc-short-form.html',
  styleUrl: './npc-short-form.scss',
})
export class NpcShortForm {
  readonly form = input.required<BasicSheetFormGroup>();
  /** The ND to XP table, for the picker. */
  readonly ratings = input<readonly ChallengeRatingVm[]>([]);
  /** "Ao ser derrotado" is for a minion, which fights; a story NPC keeps no XP. */
  readonly showDefeat = input(false);

  private readonly fb = inject(FormBuilder);
  private readonly addButton = viewChild<MatButton, ElementRef<HTMLElement>>('addButton', {
    read: ElementRef,
  });

  protected readonly maxAttacks = MAX_ATTACKS;
  /** What the last removal did, for a screen reader ("Ataque 2 removido."). */
  protected readonly status = signal('');

  /** How many cards there are now. The array's `valueChanges` fires on every
   * add and remove, which is what redraws the list and the counter. */
  protected readonly count = toSignal(
    toObservable(this.form).pipe(
      switchMap((form) =>
        form.controls.attacks.valueChanges.pipe(startWith(form.controls.attacks.value)),
      ),
      map((attacks) => attacks.length),
    ),
    { initialValue: 0 },
  );
  protected readonly full = computed(() => this.count() >= MAX_ATTACKS);

  /** The typed speed as the sheet shows it ("9 m"), or nothing while the
   * field is empty or not a step of 1,5 m. */
  protected speedLabel(): string | null {
    const control = this.form().controls.speedWalkM;
    return control.valid ? `${String(control.value).replace('.', ',')} m` : null;
  }

  protected addAttack(): void {
    const attacks = this.form().controls.attacks;
    if (attacks.length < MAX_ATTACKS) {
      attacks.push(createAttackGroup(this.fb));
      this.status.set('');
    }
  }

  protected removeAttack(index: number): void {
    this.form().controls.attacks.removeAt(index);
    this.status.set(`Ataque ${index + 1} removido.`);
    // The card that had the focus is gone: the next stop is the add button.
    queueMicrotask(() => this.addButton()?.nativeElement.focus());
  }
}
