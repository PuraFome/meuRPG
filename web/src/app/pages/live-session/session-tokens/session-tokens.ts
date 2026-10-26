import { ChangeDetectionStrategy, Component, computed, inject, input, signal } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import type { MapFormation } from '../../../../gen/meurpg/maps/v1/maps_pb';
import type { MapLayers } from '../../../core/maps/layers';
import { playersMoveErrorMessage } from '../../../core/maps/map-errors';
import { MapReveals } from '../../../core/maps/map-reveals';
import type { MapState } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
import { TokenMoveClient } from '../../../core/maps/token-move-client';
import type { Point } from '../../../core/maps/token-move';
import type { TokenMover } from '../../../core/maps/token-mover';
import type { PartyMemberInfoVm } from '../../../core/play/live-session.types';
import type { CluePlayer } from '../../../core/maps/scene-clues';
import { MapTokensList } from '../../../shared/map-lists/map-tokens-list';
import { openSheet } from '../../../shared/sheet/sheet-host';
import { KeepSwitch } from '../shown-image-panel/keep-switch';
import { PutPartyDialog, type PutPartyData } from './put-party-dialog';
import { TokenMovePanel } from './token-move-panel';

/**
 * The master's "Tokens" tab under the map (E5-04, E5-06, B2): "Os jogadores movem os próprios personagens" (the
 * switch of the map), "Pôr o grupo no mapa" and "Selecionar" (a tap on a token adds it to a selection), "Mover Tor" or
 * "N selecionados" (`app-token-move-panel`), and each token of the current map with "Visível" or "Escondido" and a box
 * that chooses it to move. Without it the master could not reveal a hidden NPC from the session page (RN-10).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-session-tokens',
  imports: [KeepSwitch, MapTokensList, MatButtonModule, MatIconModule, TokenMovePanel],
  template: `
    <section class="mr-panel" aria-labelledby="tokens-heading">
      @if (state().map()) {
        <div class="top">
          <app-keep-switch
            id="players-move"
            label="Os jogadores movem os próprios personagens"
            [hint]="playersMove() ? 'Cada jogador arrasta só o próprio token. O mestre continua movendo todos.' : 'Só o mestre move os tokens neste mapa.'"
            [checked]="playersMove()"
            [disabled]="switching()"
            (toggle)="setPlayersMove($event)"
          />
          @if (switchError(); as message) {
            <div class="mr-notice mr-notice--danger" role="alert">
              <mat-icon aria-hidden="true">error</mat-icon>
              <p>{{ message }}</p>
            </div>
          }
          <div class="actions">
            <button
              matButton="outlined"
              type="button"
              class="actions__btn"
              [class.mr-button--off]="!hasGrid() || members().length === 0"
              [disabled]="!hasGrid() || members().length === 0"
              disabledInteractive
              [attr.aria-describedby]="hasGrid() && members().length > 0 ? null : 'tokens-why'"
              (click)="putParty()"
            >
              <mat-icon aria-hidden="true">groups</mat-icon>Pôr o grupo no mapa
            </button>
            <button
              matButton="outlined"
              type="button"
              class="actions__btn"
              [class.actions__btn--on]="mover().selecting()"
              [attr.aria-pressed]="mover().selecting()"
              (click)="mover().setSelecting(!mover().selecting())"
            >
              @if (mover().selecting()) {
                <mat-icon aria-hidden="true">check</mat-icon>
              }
              <mat-icon aria-hidden="true">select_all</mat-icon>Selecionar
            </button>
          </div>
          @if (!hasGrid()) {
            <p class="mr-muted" id="tokens-why">Este mapa não tem grade: sem quadrados não dá para pôr o grupo em formação nem contar o movimento.</p>
          } @else if (members().length === 0) {
            <p class="mr-muted" id="tokens-why">Nenhum personagem de jogador vivo para pôr no mapa.</p>
          } @else {
            <p class="mr-muted hint">
              <b>Arrastar</b> um token move esse token. <b>Shift e clique</b> escolhem vários.
            </p>
          }
          @if (partyNotice(); as note) {
            <p class="mr-notice mr-notice--success" role="status"><mat-icon aria-hidden="true">check_circle</mat-icon><span>{{ note }}</span></p>
          }
        </div>
        <app-token-move-panel [mover]="mover()" />
        <app-map-tokens-list
          [tokens]="state().tokens()"
          [info]="info()"
          [pendingId]="reveals.pendingId()"
          [selectable]="hasGrid()"
          [chosen]="mover().selected()"
          (pick)="mover().pick($event, true)"
          (toggle)="reveals.toggleToken($event.token, $event.hidden)"
        />
        @if (reveals.error(); as message) {
          <div class="mr-notice mr-notice--danger" role="alert">
            <mat-icon aria-hidden="true">error</mat-icon>
            <p>{{ message }}</p>
          </div>
        }
      } @else {
        <h2 class="title" id="tokens-heading">Tokens no mapa</h2>
        <p class="mr-muted">Escolha um mapa atual para ver os tokens dele.</p>
      }
      <p class="mr-visually-hidden" role="status" aria-live="polite">{{ reveals.announcement() }}</p>
    </section>
  `,
  styles: `
    :host {
      display: block;
    }

    .title {
      margin: 0 0 var(--mr-space-1);
      font-family: var(--mr-font-display);
      font-size: 21px;
      font-weight: 700;
      line-height: 26px;
    }

    p {
      margin: 0;
    }

    .top {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-2);
      margin-bottom: var(--mr-space-3);
    }

    .actions {
      display: flex;
      flex-wrap: wrap;
      gap: var(--mr-space-2);
    }

    .actions__btn {
      min-height: 44px;
    }

    .actions__btn--on {
      background: var(--mr-accent-soft);
      color: var(--mr-accent-text);
    }

    .hint {
      font-size: 14px;
      line-height: 19px;
    }
  `,
})
export class SessionTokens {
  private readonly api = inject(MapsClient);
  private readonly moveApi = inject(TokenMoveClient);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);

  readonly campaignId = input.required<string>();
  readonly state = input.required<MapState>();
  readonly info = input<ReadonlyMap<string, PartyMemberInfoVm>>(new Map());
  /** The moving of tokens: the page owns it, so the map and this tab speak of the same selection. */
  readonly mover = input.required<TokenMover>();
  /** The living player characters, for "Pôr o grupo no mapa". */
  readonly party = input<readonly CluePlayer[]>([]);
  /** The map's layers, for the picture of the dialog (the fog map's, when it has one). */
  readonly layers = input<MapLayers | null>(null);

  protected readonly reveals = new MapReveals(
    this.api,
    () => this.state(),
    () => this.campaignId(),
  );
  protected readonly switching = signal(false);
  protected readonly switchError = signal('');
  protected readonly partyNotice = signal('');

  protected readonly playersMove = computed(() => this.state().map()?.playersMoveTokens ?? false);
  protected readonly hasGrid = computed(() => this.mover().hasGrid());
  protected readonly members = computed(() =>
    this.party().map((p) => ({
      id: p.id,
      name: p.name,
      sub: this.info().get(p.id)?.classSummary ?? '',
    })),
  );

  /** The switch of the map: the server answers with the map as it is now. */
  protected async setPlayersMove(enabled: boolean): Promise<void> {
    const map = this.state().map();
    if (!map || this.switching()) {
      return;
    }
    this.switching.set(true);
    this.switchError.set('');
    try {
      this.state().setMap(await this.moveApi.setPlayersMove(this.campaignId(), map.id, enabled));
    } catch (err) {
      this.switchError.set(playersMoveErrorMessage(err));
    } finally {
      this.switching.set(false);
    }
  }

  /** "Pôr o grupo no mapa": the dialog, then the tokens as the server put them. */
  protected putParty(): void {
    const map = this.state().map();
    if (!map || !this.hasGrid() || this.members().length === 0) {
      return;
    }
    this.partyNotice.set('');
    const first = this.state()
      .tokens()
      .find((t) => !t.creatureId);
    const data: PutPartyData = {
      mapName: map.name,
      members: this.members(),
      image: map.image
        ? { url: map.image.url, width: map.image.width, height: map.image.height }
        : null,
      layers: this.layers(),
      grid: this.mover().grid(),
      tokens: this.state().tokens(),
      start: first ? { xBp: first.xBp, yBp: first.yBp } : { xBp: 5000, yBp: 5000 },
      put: async (ids: readonly string[], formation: MapFormation, at: Point) => {
        const answer = await this.moveApi.putParty(
          this.campaignId(),
          map.id,
          ids,
          formation,
          at.xBp,
          at.yBp,
        );
        for (const token of answer.tokens) {
          this.state().upsertToken(token);
        }
        this.partyNotice.set(
          answer.movedForRoom > 0
            ? `${answer.tokens.length} no mapa. ${answer.movedForRoom} foi para o quadrado livre mais perto.`
            : `${answer.tokens.length} no mapa.`,
        );
        return answer;
      },
    };
    openSheet<PutPartyDialog, PutPartyData, boolean | undefined>(
      this.dialog,
      this.bottomSheet,
      PutPartyDialog,
      {
        data,
        ariaLabel: 'Pôr o grupo no mapa',
        labelledBy: 'party-title',
        width: '760px',
      },
    ).subscribe();
  }
}
