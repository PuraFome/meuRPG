import { Component, computed, input, output } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { MatIconModule } from '@angular/material/icon';

import { Ability } from '../../../../gen/meurpg/rules/v1/rules_pb';
import type { CatalogVm } from '../../../core/content/catalog';
import {
  type CastingDraft,
  type CastingKind,
  type Preparation,
} from '../../../core/content/class-draft';
import { FieldNote } from '../../../shared/form-fields/field-note';
import { SelectField, type SelectOption } from '../../../shared/form-fields/select-field';
import { SwitchField } from '../../../shared/form-fields/switch-field';
import { TextField } from '../../../shared/form-fields/text-field';
import { Segmented, type Segment } from '../../../shared/segmented/segmented';

type KindChoice = 'none' | 'full' | 'half' | 'pact';

const KINDS: readonly Segment<KindChoice>[] = [
  { value: 'none', label: 'Nenhuma' },
  { value: 'full', label: 'Completa' },
  { value: 'half', label: 'Metade' },
  { value: 'pact', label: 'Pacto' },
];

/**
 * How a class, or a third caster's subclass, casts (E10-02 states 1 and 4): the kind (a class only), the ability, the list the
 * spells come from, prepared or known, and under "Mais opções" the level casting starts at, the formula of how many are
 * prepared and the rituals. It changes nothing by itself: the parent decides what a new kind does to the 20-level table (it
 * asks before it replaces edited rows) and gets `kindChange` and `patch`.
 */
@Component({
  selector: 'app-casting-fields',
  imports: [
    FieldNote,
    MatIconModule,
    NgTemplateOutlet,
    Segmented,
    SelectField,
    SwitchField,
    TextField,
  ],
  template: `
    <!-- The question about the table (the parent's) comes under the control that asked it: the kind for a class, the way of preparing for a third caster. -->
    <ng-template #question><ng-content /></ng-template>
    @if (!third()) {
      <div [attr.data-field]="basePath() + '.kind'">
        <app-segmented label="Conjuração" [segments]="kinds" [value]="kindChoice()" (choose)="kindChange.emit($event === 'none' ? '' : $event)" />
      </div>
      <app-field-note [issues]="issuesOf()(basePath() + '.kind')" />
      <ng-container [ngTemplateOutlet]="question" />
    }
    @if (casting().kind !== '') {
      <div class="row">
        <app-select-field
          label="Habilidade de conjuração"
          [options]="abilityOptions()"
          [value]="casting().ability === None ? '' : casting().ability"
          placeholder="Escolha…"
          [path]="basePath() + '.ability'"
          [issues]="issuesOf()(basePath() + '.ability')"
          (valueChange)="patch.emit({ ability: $event })"
        />
        <app-select-field
          label="Lista de magias"
          [options]="listOptions()"
          [value]="casting().listFrom"
          [placeholder]="third() ? 'Escolha…' : ''"
          [path]="basePath() + '.list_from'"
          [issues]="issuesOf()(basePath() + '.list_from')"
          [hint]="third() ? 'A classe de onde saem as magias desta subclasse.' : ''"
          (valueChange)="patch.emit({ listFrom: $event })"
        />
      </div>
      <fieldset class="prep" [attr.data-field]="basePath() + '.preparation'">
        <legend class="mr-visually-hidden">Como as magias são escolhidas</legend>
        <label class="prep__card" [class.prep__card--on]="casting().preparation === 'prepared'">
          <input type="radio" class="mr-visually-hidden" [name]="basePath() + '-prep'" value="prepared" [checked]="casting().preparation === 'prepared'" (change)="preparationChange.emit('prepared')" />
          <span class="prep__dot" aria-hidden="true"></span>
          <span class="prep__text">
            <strong>Preparadas</strong>
            <span>A pessoa prepara parte da lista; o número vem da habilidade e do nível, não da tabela.</span>
          </span>
        </label>
        <label class="prep__card" [class.prep__card--on]="casting().preparation === 'known'">
          <input type="radio" class="mr-visually-hidden" [name]="basePath() + '-prep'" value="known" [checked]="casting().preparation === 'known'" (change)="preparationChange.emit('known')" />
          <span class="prep__dot" aria-hidden="true"></span>
          <span class="prep__text">
            <strong>Conhecidas</strong>
            <span>A tabela dos níveis diz quantas magias a pessoa conhece.</span>
          </span>
        </label>
      </fieldset>
      <app-field-note [issues]="issuesOf()(basePath() + '.preparation')" />
      @if (third()) {
        <ng-container [ngTemplateOutlet]="question" />
      }
      <details class="more" [open]="moreOpen()">
        <summary><mat-icon aria-hidden="true">expand_more</mat-icon>Mais opções</summary>
        <div class="row">
          <app-text-field
            label="A conjuração começa no nível"
            inputmode="numeric"
            [value]="casting().startLevel > 0 ? '' + casting().startLevel : ''"
            [path]="basePath() + '.start_level'"
            [issues]="issuesOf()(basePath() + '.start_level')"
            hint="Antes dele, a tabela fica sem truques e sem espaços."
            (valueChange)="setStart($event)"
          />
          @if (casting().preparation === 'prepared') {
            <app-text-field
              label="Quantas magias prepara (fórmula)"
              [value]="casting().preparedMax"
              [path]="basePath() + '.prepared_max'"
              [issues]="issuesOf()(basePath() + '.prepared_max')"
              hint="Vazio: a habilidade mais o nível (a metade dele, para quem conjura pela metade), no mínimo 1."
              (valueChange)="patch.emit({ preparedMax: $event })"
            />
          }
        </div>
        <app-switch-field label="Conjura rituais" [checked]="casting().ritual" (toggled)="patch.emit({ ritual: $event })" />
      </details>
    }
    <app-field-note [issues]="issuesOf()(basePath())" />
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
      min-width: 0;
    }

    .row {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      gap: var(--mr-space-3);
      align-items: start;

      @media (min-width: 768px) {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }
    }

    .prep {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      gap: var(--mr-space-3);
      margin: 0;
      padding: 0;
      border: 0;

      @media (min-width: 768px) {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }
    }

    .prep__card {
      display: flex;
      align-items: flex-start;
      gap: var(--mr-space-3);
      min-height: 64px;
      box-sizing: border-box;
      padding: var(--mr-space-3) var(--mr-space-4);
      border: 1px solid var(--mr-control-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-surface);
      cursor: pointer;

      &--on {
        border: 2px solid var(--mr-accent);
        padding: calc(var(--mr-space-3) - 1px) calc(var(--mr-space-4) - 1px);
        background: var(--mr-accent-soft);
      }

      &:has(input:focus-visible) {
        outline: 2px solid var(--mr-focus);
        outline-offset: 2px;
      }
    }

    .prep__dot {
      flex: none;
      box-sizing: border-box;
      width: 24px;
      height: 24px;
      margin-top: 2px;
      border: 2px solid var(--mr-control-line);
      border-radius: 50%;
      background: var(--mr-surface);

      .prep__card--on & {
        border: 7px solid var(--mr-accent);
      }
    }

    .prep__text {
      display: flex;
      flex-direction: column;
      gap: 2px;
      font-size: 16px;
      color: var(--mr-ink-muted);

      strong {
        font-size: 17px;
        color: var(--mr-ink);
      }
    }

    .more {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);

      summary {
        min-height: 44px;
        display: flex;
        align-items: center;
        gap: 4px;
        cursor: pointer;
        font-weight: 700;
        color: var(--mr-accent-text);
        list-style: none;

        &::-webkit-details-marker {
          display: none;
        }
      }

      &[open] summary {
        margin-bottom: var(--mr-space-2);

        .mat-icon {
          transform: rotate(180deg);
        }
      }
    }
  `,
})
export class CastingFields {
  readonly casting = input.required<CastingDraft>();
  /** The subclass's: no kind to choose (it is always "um terço") and the list is required. */
  readonly third = input(false);
  readonly catalog = input.required<CatalogVm>();
  /** "table_class.casting" or "table_subclass.casting". */
  readonly basePath = input.required<string>();
  /** The class being edited: it never takes its own list from itself. */
  readonly selfKey = input('');
  readonly issuesOf = input<(path: string) => readonly string[]>(() => []);

