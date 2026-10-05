import { Component, DestroyRef, inject, input, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { MapToken } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { litToast } from '../../../core/maps/carried-light';
import type { FogView } from '../../../core/maps/fog-view';
import type { LightOption } from '../../../core/maps/light-presets';
import type { MapState } from '../../../core/maps/map-state';
import { CarriedLight } from '../carried-light/carried-light';
import { FamiliarRow } from './familiar-row';

/**
 * What a player does with the fog, under the map (E9-04), in the session and in the combat: the familiar's
 * row ("Ver pelos olhos") and "Luz que você carrega", with the confirmation of a light. The confirmation
 * is a toast over the page, six seconds, read once (`role="status"`): it never pushes the map down and
 * pulls it back.
 */
@Component({
  selector: 'app-fog-player-tools',
  imports: [CarriedLight, FamiliarRow, MatIconModule],
  template: `
    @if (showFamiliar()) {
      <app-familiar-row [campaignId]="campaignId()" [characterId]="characterId()" [characterName]="characterName()" [reload]="creaturesTick()" [seeing]="seeing()" />
    }
    @if (own(); as me) {
      <app-carried-light [campaignId]="campaignId()" [mapId]="mapId()" [token]="me" (changed)="lightChanged($event)" />
    }
    @if (toast()) {
      <p class="toast" role="status"><mat-icon aria-hidden="true">lightbulb</mat-icon><span>{{ toast() }}</span></p>
    }
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
    }

    // Over the page, at the bottom: it takes no room in the layout.
    .toast {
      position: fixed;
      right: var(--mr-space-4);
      bottom: var(--mr-space-4);
      left: var(--mr-space-4);
      z-index: 1000;
      display: flex;
      align-items: flex-start;
      gap: var(--mr-space-3);
      max-width: 520px;
      margin: 0 auto;
      padding: 12px 18px;
      border: 1px solid var(--mr-success-ink);
      border-radius: var(--mr-radius-md);
      background: var(--mr-success-surface);
      color: var(--mr-success-ink);
      box-shadow: 0 6px 24px rgb(0 0 0 / 25%);
      font-size: 16px;
      line-height: 22px;

      .mat-icon {
        flex: none;
        width: 20px;
        height: 20px;
        margin-top: 1px;
        font-size: 20px;
      }
    }
  `,
})
export class FogPlayerTools {
  readonly campaignId = input.required<string>();
  readonly state = input.required<MapState>();
  readonly fog = input<FogView | null>(null);
  readonly characterId = input('');
  readonly characterName = input('');
  /** The player's own token on this map, whose light is carried. */
  readonly own = input<MapToken | null>(null);
  readonly mapId = input.required<string>();
  /** The familiar's row is for the exploration; in a combat the action list has "Ver pelos olhos do Nanquim". */
  readonly showFamiliar = input(true);
  readonly seeing = input(false);
  readonly creaturesTick = input(0);

  protected readonly toast = signal('');
  private timer: ReturnType<typeof setTimeout> | undefined;

  constructor() {
    inject(DestroyRef).onDestroy(() => clearTimeout(this.timer));
  }

  protected lightChanged(change: { token: MapToken; option: LightOption | null }): void {
    this.state().upsertToken(change.token);
    this.toast.set(litToast(change.option));
    clearTimeout(this.timer);
    this.timer = setTimeout(() => this.toast.set(''), 6000);
    void this.fog()?.refresh();
  }
}
