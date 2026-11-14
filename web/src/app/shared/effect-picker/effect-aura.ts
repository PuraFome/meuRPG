import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';

import { type CatalogVm } from '../../core/content/catalog';
import { type SpellEffectDraft } from '../../core/content/spell-effect-draft';
import { FEATURE_AURA_CONTEXT, SpellEffect } from '../spell-effect/spell-effect';

/** The block of a feature's aura (#579): the spell's effect block, as the feature has it, with the lists of the table's catalog. */
@Component({
  selector: 'app-effect-aura',
  imports: [SpellEffect],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-spell-effect
      [feature]="true"
      [effect]="effect()"
      [context]="context"
      [basePath]="basePath()"
      [skills]="catalog()?.skills ?? []"
      [damageTypes]="catalog()?.damageTypes ?? []"
      [abilityNames]="abilityNames()"
      [nameOf]="catalog()?.nameOf ?? identity"
      [issuesOf]="issuesOf()"
      (effectChange)="effectChange.emit($event)"
    />
  `,
})
export class EffectAura {
  readonly effect = input.required<SpellEffectDraft>();
  readonly catalog = input<CatalogVm | null>(null);
  readonly basePath = input.required<string>();
  readonly issuesOf = input<(path: string) => readonly string[]>(() => []);
  readonly effectChange = output<SpellEffectDraft>();

  protected readonly context = FEATURE_AURA_CONTEXT;
  protected readonly identity = (key: string): string => key;
  protected readonly abilityNames = computed(
    () => this.catalog()?.abilities.map((a) => a.name) ?? [],
  );
}
