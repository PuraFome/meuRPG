import { Component, input } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

import { formatSpeedFt } from '../../../core/characters/character-labels';
import { FictionNotice } from '../../../shared/fiction-notice/fiction-notice';

export type BasicSheetFormGroup = FormGroup<{
  name: FormControl<string>;
  hitPointsMax: FormControl<number>;
  armorClass: FormControl<number>;
  speedWalkFt: FormControl<number>;
  attackBonus: FormControl<number>;
  damage: FormControl<string>;
  description: FormControl<string>;
}>;

/**
 * The short form of a minion or story NPC (`BasicSheet`), as one compact
 * panel: the name, the four combat numbers side by side, the damage, and
 * the description with the fiction notice next to it. The form itself and
 * its submit stay in `CharacterEditor`.
 */
@Component({
  selector: 'app-npc-short-form',
  imports: [FictionNotice, MatFormFieldModule, MatInputModule, ReactiveFormsModule],
  templateUrl: './npc-short-form.html',
  styleUrl: './npc-short-form.scss',
})
export class NpcShortForm {
  readonly form = input.required<BasicSheetFormGroup>();

  /** The typed speed in both units, the way the sheet shows it ("9 m (30
   * pés)"), or nothing while the field is empty. Unit formatting only. */
  protected speedLabel(): string | null {
    const feet = this.form().controls.speedWalkFt.value;
    return typeof feet === 'number' && Number.isFinite(feet) && feet >= 0
      ? formatSpeedFt(feet)
      : null;
  }
}
