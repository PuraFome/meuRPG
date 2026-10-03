import {
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { SceneAction } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { sceneActionErrorMessage } from '../../../core/maps/map-errors';
import { MapsClient } from '../../../core/maps/maps-client';
import {
  SCENE_ACTION_LIMIT,
  actionSubtitle,
  actionTitle,
} from '../../../core/maps/scene-actions';
import { SceneActionForm, type NewSceneAction } from './scene-action-form';

/**
 * "Ações da cena" in the point panel of a SCENE point (E7-01, MR-015): what
 * the players may roll in the scene, one to twenty checks. Each change saves
 * at once (add, ↑, ↓, remove: no "save all", no question before removing), so
 * the panel's "Salvar ponto" never carries them. One write at a time: while
 * one is in flight, the other buttons answer nothing (`aria-disabled`, not
 * `disabled`, so focus stays where it was).
 *
 * Focus: after ↑ or ↓ it stays on the same button of the moved row; after a
 * removal it goes to the next row's "Remover", or to "Adicionar ação" when
 * there is none; "Adicionar ação" opens the form with focus on its first
 * choice, and returns there when the form closes.
 */
@Component({
  selector: 'app-scene-actions',
  imports: [MatButtonModule, MatIconModule, SceneActionForm],
  templateUrl: './scene-actions.html',
  styleUrl: './scene-actions.scss',
})
export class SceneActions {
  private readonly api = inject(MapsClient);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly campaignId = input.required<string>();
  readonly mapId = input.required<string>();
  readonly pointId = input.required<string>();
  readonly actions = input.required<readonly SceneAction[]>();
  /** The point's actions as the server has them now (after each write). */
  readonly actionsChange = output<readonly SceneAction[]>();

  protected readonly limit = SCENE_ACTION_LIMIT;
  protected readonly title = actionTitle;
  protected readonly subtitle = actionSubtitle;
  protected readonly adding = signal(false);
  protected readonly busy = signal(false);
  /** A refusal of the add form, shown inside it. */
  protected readonly formError = signal('');
  /** A refusal of a move or a removal. */
  protected readonly error = signal('');
  /** What a screen reader hears after a change. */
  protected readonly status = signal('');
  protected readonly full = computed(() => this.actions().length >= SCENE_ACTION_LIMIT);

  private readonly addButton = viewChild('addButton', { read: ElementRef<HTMLButtonElement> });

  protected openForm(): void {
    if (this.full()) {
      return;
    }
    this.formError.set('');
    this.adding.set(true);
  }

  protected closeForm(): void {
    this.adding.set(false);
    this.formError.set('');
    this.focusAdd();
  }

  protected async add(action: NewSceneAction): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.formError.set('');
    try {
      const actions = await this.api.addSceneAction(
        this.campaignId(),
        this.mapId(),
        this.pointId(),
        action,
      );
      this.actionsChange.emit(actions);
      this.adding.set(false);
      this.status.set(`Ação adicionada. ${actions.length} de ${SCENE_ACTION_LIMIT}.`);
      this.focusAdd();
    } catch (err) {
      this.formError.set(sceneActionErrorMessage(err, 'adicionar a ação'));
    } finally {
      this.busy.set(false);
    }
  }

  protected async move(action: SceneAction, direction: 'up' | 'down'): Promise<void> {
    const index = this.actions().findIndex((a) => a.id === action.id);
    const edge = direction === 'up' ? index === 0 : index === this.actions().length - 1;
    if (this.busy() || edge) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const actions = await this.api.moveSceneAction(
        this.campaignId(),
        this.mapId(),
        this.pointId(),
        action.id,
        direction,
      );
      this.actionsChange.emit(actions);
      const place = actions.findIndex((a) => a.id === action.id) + 1;
      this.status.set(`${actionTitle(action)}: posição ${place} de ${actions.length}.`);
      // The moved row keeps the same button, as soon as it is drawn.
      this.focusRow(action.id, direction === 'up' ? 'up' : 'down');
    } catch (err) {
      this.error.set(sceneActionErrorMessage(err, 'mover a ação'));
    } finally {
      this.busy.set(false);
    }
  }

  protected async remove(action: SceneAction): Promise<void> {
    if (this.busy()) {
      return;
    }
    const before = this.actions();
    const index = before.findIndex((a) => a.id === action.id);
    this.busy.set(true);
    this.error.set('');
    try {
      const actions = await this.api.removeSceneAction(
        this.campaignId(),
        this.mapId(),
        this.pointId(),
        action.id,
      );
      this.actionsChange.emit(actions);
      this.status.set(`Ação removida: ${actionTitle(action)}.`);
      const next = actions[index];
      if (next) {
        this.focusRow(next.id, 'remove');
      } else {
        this.focusAdd();
      }
    } catch (err) {
      this.error.set(sceneActionErrorMessage(err, 'remover a ação'));
    } finally {
      this.busy.set(false);
    }
  }

  private focusRow(actionId: string, control: 'up' | 'down' | 'remove'): void {
    afterNextRender(
      () =>
        this.host.nativeElement
          .querySelector<HTMLElement>(`[data-action="${actionId}"][data-control="${control}"]`)
          ?.focus(),
      { injector: this.injector },
    );
  }

  private focusAdd(): void {
    afterNextRender(() => this.addButton()?.nativeElement.focus(), { injector: this.injector });
  }
}
