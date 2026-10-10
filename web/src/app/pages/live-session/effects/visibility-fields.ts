import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

import { EffectAudience } from '../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { MAX_LABEL } from '../../../core/effects/effects-text';
import { TextField } from '../../../shared/form-fields/text-field';

let nextId = 0;

/**
 * The two choices of what the players read of a visible effect (W7-E board 4c): which players ("Todos os jogadores" or
 * "Só o dono do alvo", in radios) and the free label ("O que aparece", at most 30 characters). The parent owns the
 * values. The server does not read the text, so the field says not to put numbers in it.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-visibility-fields',
  imports: [TextField],
  template: `
    <fieldset class="choices">
      <legend class="mr-visually-hidden">Quais jogadores veem</legend>
      <label class="choice">
        <input
          type="radio"
          [name]="name"
          [checked]="audience() === Audience.ALL"
          (change)="audienceChange.emit(Audience.ALL)"
        />
        <span class="choice__title">Todos os jogadores</span>
        <span class="choice__text">A etiqueta aparece na ordem e no mapa.</span>
      </label>
      <label class="choice">
        <input
          type="radio"
          [name]="name"
          [checked]="audience() === Audience.OWNER"
          (change)="audienceChange.emit(Audience.OWNER)"
        />
        <span class="choice__title">Só o dono do alvo</span>
        <span class="choice__text">Só quem controla a criatura (em personagem).</span>
      </label>
    </fieldset>
    <app-text-field
      label="O que aparece"
      [value]="label()"
      [maxlength]="maxLabel"
      hint="Até 30 caracteres. Aparece só enquanto o efeito está visível. O app não confere o texto: não ponha números nele."
      (valueChange)="labelChange.emit($event.slice(0, maxLabel))"
    />
  `,
  styleUrls: ['./effects-sheet.scss'],
})
export class VisibilityFields {
  readonly audience = input.required<EffectAudience>();
  readonly label = input.required<string>();
  readonly audienceChange = output<EffectAudience>();
  readonly labelChange = output<string>();

  protected readonly Audience = EffectAudience;
  protected readonly maxLabel = MAX_LABEL;
  protected readonly name = `vis-${nextId++}`;
}
