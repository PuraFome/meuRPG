import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';

import { MAX_LABEL } from '../../../core/content/spell-effect-draft';
import { RadioCards, type RadioCard } from '../../../shared/form-fields/radio-cards';
import { TextField } from '../../../shared/form-fields/text-field';

/** Who sees the weapon hit's effect and what the others read (#577): the audience of the spell's effect, asked beside the preview. */
export type HitAudienceValue = 'public' | 'owner';

export function audienceCards(marked: boolean): RadioCard[] {
  return [
    {
      value: 'public',
      label: 'Todos na mesa',
      hint: marked
        ? 'Os outros leem só o nome da marca no alvo.'
        : 'Os outros leem só o rótulo em quem conjura.',
    },
    { value: 'owner', label: 'Só quem conjura e você', hint: 'Os outros não leem nada.' },
  ];
}

/**
 * "Quem vê o efeito" of the spell editor's side column, for a spell whose only effect is the one on the caster's next weapon hit:
 * the caster and the master read all of it (dice, trigger, test); with "Todos na mesa" the others read one name (the mark on the
 * marked target, or the label on the caster) and never the dice, the trigger or the test. With "Deixa um efeito nos alvos" on, the same
 * question is asked inside that block, so this card is not shown.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-hit-audience',
  imports: [RadioCards, TextField],
  template: `
    <section class="mr-panel panel" aria-labelledby="hit-audience-t">
      <h2 class="mr-panel__title" id="hit-audience-t">Quem vê o efeito</h2>
      <app-radio-cards
        label="Quem vê o efeito"
        [legend]="false"
        [options]="cards()"
        [value]="visibility()"
        [path]="pathOf('visibility')"
        [issues]="issuesOf()(pathOf('visibility'))"
        (valueChange)="visibilityChange.emit($any($event))"
      />
      @if (visibility() === 'public') {
        <app-text-field
          label="O que os outros jogadores leem"
          [maxlength]="maxLabel"
          [value]="label()"
          [path]="pathOf('player_label_pt')"
          [issues]="issuesOf()(pathOf('player_label_pt'))"
          [hint]="labelHint()"
          (valueChange)="labelChange.emit($event)"
        />
      }
    </section>
  `,
  styles: `
    // Under the preview card, spaced as the players switch below is.
    :host {
      display: block;
      margin-top: var(--mr-space-4);
    }

    .panel {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);

      .mr-panel__title {
        margin: 0;
      }
    }
  `,
})
export class HitAudience {
  readonly visibility = input.required<HitAudienceValue>();
  readonly label = input.required<string>();
  /** The trigger is the marked target: the others read the mark's name on the target, not a label on the caster. */
  readonly marked = input(false);
  /** The messages of one input by its path: the server's refusals and the form's own. */
  readonly issuesOf = input<(path: string) => readonly string[]>(() => []);
  readonly visibilityChange = output<HitAudienceValue>();
  readonly labelChange = output<string>();

  protected readonly maxLabel = MAX_LABEL;
  protected readonly cards = computed(() => audienceCards(this.marked()));
  protected readonly labelHint = computed(
    () =>
      `${this.marked() ? 'No alvo marcado, na ordem.' : 'Em quem conjura.'} Nunca os dados. ${[...this.label()].length} de ${MAX_LABEL}`,
  );

  protected pathOf(field: string): string {
    return `table_spell.effect.${field}`;
  }
}
