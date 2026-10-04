import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { characterKindLabel } from '../../../core/characters/character-labels';
import { LevelUpTag } from '../../../shared/xp/level-up-tag';
import { CampaignXpMode, CharacterSheetVm } from '../character-sheet.types';
import { XpBlock } from '../xp-block/xp-block';
import { formatXp } from '../../../core/format/text';
import { formatDate, stateTagLabel } from '../sheet-format';

interface HeaderField {
  readonly label: string;
  readonly value: string;
}

/**
 * The top of the sheet, as on the paper sheet (docs/design.md): the name,
 * the state tags, the lock or death line, and the identity fields as a
 * `<dl>` (value above a rule, label below; a field with no value is left
 * out). The page projects the viewer's actions into the row at the bottom.
 */
@Component({
  selector: 'app-sheet-header',
  imports: [LevelUpTag, MatIconModule, XpBlock],
  templateUrl: './sheet-header.html',
  styleUrl: './sheet-header.scss',
})
export class SheetHeader {
  readonly vm = input.required<CharacterSheetVm>();
  /** How the campaign levels (RN-09), once known: a milestones campaign has no XP block. */
  readonly xpMode = input<CampaignXpMode | null>(null);

  protected readonly isNpc = computed(() => this.vm().characterKind !== 'player');
  protected readonly kindLabel = computed(() => characterKindLabel(this.vm().characterKind));
  protected readonly stateLabel = computed(() => stateTagLabel(this.vm().state));
  /** An NPC never leaves "Rascunho" (only player sheets lock, RN-01, and an
   * NPC isn't marked dead, RN-04), so its tag is its kind alone. */
  protected readonly showState = computed(() => !this.isNpc() || this.vm().state !== 'draft');

  protected readonly lockedOn = computed(() => {
    const vm = this.vm();
    return vm.sheetLockedAt && vm.state !== 'dead' ? formatDate(vm.sheetLockedAt) : null;
  });
  protected readonly diedOn = computed(() => {
    const vm = this.vm();
    return vm.state === 'dead' && vm.diedAt ? formatDate(vm.diedAt) : null;
  });

  /** The XP block of a player character in a campaign that counts XP (E7-10). */
  protected readonly xp = computed(() => {
    const vm = this.vm();
    const mode = this.xpMode();
    if (this.isNpc() || vm.experiencePoints === null || mode === null || mode === 'milestones') {
      return null;
    }
    return {
      xp: vm.experiencePoints,
      nextLevelXp: vm.nextLevelXp,
      level: vm.totalLevel,
      canLevelUp: vm.canLevelUp,
    };
  });
  /** The owner of a locked sheet levels up with the block under the header (MR-040), which already says so:
   * the tag and the XP block's own tag would say it twice. */
  protected readonly selfLevelUp = computed(() => {
    const vm = this.vm();
    return vm.canLevelUp && !vm.isMaster && vm.state === 'locked' && !this.isNpc();
  });
  /** "Pode subir de nível" beside the state tags, where there is no XP block to carry it. */
  protected readonly milestoneTag = computed(
    () => this.xpMode() === 'milestones' && this.vm().canLevelUp && !this.isNpc() && !this.selfLevelUp(),
  );

  protected readonly fields = computed<HeaderField[]>(() => {
    const vm = this.vm();
    const fields: HeaderField[] = [
      { label: 'Classe e nível', value: vm.classSummary },
      { label: 'Raça', value: vm.raceLabel },
      { label: 'Antecedente', value: vm.backgroundLabel },
    ];
    if (!this.isNpc()) {
      fields.push({ label: 'Jogador', value: vm.playerDisplayName ?? 'Jogador sem nome' });
    }
    fields.push({ label: 'Tendência', value: vm.alignmentLabel });
    // What an enemy, a boss or a minion is worth when defeated: the master's
    // (RN-20), read here; the editor changes it (E7-11).
    if (this.isNpc() && vm.isMaster && vm.characterKind !== 'story') {
      if (vm.challengeRating !== '') {
        fields.push({ label: 'Nível de desafio', value: `ND ${vm.challengeRating}` });
      }
      if (vm.challengeRating !== '' || vm.xpValue > 0) {
        fields.push({ label: 'XP ao derrotar', value: formatXp(vm.xpValue) });
      }
    }
    return fields.filter((f) => f.value !== '');
  });
}
