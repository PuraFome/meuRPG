import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { OpenSceneInfo, SceneRoll } from '../../../../../gen/meurpg/play/v1/scene_pb';
import { joinDots } from '../../../../core/format/text';
import {
  initialOf,
  passLabel,
  rollActionTitle,
  rollClock,
  sceneRollFormula,
} from '../../../../core/play/scene-view';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';

/**
 * One roll of the scene log, as the master reads it (E7-02, E7-05): who and
 * when, which action (with "dado físico" when the player typed a real die),
 * the formula with its total, and, only when the action had a DC, a pill on
 * its own line under the formula: "Passou · CD 12" with a check, "Não passou
 * · CD 13" with a cross (the words always, never colour alone).
 */
@Component({
  selector: 'app-scene-roll-line',
  imports: [CombatantToken, MatIconModule],
  template: `
    <app-combatant-token [initial]="initial()" [size]="32" />
    <div class="rl__body">
      <div class="rl__top">
        <b class="rl__who">{{ roll().characterName }}</b>
        <time class="rl__time">{{ clock() }}</time>
      </div>
      <p class="rl__action">{{ action() }}</p>
      <p class="rl__formula">{{ formula() }}</p>
      @if (pass(); as text) {
        <span class="mr-tag" [class.mr-tag--success]="roll().passed" [class.mr-tag--danger]="!roll().passed">
          <mat-icon aria-hidden="true">{{ roll().passed ? 'check' : 'close' }}</mat-icon>{{ text }}
        </span>
      }
    </div>
  `,
  styleUrl: './scene-roll-line.scss',
  host: { class: 'rl' },
})
export class SceneRollLine {
  readonly scene = input.required<OpenSceneInfo>();
  readonly roll = input.required<SceneRoll>();

  protected readonly initial = computed(() => initialOf(this.roll().characterName));
  protected readonly clock = computed(() => rollClock(this.roll()));
  protected readonly action = computed(() => {
    const title = rollActionTitle(this.scene(), this.roll());
    return this.roll().roll?.physical ? joinDots([title, 'dado físico']) : title;
  });
  protected readonly formula = computed(() => sceneRollFormula(this.roll()));
  protected readonly pass = computed(() => passLabel(this.scene(), this.roll()));
}
