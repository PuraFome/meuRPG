import { Component, computed, effect, inject, input, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import { XPAwardMode } from '../../../../gen/meurpg/progression/v1/progression_pb';
import { formatXp, tight } from '../../../core/format/text';
import { ProgressionClient } from '../../../core/progression/progression-client';
import type { CharacterSheetVm } from '../character-sheet.types';

/**
 * "Pensantus pode subir de nível" (MR-040, RN-12): the block on the locked sheet of a character that
 * can level up, with why ("O mestre marcou “Chegar ao Vale Seco”." or "Você chegou a 2.700 XP.")
 * and the one filled button, "Subir para o nível N", which opens the guided level-up. Only the
 * owning player gets it: the master raises the level in the sheet editor, and an NPC has no level-up.
 * The milestone's name is the master's own words, read from the campaign's history; when it
 * cannot be read the block says "O mestre marcou um marco." and still works.
 */
@Component({
  selector: 'app-level-up-banner',
  imports: [MatButtonModule, MatIconModule, RouterLink],
  template: `
    @if (shown()) {
      <section class="mr-notice mr-notice--success banner" [attr.aria-label]="vm().name + ' pode subir de nível'">
        <mat-icon aria-hidden="true">arrow_upward</mat-icon>
        <div class="banner__body">
          <h2 class="banner__title">{{ vm().name }} pode subir de nível</h2>
          <p class="banner__why">{{ why() }}</p>
          <a matButton="filled" class="banner__go" [routerLink]="['/campanhas', vm().campaignId, 'personagens', vm().id, 'subir-de-nivel']">
            Subir para o nível {{ vm().totalLevel + 1 }}
          </a>
        </div>
      </section>
    }
  `,
  styleUrl: './level-up-banner.scss',
})
export class LevelUpBanner {
  private readonly progression = inject(ProgressionClient);

  readonly vm = input.required<CharacterSheetVm>();

  /** What the master wrote when it marked the milestone, once read. */
  private readonly milestone = signal('');

  protected readonly shown = computed(() => {
    const vm = this.vm();
    return vm.canLevelUp && !vm.isMaster && vm.characterKind === 'player' && vm.state === 'locked';
  });

  protected readonly why = computed(() => {
    const vm = this.vm();
    if (vm.levelUpReason === 'milestone') {
      const name = this.milestone();
      return `O mestre marcou ${name ? `“${name}”` : 'um marco'}. Você pode subir para o nível ${vm.totalLevel + 1}.`;
    }
    if (vm.levelUpReason === 'xp') {
      return tight(`Você chegou a ${formatXp(vm.nextLevelXp)}. Você pode subir para o nível ${vm.totalLevel + 1}.`);
    }
    return `Você pode subir para o nível ${vm.totalLevel + 1}.`;
  });

  /** What the milestone's name depends on: the effect below reads only this, so a re-read of the sheet
   * (the XP block, the stream) that changes nothing about it does not ask the history again. */
  private readonly milestoneKey = computed(() => {
    const vm = this.vm();
    return this.shown() && vm.levelUpReason === 'milestone' ? `${vm.campaignId}|${vm.id}` : '';
  });

  constructor() {
    // The milestone's name is the newest award of the mode that marks this character and was not undone.
    effect(() => {
      const key = this.milestoneKey();
      if (key === '') {
        return;
      }
      const [campaignId, id] = key.split('|');
      void this.progression
        .listAwards(campaignId)
        .then((res) => {
          const award = res.awards.find(
            (a) => a.mode === XPAwardMode.XP_AWARD_MODE_MILESTONE && !a.undone && a.shares.some((s) => s.characterId === id),
          );
          this.milestone.set(award?.reason ?? '');
        })
        .catch(() => this.milestone.set(''));
    });
  }
}
