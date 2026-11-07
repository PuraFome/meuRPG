import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { ChoiceKind, ChoiceUse } from '../../../../core/content/effect-draft';
import { CHOICE_GROUP_ICON } from '../../../../shared/effect-picker/effect-picker-choices';

/** What a kind of choice is called in a field's list, and what the field takes. */
const KINDS: Readonly<Record<ChoiceKind, { title: string; field: string; feminine: boolean }>> = {
  ability: { title: 'Habilidade', field: 'uma habilidade', feminine: true },
  damage_type: { title: 'Tipo de dano', field: 'um tipo de dano', feminine: false },
};

/**
 * "Escolhas deste talento": the feat's named choices, each with the effect that holds it and how many effects use it.
 * It sits under "Para os jogadores" and shows only when the feat has a named choice.
 * The note says the master types no name: the fields offer the choice before their fixed values.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-feat-choices',
  imports: [MatIconModule],
  templateUrl: './feat-choices.html',
  styleUrl: './feat-choices.scss',
})
export class FeatChoices {
  readonly uses = input.required<readonly ChoiceUse[]>();
  protected readonly icon = CHOICE_GROUP_ICON;

  protected readonly rows = computed(() =>
    this.uses().map(({ choice, effects }) => {
      const used = KINDS[choice.kind].feminine ? 'usada' : 'usado';
      return {
        key: choice.name,
        label: choice.label,
        place: `efeito ${choice.index + 1}`,
        used:
          effects === 0
            ? `ainda não ${used}`
            : `${used} em ${effects} ${effects === 1 ? 'efeito' : 'efeitos'}`,
      };
    }),
  );

  /** The words every kind of choice here starts the line with, once each. */
  protected readonly note = computed(() => {
    const kinds = [...new Set(this.uses().map((u) => u.choice.kind))];
    const lines = kinds.map(
      (k) =>
        `"${KINDS[k].title}" vem antes dos valores fixos nos campos que aceitam ${KINDS[k].field}`,
    );
    return `Você não digita nada: ${lines.join('; ')}.`;
  });
}