  readonly kindChange = output<CastingKind>();
  readonly preparationChange = output<Preparation>();
  readonly patch = output<Partial<CastingDraft>>();

  protected readonly kinds = KINDS;
  protected readonly kindChoice = computed<KindChoice>(() => {
    const k = this.casting().kind;
    return k === 'full' || k === 'half' || k === 'pact' ? k : 'none';
  });
  protected readonly abilityOptions = computed<SelectOption<number>[]>(() =>
    this.catalog().abilities.map((a) => ({ value: a.ability as number, label: a.name })),
  );
  protected readonly listOptions = computed<SelectOption[]>(() => {
    const own: SelectOption[] = this.third()
      ? []
      : [{ value: '', label: 'A própria lista da classe' }];
    const classes = this.catalog()
      .castingClasses.filter((c) => c.key !== this.selfKey())
      .map((c) => ({
        value: c.key,
        label: c.table ? `A lista do ${c.name} (da mesa)` : `A lista do ${c.name}`,
      }));
    return [...own, ...classes];
  });
  /** "Mais opções" starts open when something in it was refused or is not the default. */
  protected readonly moreOpen = computed(
    () =>
      this.issuesOf()(this.basePath() + '.start_level').length +
        this.issuesOf()(this.basePath() + '.prepared_max').length >
      0,
  );

  protected setStart(text: string): void {
    const n = Number(text.trim());
    this.patch.emit({ startLevel: Number.isInteger(n) && n > 0 ? n : 0 });
  }

  protected readonly None = Ability.UNSPECIFIED as number;
}
