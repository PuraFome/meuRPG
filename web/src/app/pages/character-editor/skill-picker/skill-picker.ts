import { Component, computed, input, output } from '@angular/core';
import { MatCheckboxModule } from '@angular/material/checkbox';

import { SkillOptionVm } from '../character-editor.types';
import { abilityAbbreviation, countLabel } from '../editor-labels';

/**
 * The "Perícias" step: one row per skill, like the sheet's proficiency
 * rows — the proficiency checkbox (named by the skill alone), the ability
 * abbreviation, and "Especialização", which only a proficient skill can
 * take. The state and its rules stay in `CharacterEditor`
 * (`toggleSkill`/`toggleExpertise`); this only shows it and reports clicks.
 */
@Component({
  selector: 'app-skill-picker',
  imports: [MatCheckboxModule],
  templateUrl: './skill-picker.html',
  styleUrl: './skill-picker.scss',
})
export class SkillPicker {
  readonly skills = input.required<readonly SkillOptionVm[]>();
  readonly proficient = input.required<ReadonlySet<string>>();
  readonly expertise = input.required<ReadonlySet<string>>();

  readonly toggleSkill = output<string>();
  readonly toggleExpertise = output<string>();

  protected readonly abilityAbbreviation = abilityAbbreviation;
  /** Two columns on a wide screen, read top to bottom like the sheet's
   * list (the first half, then the second); stacked on a phone. */
  protected readonly columns = computed(() => {
    const skills = this.skills();
    const half = Math.ceil(skills.length / 2);
    return [skills.slice(0, half), skills.slice(half)].filter((column) => column.length > 0);
  });
  protected readonly summary = computed(() => {
    const proficient = countLabel(
      this.proficient().size,
      'perícia marcada',
      'perícias marcadas',
      'Nenhuma perícia marcada',
    );
    const expertise = this.expertise().size;
    return expertise > 0 ? `${proficient}, ${expertise} com especialização` : proficient;
  });
}
