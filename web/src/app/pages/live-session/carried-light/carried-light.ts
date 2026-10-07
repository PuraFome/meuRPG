import {
  Component,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import type { MapToken } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { carriedName, carriedOptions } from '../../../core/maps/carried-light';
import { type LightOption, LightPresets } from '../../../core/maps/light-presets';
import { openCarriedLight } from './carried-light-sheet';

/**
 * "Luz que você carrega: Nenhuma · Mudar" (MR-036, E9-04 state 1): the player's
 * own row under the fog map, 56 px high and one tap target, opening the sheet
 * where the light is chosen. The row says what the character carries now (from
 * its token, which only the character's own player and the master get); each
 * choice in the sheet is applied at once and reported here (`changed`) so the page
 * can say it and read the map again. The presets are read once and kept.
 */
@Component({
  selector: 'app-carried-light',
  imports: [MatIconModule],
  template: `
    <button type="button" class="row" [disabled]="!ready()" (click)="open()">
      <mat-icon class="row__icon" aria-hidden="true">lightbulb</mat-icon>
      <span class="row__text">
        <span class="row__label">Luz que você carrega</span>
        <span class="row__value">{{ name() }}</span>
      </span>
      <span class="row__action">Mudar</span>
    </button>
    @if (failed()) {
      <p class="row__fail" role="alert">Não deu para ler as luzes. Recarregue a página.</p>
    }
  `,
  styleUrl: './carried-light.scss',
})
export class CarriedLight {
  private readonly presets = inject(LightPresets);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);

  readonly campaignId = input.required<string>();
  readonly mapId = input.required<string>();
  /** The viewer's own token: its `carried_light` is what is carried now. */
  readonly token = input.required<MapToken>();

  /** A light was set: the new token, and the option (`null` for none). */
  readonly changed = output<{ token: MapToken; option: LightOption | null }>();

  private readonly all = signal<readonly LightOption[] | null>(null);
  protected readonly failed = signal(false);
  protected readonly ready = computed(() => this.all() !== null);
  protected readonly name = computed(() =>
    carriedName(this.all() ?? [], this.token().carriedLight),
  );

  constructor() {
    // A required input has no value in the constructor: read the presets once it has.
    effect(() => {
      const campaignId = this.campaignId();
      void untracked(() =>
        this.presets.list(campaignId).then(
          (list) => this.all.set(list),
          () => this.failed.set(true),
        ),
      );
    });
  }

  protected open(): void {
    const options = this.all();
    if (!options) {
      return;
    }
    const token = this.token();
    openCarriedLight(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      mapId: this.mapId(),
      characterId: token.characterId,
      characterName: token.name,
      current: token.carriedLight,
      options: carriedOptions(options, token.carriedLight),
      changed: (next, option) => this.changed.emit({ token: next, option }),
    });
  }
}
