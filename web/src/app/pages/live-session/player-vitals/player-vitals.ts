import { Component, computed, effect, input, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import { PlayerSheetVm, VitalsVm } from '../live-session.types';
import { SlotDots } from '../slot-dots/slot-dots';
import { hitPointsPercent, slotLevelLabel, slotRowLabel, usedWords } from '../vitals';

interface SlotRowVm {
  readonly key: string;
  readonly label: string;
  readonly total: number;
  readonly used: number;
}

/**
 * The player's own vitals on the session page (RN-02, artboards E5-02 and
 * E5-03): the sheet's combat block with the current values filled in. The
 * PV box first (the number they came for), then the CA shield, temporary
 * HP and hit dice, then the spell slots, a link to the full sheet and who
 * changes these numbers. A player only ever sees their own character
 * (decision row 28): the server sends nothing else.
 */
@Component({
  selector: 'app-player-vitals',
  imports: [MatIconModule, RouterLink, SlotDots],
  templateUrl: './player-vitals.html',
  styleUrl: './player-vitals.scss',
})
export class PlayerVitals {
  readonly vitals = input.required<VitalsVm>();
  /** From the character's sheet; `null` until it loads. */
  readonly sheet = input<PlayerSheetVm | null>(null);
  readonly campaignId = input.required<string>();

  protected readonly usedWords = usedWords;
  protected readonly percent = computed(() => hitPointsPercent(this.vitals()));

  protected readonly slotRows = computed<SlotRowVm[]>(() => {
    const v = this.vitals();
    const rows: SlotRowVm[] = v.spellSlots.map((s) => ({
      key: `level-${s.level}`,
      label: slotLevelLabel(s.level),
      total: s.total,
      used: s.used,
    }));
    if (v.pactSlots) {
      rows.push({
        key: 'pact',
        label: `Pacto, ${slotLevelLabel(v.pactSlots.slotLevel)}`,
        total: v.pactSlots.total,
        used: v.pactSlots.used,
      });
    }
    return rows;
  });

  protected readonly slotRowLabel = slotRowLabel;

  /** Said once by screen readers when the master changes a number: the
   * screen changes under the player's eyes, and this says what changed. */
  protected readonly announcement = signal('');
  private previous: VitalsVm | null = null;

  constructor() {
    effect(() => {
      const v = this.vitals();
      const before = this.previous;
      this.previous = v;
      if (!before || before.characterId !== v.characterId || before.revision === v.revision) {
        return;
      }
      this.announcement.set(
        `O mestre ajustou: ${v.hitPointsCurrent} de ${v.hitPointsMax} pontos de vida, ` +
          `${v.hitPointsTemporary} temporários.`,
      );
    });
  }
}
