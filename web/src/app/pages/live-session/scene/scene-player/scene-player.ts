import {
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  input,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import type { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { SceneActionView } from '../../../../../gen/meurpg/play/v1/scene_pb';
import { actionSubtitle, actionTitle } from '../../../../core/maps/scene-actions';
import type { SceneState } from '../../../../core/play/scene-state';
import { ownRollOf, sceneRollFormula, signedBonus } from '../../../../core/play/scene-view';
import { tight } from '../../../../core/combat/combat-grid';
import { openSceneRollSheet } from '../scene-roll-sheet/scene-roll-sheet';

/**
 * "Cena" on the player's page (E7-03, MR-015, RN-20): the block under the
 * header, in the same accent frame as "O mestre está mostrando". It shows the
 * point's description and each action as a row: its name (or the check's), the
 * check, the passive value where the server sends one ("Investigação passiva
 * 16"), the player's own bonus and "Rolar" (48px). After rolling, the row is
 * the result and the word "Rolada" with a check; each action rolls once while
 * the scene is open. There is no DC and no pass or fail anywhere: the server
 * never sends them, and the master says what happens.
 *
 * It comes live without moving focus (a live region elsewhere reads the
 * title), and "Rolar" opens the roll sheet; when that closes, focus returns to
 * the action's row.
 */
@Component({
  selector: 'app-scene-player',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './scene-player.html',
  styleUrl: './scene-player.scss',
})
export class ScenePlayer {
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly campaignId = input.required<string>();
  readonly state = input.required<SceneState>();
  readonly diceMode = input.required<DiceMode>();
  readonly dicePreference = input.required<DicePreference>();

  protected readonly scene = computed(() => this.state().scene());
  protected readonly bonusText = signedBonus;
  protected readonly formula = sceneRollFormula;

  /** The rows with what the screen needs of each. */
  protected readonly rows = computed(() => {
    const scene = this.scene();
    if (!scene) {
      return [];
    }
    return scene.actions.map((action) => ({
      action,
      title: actionTitle(action),
      check: actionSubtitle(action),
      passive: passiveLine(action),
      own: ownRollOf(scene, action.id),
    }));
  });

  protected roll(action: SceneActionView): void {
    openSceneRollSheet(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      action,
      diceMode: this.diceMode(),
      preference: this.dicePreference(),
      state: this.state(),
    }).subscribe(() => this.focusRow(action.id));
  }

  /** The row that was rolled turns into the result; the sheet's opener is gone,
   * so focus is put on the row itself (tabindex -1). */
  private focusRow(actionId: string): void {
    afterNextRender(
      () =>
        this.host.nativeElement
          .querySelector<HTMLElement>(`[data-action="${actionId}"]`)
          ?.focus(),
      { injector: this.injector },
    );
  }
}

/** "Investigação passiva 16", only where the server sent a passive value. */
function passiveLine(action: SceneActionView): string {
  return action.passive === undefined ? '' : tight(`${action.checkName} passiva ${action.passive}`);
}
