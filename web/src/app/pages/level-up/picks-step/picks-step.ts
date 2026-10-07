import { Component, computed, input } from '@angular/core';

import type { PickItem } from '../../../core/levelup/levelup-flow';
import { PickList } from '../pick-list/pick-list';
import { LevelUpSession } from '../level-up-session';

/**
 * Step "Escolhas" of the guided level-up (MR-040): what a level offers that is neither
 * an ability nor a spell, in the order the rules need it. The subclass comes first, since
 * it can add cantrips, skills and feature options to the level; then each feature's options
 * (a fighting style, a metamagic), the new skills and the expertise. It exists only for a
 * class and level that offer one of them (the step list drops it otherwise). The artboards
 * of E8-15 do not draw it: it is the same picker as the Magias step, one panel per choice.
 */
@Component({
  selector: 'app-picks-step',
  imports: [PickList],
  template: `
    @if (s().options.subclassDue) {
      <app-pick-list
        pickId="subclass"
        title="Subclasse"
        lead="Escolha a subclasse que o personagem segue a partir deste nível."
        noun="subclasse"
        nounMany="subclasses"
        [items]="subclasses()"
        [picked]="subclassPicked()"
        [count]="1"
        (pick)="s().draft.setSubclass($event)"
      />
    }
    @for (g of groups(); track g.id) {
      <app-pick-list
        [pickId]="g.id"
        [title]="g.title"
        [lead]="g.lead"
        noun="opção"
        nounMany="opções"
        [items]="g.items"
        [picked]="g.picked"
        [count]="g.choose"
        [fullNote]="'Limite de ' + g.choose + ' opções'"
        (pick)="s().draft.toggleFeature($event, g.keys, g.choose)"
      />
    }
    @if (s().draft.skillsAsked() > 0) {
      <app-pick-list
        pickId="skills"
        [title]="s().draft.skillsAsked() === 1 ? 'Perícia nova' : 'Perícias novas'"
        [lead]="'Escolha ' + s().draft.skillsAsked() + ' ' + (s().draft.skillsAsked() === 1 ? 'perícia' : 'perícias') + ' em que o personagem ainda não é treinado.'"
        noun="perícia"
        nounMany="perícias"
        [items]="s().draft.skillItems()"
        [picked]="s().draft.skills()"
        [count]="s().draft.skillsAsked()"
        [fullNote]="'Limite de ' + s().draft.skillsAsked() + ' perícias novas'"
        (pick)="s().draft.toggleSkill($event)"
      />
    }
    @if (s().draft.expertiseAsked() > 0) {
      <app-pick-list
        pickId="expertise"
        title="Especialização"
        [lead]="'Escolha ' + s().draft.expertiseAsked() + ' ' + (s().draft.expertiseAsked() === 1 ? 'perícia' : 'perícias') + ' em que o personagem já é treinado, para dobrar o bônus de proficiência.'"
        noun="perícia"
        nounMany="perícias"
        [items]="s().draft.expertiseItems()"
        [picked]="s().draft.expertise()"
        [count]="s().draft.expertiseAsked()"
        [fullNote]="'Limite de ' + s().draft.expertiseAsked() + ' especializações'"
        (pick)="s().draft.toggleExpertise($event)"
      />
    }
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-4);
    }
  `,
})
export class PicksStep {
  readonly s = input.required<LevelUpSession>();

  protected readonly subclasses = computed<PickItem[]>(() =>
    this.s().options.subclasses.map((c) => ({ key: c.key, name: c.namePt, sub: '' })),
  );
  protected readonly subclassPicked = computed(
    () => new Set(this.s().draft.subclassKey() ? [this.s().draft.subclassKey()] : []),
  );

  /** One group per feature that offers options, with the picks that belong to it. */
  protected readonly groups = computed(() => {
    const d = this.s().draft;
    const chosen = d.features();
    return d.totals().featureChoices.map((f, i) => {
      const items = f.options.map((o) => ({ key: o.key, name: o.namePt, sub: '' }));
      const keys = items.map((o) => o.key);
      return {
        id: `feature-${i}`,
        title: f.feature?.namePt ?? 'Característica',
        lead: f.choose === 1 ? 'Escolha 1 opção.' : `Escolha ${f.choose} opções.`,
        items,
        keys,
        choose: f.choose,
        picked: new Set(keys.filter((k) => chosen.has(k))),
      };
    });
  });
}
