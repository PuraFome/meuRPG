import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { ChangeRowsList } from '../change-rows/change-rows';
import { LevelUpSession } from '../level-up-session';

/** What stays locked, as the paper sheet names it. */
const LOCKED = [
  'Nome, raça e antecedente',
  'Valores base das habilidades',
  'Perícias e proficiências',
  'Equipamento e ataques',
  'História e anotações',
];

/**
 * The desktop page's right column (MR-040, E8-15): "O que muda até aqui", which follows every
 * choice (a polite status region, so a screen reader hears the change without the page
 * scrolling), and "O resto da ficha", the read-only rows with the lock. From 1100px it stands
 * beside the step; under that the page drops it below the step, full width.
 */
@Component({
  selector: 'app-side-column',
  imports: [ChangeRowsList, MatIconModule],
  templateUrl: './side-column.html',
  styleUrl: './side-column.scss',
})
export class SideColumn {
  readonly s = input.required<LevelUpSession>();
  /** "O que muda até aqui" is the summary itself on the last step: it is left out there. */
  readonly showChanges = input(true);
  protected readonly locked = LOCKED;
}
