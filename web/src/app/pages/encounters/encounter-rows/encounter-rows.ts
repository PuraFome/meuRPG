import { Component, input, output } from '@angular/core';
import { RouterLink } from '@angular/router';

import type { CreatureSummary } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { creatureSlug } from '../../../core/creatures/bestiary-format';
import { CountStepper } from '../../../shared/count-stepper/count-stepper';
import { CreatureArt } from '../../../shared/creatures/creature-art';

/** One line of the builder: the creature, how many, and what the server measured for it. */
export interface EncounterRow {
  readonly creature: CreatureSummary;
  readonly count: number;
  /** "450 XP cada". */
  readonly each: string;
  /** The server's subtotal, or "…" while a newer measure is on its way. */
  readonly subtotal: string;
  readonly aboveCap: boolean;
}

/**
 * The creatures of the encounter (E10-09 state 1): the picture, the name (a link to its stat block), the ND, the count stepper (the
 * "−" at 1 takes the creature out), "450 XP cada" and the subtotal. Narrow phones give the stepper a row of its own, so its buttons keep
 * 44 px and nothing touches the XP.
 */
@Component({
  selector: 'app-encounter-rows',
  imports: [CountStepper, CreatureArt, RouterLink],
  template: `
            <ul class="rows">
                @for (r of rows(); track r.creature.key) {
                  <li class="row">
                    <app-creature-art class="row__art" [monsterKey]="r.creature.key" [type]="r.creature.type" />
                    <span class="row__name">
                      <a class="row__pt" [routerLink]="['/campanhas', campaignId(), 'bestiario', slug(r.creature.key)]">{{ r.creature.namePt }}</a>
                      <span class="row__en"><span lang="en">{{ r.creature.name }}</span> · SRD</span>
                    </span>
                    <span class="row__nd">
                      ND {{ r.creature.challengeRating }}
                      @if (r.aboveCap) {
                        <span class="row__cap">Acima do ND máximo</span>
                      }
                    </span>
                    <app-count-stepper
                      class="row__step"
                      [value]="r.count"
                      [min]="0"
                      [max]="max()"
                      [minAt]="1"
                      [noun]="r.creature.namePt"
                      [minLabel]="'Tirar ' + r.creature.namePt + ' do encontro'"
                      (valueChange)="count.emit({ key: r.creature.key, value: $event })"
                    />
                    <span class="row__each">{{ r.each }}</span>
                    <span class="row__sub">{{ r.subtotal }}</span>
                  </li>
                }
              </ul>
  `,
  styleUrl: './encounter-rows.scss',
})
export class EncounterRows {
  readonly rows = input.required<readonly EncounterRow[]>();
  readonly campaignId = input.required<string>();
  readonly max = input(40);
  readonly count = output<{ key: string; value: number }>();
  protected readonly slug = creatureSlug;
}
