import {
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  input,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { actionSubtitle, actionTitle } from '../../../../core/maps/scene-actions';
import type { MapState } from '../../../../core/maps/map-state';
import { joinDots } from '../../../../core/combat/combat-grid';
import { SceneClient } from '../../../../core/play/scene-client';
import { sceneErrorMessage } from '../../../../core/play/scene-errors';
import type { SceneState } from '../../../../core/play/scene-state';
import { actionCount, rollCount } from '../../../../core/play/scene-view';
import { formatClock } from '../../../../shared/session-time/session-time';
import { PHONE_QUERY, mediaQuery } from '../../../../shared/map-view/media-query';
import { SceneRollLine } from '../scene-roll-line/scene-roll-line';
import { openScenePicker } from '../scene-picker/scene-picker';

/**
 * The scene that is open, as the master reads it (E7-02 state 3, E7-05): the
 * first block of the left column with a 1px accent frame (it is on the
 * players' screens right now), its title, "Trocar cena" and "Fechar cena",
 * the rolls (newest first, a live region says each new one) and the actions
 * with their DCs, which only the master sees. On a phone the rolls come first
 * and the actions are a section that opens and closes (a 48px button with
 * `aria-expanded`, open at the start); the two buttons are equal, 48px, and
 * stack when the width is short. Closing asks nothing.
 *
 * Focus: when the scene opens or is swapped here, the title takes it (the
 * state's `focusNext`); the page never moves focus for a scene that was
 * already open.
 */
@Component({
  selector: 'app-scene-open',
  imports: [MatButtonModule, MatIconModule, SceneRollLine],
  templateUrl: './scene-open.html',
  styleUrl: './scene-open.scss',
})
export class SceneOpen {
  private readonly api = inject(SceneClient);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  private readonly injector = inject(Injector);

  readonly campaignId = input.required<string>();
  readonly state = input.required<SceneState>();
  readonly mapState = input.required<MapState>();

  protected readonly phone = mediaQuery(PHONE_QUERY);
  protected readonly actionsOpen = signal(true);
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly title = actionTitle;
  protected readonly subtitle = actionSubtitle;

  private readonly heading = viewChild<ElementRef<HTMLElement>>('heading');

  protected readonly scene = computed(() => this.state().scene());
  protected readonly since = computed(() => {
    const at = this.scene()?.openedAt;
    return at ? `Aberta para os jogadores desde ${formatClock(timestampDate(at))}` : 'Aberta para os jogadores';
  });
  /** The phone's line under "Rolagens": how many, and the order (E7-05). */
  protected readonly rollsHint = computed(() => {
    const n = this.scene()?.rolls.length ?? 0;
    return joinDots([n === 0 ? 'Nenhuma rolagem' : rollCount(n), 'a mais nova em cima']);
  });
  protected readonly meta = computed(() => {
    const scene = this.scene();
    if (!scene) {
      return '';
    }
    const onMap = this.mapState().points().some((p) => p.id === scene.pointId);
    const mapName = this.mapState().map()?.name;
    return joinDots([
      ...(onMap && mapName ? [`Mapa ${mapName}`] : []),
      actionCount(scene.actions.length),
      rollCount(scene.rolls.length),
    ]);
  });

  constructor() {
    // The title takes focus once, after the master's own open or swap.
    effect(() => {
      if (this.state().focusNext() === 'title' && this.heading()) {
        const state = untracked(() => this.state());
        afterNextRender(
          () => {
            this.heading()?.nativeElement.focus();
            state.focusNext.set(null);
          },
          { injector: this.injector },
        );
      }
    });
  }

  protected swap(): void {
    const scene = this.scene();
    if (!scene) {
      return;
    }
    // The map may have changed in the editor since the page read it.
    void this.mapState().refresh();
    openScenePicker(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      mapName: this.mapState().map()?.name ?? '',
      points: this.mapState().points(),
      openPointId: scene.pointId,
      state: this.state(),
    }).subscribe();
  }

  protected async close(): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      await this.api.close(this.campaignId());
      this.state().closedHere();
    } catch (err) {
      this.error.set(sceneErrorMessage(err, 'fechar a cena'));
    } finally {
      this.busy.set(false);
    }
  }
}
