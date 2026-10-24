import { Component, inject, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';

import { aidTagForMaster } from '../../../core/combat/combat-effects';
import { Nudger } from '../../../core/combat/nudger';
import { CombatantToken } from '../../../shared/combatant-token/combatant-token';
import { PartyMemberInfoVm, VitalsVm } from '../../../core/play/live-session.types';
import { SessionXp } from '../session-xp/session-xp';
import { initialOf } from '../../../core/play/scene-view';
import { SlotDots } from '../../../shared/slot-dots/slot-dots';
import { hitPointsPercent, hitPointsState, slotRowLabel } from '../../../core/play/vitals';

/**
 * The master's "Grupo" panel on the session page (C6-06): one row per living player character, in a line on a computer: the
 * token, the name with the class and a condition ("Barda 3 · Envenenada"), the hit points ("23 de 24 PV", a thin bar and a
 * word when they are low), the spell slots with how many are free ("1º ○○●● 2 de 4", "Sem espaços de magia"), "Ajustar" as a
 * text action (it opens the adjust sheet, RN-02) and a menu with "Chamar a atenção", which makes the character's player's
 * screen alert. "Dar XP" is in the panel title. No filled button on the tab.
 */
@Component({
  selector: 'app-party-panel',
  imports: [CombatantToken, MatButtonModule, MatIconModule, MatMenuModule, SessionXp, SlotDots],
  templateUrl: './party-panel.html',
  styleUrl: './party-panel.scss',
})
export class PartyPanel {
  readonly campaignId = input.required<string>();
  readonly campaignName = input('');
  readonly party = input.required<readonly VitalsVm[]>();
  readonly info = input<ReadonlyMap<string, PartyMemberInfoVm>>(new Map());

  /** The characters who see nothing where they stand (the master's light warning): a "No escuro" badge beside their names. */
  readonly dark = input<ReadonlySet<string>>(new Set());

  /** The conditions the effects in play put on each character ("Envenenada"), by character. */
  readonly conditions = input<ReadonlyMap<string, readonly string[]>>(new Map());

  /** "Ajustar" on a row. */
  readonly adjust = output<VitalsVm>();

  protected readonly aidTag = aidTagForMaster;
  protected readonly percent = hitPointsPercent;
  protected readonly hpState = hitPointsState;
  protected readonly slotRowLabel = slotRowLabel;
  protected readonly nudger = inject(Nudger);
  protected readonly initial = initialOf;

  protected conditionsOf(characterId: string): readonly string[] {
    return this.conditions().get(characterId) ?? [];
  }

  /** The tags under the bar: the temporary HP, what Ajuda adds, an armor an effect gives, a beast's shape and the low-HP word. */
  protected hasTags(v: VitalsVm): boolean {
    return (
      v.hitPointsTemporary > 0 ||
      (v.hitPointsMaxBonus ?? 0) > 0 ||
      (v.armorClassBase ?? 0) > 0 ||
      !!v.wildShape ||
      this.hpState(v) !== null
    );
  }

  protected call(v: VitalsVm): void {
    void this.nudger.call(this.campaignId(), v.characterId);
  }
}
