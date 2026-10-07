import { Component, computed, effect, inject, input, output, untracked } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { CharacterKind } from '../../../gen/meurpg/characters/v1/characters_pb';
import { seenCount } from '../../core/maps/vision';
import { MapsClient } from '../../core/maps/maps-client';
import { ViewAsMap } from '../../core/maps/view-as';
import { FogMap } from './fog-map';

/**
 * The map as the master sees it "Ver como" one character (MR-036, E9-03 state 5):
 * the amber band that says so, and the fog map with that player's tiles, tokens
 * and layers. It reads everything for itself (`ViewAsMap`) and again whenever
 * `tick` changes, because the stream tells the master nothing of what a player
 * sees. `read` reports the NPC tokens that view holds, for the line under the list.
 */
@Component({
  selector: 'app-view-as-map',
  imports: [FogMap, MatButtonModule, MatIconModule],
  template: `
    <div class="mr-notice mr-notice--warning band" role="status">
      <mat-icon aria-hidden="true">visibility</mat-icon>
      <p>
        Você está vendo o mapa como <strong>{{ name() }}</strong
        >. Para voltar {{ backTo() }}, escolha “Todos”.
      </p>
      <button mat-stroked-button type="button" class="band__back" (click)="back.emit()">Voltar {{ backTo() }}</button>
    </div>
    <app-fog-map
      [mapName]="mapName()"
      [imageWidth]="imageWidth()"
      [imageHeight]="imageHeight()"
      [status]="view.fog.status()"
      [error]="view.fog.error()"
      [vision]="view.fog.vision()"
      [layers]="view.fog.layers()"
      [tokens]="view.map.tokens()"
      [points]="view.map.points()"
      [pins]="true"
      [forCharacter]="characterId()"
      [viewer]="viewer()"
      [badge]="badge()"
      [selectablePoints]="false"
      [caption]="false"
    />
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
    }

    .band {
      align-items: center;
      flex-wrap: wrap;
    }

    .band p {
      flex: 1 1 14rem;
      margin: 0;
    }

    .band__back {
      min-height: 44px;
    }
  `,
})
export class ViewAsMapView {
  private readonly api = inject(MapsClient);

  readonly campaignId = input.required<string>();
  readonly mapId = input.required<string>();
  readonly characterId = input.required<string>();
  /** The character's name and its player's, for the band and the badge. */
  readonly name = input.required<string>();
  readonly playerName = input<string | null>(null);
  readonly mapName = input('');
  readonly imageWidth = input.required<number>();
  readonly imageHeight = input.required<number>();
  /** Where "Voltar" goes, in words: "ao seu mapa" on the session, "à sua vista" in the map editor. */
  readonly backTo = input('ao seu mapa');
  /** Goes up when the map may have changed for that player (a token moved, the map changed). */
  readonly tick = input(0);

  /** The character died or left the campaign (the read says `not_found`): the screen goes back to "Todos". */
  readonly gone = output<void>();
  /** "Voltar ao seu mapa". */
  readonly back = output<void>();
  /** The line about this view for the list: "Toren vê 22 quadrados de 384 e nenhum inimigo à vista." */
  readonly noteChange = output<string>();

  protected readonly view = new ViewAsMap(this.api, () => this.campaignId());
  protected readonly viewer = computed(() => ({ name: this.name(), own: false }));
  protected readonly badge = computed(() => {
    const player = this.playerName();
    return player ? `Vendo como ${this.name()} (${player})` : `Vendo como ${this.name()}`;
  });

  private seenTick: number | null = null;

  /** What the view holds: the squares seen and the enemies that player sees (counting, not rules). */
  protected readonly note = computed(() => {
    const vision = this.view.fog.vision();
    if (!vision) {
      return '';
    }
    const enemies = this.view.map
      .tokens()
      .filter(
        (t) =>
          !t.creatureId && t.kind !== CharacterKind.PLAYER && t.kind !== CharacterKind.UNSPECIFIED,
      )
      .map((t) => t.name);
    const seen = `${this.name()} vê ${seenCount(vision).toLocaleString('pt-BR')} ${seenCount(vision) === 1 ? 'quadrado' : 'quadrados'} de ${(vision.columns * vision.rows).toLocaleString('pt-BR')}`;
    return enemies.length === 0
      ? `${seen} e nenhum inimigo.`
      : `${seen} e ${enemies.length === 1 ? 'este inimigo' : 'estes inimigos'}: ${enemies.join(', ')}.`;
  });

  constructor() {
    effect(() => {
      const map = this.mapId();
      const character = this.characterId();
      untracked(() => void this.view.open(map, character));
    });
    effect(() => {
      const tick = this.tick();
      untracked(() => {
        // The first run is the read `open` already makes.
        if (this.seenTick !== null && tick !== this.seenTick) {
          void this.view.refresh();
        }
        this.seenTick = tick;
      });
    });
    effect(() => this.noteChange.emit(this.note()));
    effect(() => {
      if (this.view.fog.error() === 'gone' || this.view.map.status() === 'gone') {
        untracked(() => this.gone.emit());
      }
    });
  }
}
