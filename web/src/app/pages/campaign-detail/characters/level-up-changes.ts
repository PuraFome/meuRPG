import { Component, computed, input } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';

import type { LevelUp } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { choiceRows } from '../../../core/levelup/levelup-changes';
import { formatDateAt } from '../campaign-detail.copy';

/**
 * "O que mudou" for the master (MR-040, E8-15): what the player chose in one level-up, in words,
 * when they confirmed, and the link to the sheet. The line at the bottom says there is
 * nothing to approve: the rules checked every choice already, and the master can still edit the sheet.
 */
@Component({
  selector: 'app-level-up-changes',
  imports: [MatButtonModule, RouterLink],
  template: `
    <section class="changes" [attr.aria-label]="'O que ' + levelUp().characterName + ' escolheu no nível ' + levelUp().toLevel">
      <h4 class="changes__title">O que {{ levelUp().characterName }} escolheu no nível {{ levelUp().toLevel }}</h4>
      <dl class="changes__rows">
        @for (r of rows(); track r.label) {
          <div class="changes__row">
            <dt>{{ r.label }}</dt>
            <dd>{{ r.value }}</dd>
          </div>
        }
      </dl>
      <p class="changes__when">
        Confirmado em {{ when() }}. Nada fica à espera do seu OK: as regras já conferiram cada escolha.
      </p>
      <a matButton="outlined" [routerLink]="['/campanhas', campaignId(), 'personagens', levelUp().characterId]">Abrir a ficha</a>
    </section>
  `,
  styleUrl: './level-up-changes.scss',
})
export class LevelUpChanges {
  readonly levelUp = input.required<LevelUp>();
  readonly campaignId = input.required<string>();

  protected readonly rows = computed(() => choiceRows(this.levelUp()));
  protected readonly when = computed(() => {
    const at = this.levelUp().createdAt;
    return at ? formatDateAt(new Date(Number(at.seconds) * 1000)) : '';
  });
}
