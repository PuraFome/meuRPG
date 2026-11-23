import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { CreatureSummary } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { NpcCheckSource } from '../../../../gen/meurpg/play/v1/scene_pb';
import type { PickGroup, PickRow } from '../../../core/npc-check/npc-check-picker';
import { CreaturePick } from '../../../shared/creature-pick/creature-pick';

/**
 * "Quem rola" of the sheet "Teste de um NPC" (591-01): radio rows in the groups the server lists ("Em cena", "No mapa") and the
 * bestiary's search ("Do bestiário"), which ends in one creature the server reads the checks of. Each row says what the players see
 * of the creature and, for an NPC typed by hand, that its checks roll with +0. Real radios, so the arrow keys move the choice.
 */
@Component({
  selector: 'app-npc-check-who',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CreaturePick, MatIconModule],
  templateUrl: './npc-check-who.html',
  styleUrl: './npc-check-who.scss',
})
export class NpcCheckWho {
  readonly campaignId = input.required<string>();
  readonly groups = input.required<readonly PickGroup[]>();
  readonly selected = input('');
  readonly choose = output<PickRow>();
  readonly searched = output<CreatureSummary>();
  protected readonly bestiary = NpcCheckSource.MONSTER;
}
