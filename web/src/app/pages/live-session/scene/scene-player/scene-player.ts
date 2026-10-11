import {
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  DestroyRef,
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
import {
  ownRollOf,
  passLabel,
  playerAttempts,
  rollClock,
  sceneRollFormula,
  sceneRollMark,
  signedBonus,
} from '../../../../core/play/scene-view';
import { tieShortWords, tight } from '../../../../core/format/text';
import { StagePlayer } from '../stage-player/stage-player';
import { SceneRollSheet, openSceneRollSheet } from '../scene-roll-sheet/scene-roll-sheet';

/**
 * "Cena" on the player's page (E7-03, MR-015, RN-20): the block under the
 * header, in the same accent frame as "O mestre está mostrando". It shows the
 * point's description and each action as a row: its name (or the check's), the
 * check, the passive value where the server sends one ("Investigação passiva
 * 16"), the player's own bonus and "Rolar" (48px). After rolling, the row is
 * the result; an action rolls as many times as the master allows (1 by
 * default; MR-015, question 55) and says how many attempts are left ("Restam 2
 * de 3 tentativas", "Sem mais tentativas"). When the master shows the DC
 * (RN-20) the row has the "CD 12" pill and, after a roll, "Passou · CD 12" or
 * "Não passou · CD 10" (icon and words); otherwise there is no DC and no pass
 * or fail anywhere, because the server never sends them. "Rolar" sits in the
 * same place on every row, on the last line: at phone width the words stack
 * on top and the number is left, "Rolar" right.
 *
 * It comes live without moving focus (a live region elsewhere reads the
 * title), and "Rolar" opens the roll sheet; when that closes, focus returns to
 * the action's row.
 */
@Component({
  selector: 'app-scene-player',
  imports: [MatButtonModule, MatIconModule, StagePlayer],
  templateUrl: './scene-player.html',
  styleUrl: './scene-player.scss',
})
export class ScenePlayer {
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  /** The roll sheet this block opened is on screen (it is the page's overlay, not part of this block's DOM). */
  private rollSheetOpen = false;
  private gone = false;

  constructor() {
    // The block leaves the page when a combat shows (the page draws the combat instead). The roll sheet is an overlay
    // that would stay on top of the initiative card and keep "Rolar no app" out of reach: it closes with the block.
    inject(DestroyRef).onDestroy(() => {
      this.gone = true;
      if (this.rollSheetOpen) {
        // Only the roll sheet: another dialog on the page is not this block's to close.
        this.dialog.openDialogs
          .filter((d) => d.componentInstance instanceof SceneRollSheet)
          .forEach((d) => d.close());
        if (this.bottomSheet._openedBottomSheetRef?.instance instanceof SceneRollSheet) {
          this.bottomSheet.dismiss();
        }
      }
    });
  }

  readonly campaignId = input.required<string>();
  readonly state = input.required<SceneState>();
  readonly diceMode = input.required<DiceMode>();
  readonly dicePreference = input.required<DicePreference>();
  /** The skills Talento Confiável raises for the character (the sheet's), for the typed die's preview. */
  readonly reliableTalent = input<readonly string[]>([]);

  protected readonly scene = computed(() => this.state().scene());
  /** The scene's name with "A" tied to the next word (no break after it). */
  protected readonly sceneName = computed(() => tieShortWords(this.scene()?.name ?? ''));
  protected readonly bonusText = signedBonus;
  protected readonly formula = sceneRollFormula;
  protected readonly mark = sceneRollMark;

  /** The rows with what the screen needs of each. */
  protected readonly rows = computed(() => {
    const scene = this.scene();
    if (!scene) {
      return [];
    }
    return scene.actions.map((action) => {
      const own = ownRollOf(scene, action.id);
      const attempts = playerAttempts(action);
      const pass = own ? passLabel(scene, own) : null;
      return {
        action,
        title: actionTitle(action),
        check: actionSubtitle(action),
        passive: passiveLine(action),
        own,
        attempts,
        // The DC comes only when the master shows it; "Passou · CD 12" replaces the pill after a roll.
        pass: pass && own ? { text: pass, ok: own.passed === true } : null,
        dc: action.dc > 0 ? `CD\u00a0${action.dc}` : '',
        // Rolled out of attempts and no pill to say how it went: the word "Rolada".
        done: own && attempts?.out && !pass ? `Rolada às ${rollClock(own)}` : '',
        canRoll: action.bonus !== undefined && !attempts?.out,
      };
    });
  });

  protected roll(action: SceneActionView): void {
    this.rollSheetOpen = true;
    openSceneRollSheet(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      action,
      diceMode: this.diceMode(),
      preference: this.dicePreference(),
      state: this.state(),
      reliableTalent: this.reliableTalent(),
    }).subscribe(() => {
      this.rollSheetOpen = false;
      if (!this.gone) {
        this.focusRow(action.id);
      }
    });
  }

  /** The row that was rolled turns into the result; the sheet's opener is gone,
   * so focus is put on the row itself (tabindex -1). */
  private focusRow(actionId: string): void {
    afterNextRender(
      () =>
        this.host.nativeElement.querySelector<HTMLElement>(`[data-action="${actionId}"]`)?.focus(),
      { injector: this.injector },
    );
  }
}

/** "Investigação passiva 16", only where the server sent a passive value. */
function passiveLine(action: SceneActionView): string {
  return action.passive === undefined ? '' : tight(`${action.checkName} passiva ${action.passive}`);
}
