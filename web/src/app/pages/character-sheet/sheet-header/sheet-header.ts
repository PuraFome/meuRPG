import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { characterKindLabel } from '../../../core/characters/character-labels';
import { CharacterSheetVm } from '../character-sheet.types';
import { formatDate, formatXp, stateTagLabel } from '../sheet-format';

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
  imports: [MatIconModule],
  templateUrl: './sheet-header.html',
  styleUrl: './sheet-header.scss',
})
export class SheetHeader {
  readonly vm = input.required<CharacterSheetVm>();

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
    if (vm.experiencePoints !== null) {
      fields.push({ label: 'Experiência', value: formatXp(vm.experiencePoints) });
    }
    fields.push({ label: 'Tendência', value: vm.alignmentLabel });
    return fields.filter((f) => f.value !== '');
  });
}
