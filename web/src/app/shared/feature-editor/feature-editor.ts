import { Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { EffectDraft, EffectMenuVm } from '../../core/content/effect-draft';
import { emptyEffect } from '../../core/content/effect-draft';
import type { FeatureDraft } from '../../core/content/feature-draft';
import { EffectPicker } from '../effect-picker/effect-picker';
import { FieldNote } from '../form-fields/field-note';
import { TextField } from '../form-fields/text-field';
import type { SelectOption } from '../form-fields/select-field';

/**
 * One feature of a race, a subrace, a background (and, in 10.12, a class or a subclass): a name, a text and effects from the
 * server's menu (`app-effect-picker`). With no effect it is text only. The first effect is chosen in the "Efeito" select
 * ("Só texto" takes it away); a feature may have as many as the menu says (`maxEffects`). `basePath` is the feature's own path
 * ("table_race.traits[1]"): its inputs carry it, so a refusal lands on them.
 */
@Component({
  selector: 'app-feature-editor',
  imports: [EffectPicker, FieldNote, MatButtonModule, MatIconModule, TextField],
  templateUrl: './feature-editor.html',
  styleUrl: './feature-editor.scss',
})
export class FeatureEditor {
  readonly feature = input.required<FeatureDraft>();
  readonly menu = input.required<EffectMenuVm>();
  readonly basePath = input.required<string>();
  readonly issuesOf = input<(path: string) => readonly string[]>(() => []);
  readonly spellOptions = input<readonly SelectOption[]>([]);
  readonly nameLabel = input('Nome do traço');
  readonly textLabel = input('Texto');
  /** "Remover traço": absent for a feature that must exist (a background's). */
  readonly removeLabel = input('');
  /** The place in a list, for "Subir" and "Descer": -1 when it is not in a list. */
  readonly index = input(-1);
  readonly count = input(0);
  /** The heading above the fields, when the feature has one of its own ("Característica"). */
  readonly heading = input('');
  /** No card of its own (no border, fill or padding): the page already frames it, as the open feature of a class does. */
  readonly flat = input(false);
  /** A narrow field projected between the name and the effect (`<… lead>`): the class editor's "Nível". */
  readonly withLead = input(false);

  readonly featureChange = output<FeatureDraft>();
  readonly removed = output<void>();
  readonly moved = output<-1 | 1>();

  protected readonly canAddEffect = computed(
    () => this.feature().effects.length > 0 && this.feature().effects.length < this.menu().maxEffects,
  );
  /** The effects after the first, with their place in the list (the first sits beside the name). */
  protected readonly otherEffects = computed(() => this.feature().effects.map((effect, index) => ({ id: effect.id, effect, index })).slice(1));
  /** With no effect, the picker shows "Só texto". */
  protected readonly firstEffect = computed<EffectDraft>(() => this.feature().effects[0] ?? emptyEffect(''));

  /** The group's own name, never the name field's label (two things with one name confuse a screen reader and a test). */
  protected readonly groupLabel = computed(() => this.heading() || (this.index() >= 0 ? `Traço ${this.index() + 1}` : 'Característica'));

  protected path(field: string): string {
    return `${this.basePath()}.${field}`;
  }

  protected patch(p: Partial<FeatureDraft>): void {
    this.featureChange.emit({ ...this.feature(), ...p });
  }

  protected setEffect(i: number, e: EffectDraft): void {
    const effects = [...this.feature().effects];
    if (e.type === '') {
      // "Só texto" takes this effect away; the others stay.
      this.patch({ effects: effects.filter((_, k) => k !== i) });
      return;
    }
    effects[i] = e;
    this.patch({ effects });
  }

  protected addEffect(): void {
    this.patch({ effects: [...this.feature().effects, emptyEffect('note')] });
  }

  protected removeEffect(i: number): void {
    this.patch({ effects: this.feature().effects.filter((_, k) => k !== i) });
  }
}
