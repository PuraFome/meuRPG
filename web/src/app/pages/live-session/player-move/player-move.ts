import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  input,
  signal,
  untracked,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import type { MapState } from '../../../core/maps/map-state';
import type { TokenMover } from '../../../core/maps/token-mover';
import { PHONE_QUERY, mediaQuery } from '../../../shared/map-view/media-query';
import { openSheet } from '../../../shared/sheet/sheet-host';
import { TokenMovePanel } from '../session-tokens/token-move-panel';
import { PlayerMoveSheet, type PlayerMoveSheetData } from './player-move-sheet';

/** How long the line that says the master freed or locked the moving stays, before the usual line comes back (C9-03). */
export const NOTICE_MS = 8000;

/**
 * Under the player's map (C9-01, C9-02, C9-03): one line that says who moves the tokens. With "Os jogadores movem os próprios
 * personagens" off (the map's default) it is "O mestre move os tokens agora." and there is no control; on, it says "Arraste Brisa no mapa
 * ou use “Mover Brisa”. Os outros ficam com o mestre." with the button "Mover Brisa", only for the player's own character. The button
 * opens the master's panel (direction pad, steps, the plan, the button that sends it): in the card on a computer, in a sheet on a phone.
 * When the master frees or locks the moving while the player looks, the line is a polite one-line notice for 8 s ("O mestre liberou:
 * agora você move Brisa." / "O mestre voltou a mover os tokens."), the focus stays where it was, and locking closes the panel without moving.
 * The server decides every move; there is no warning before sending a path across a marked square, only the result says it.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-player-move',
  imports: [MatIconModule, TokenMovePanel],
  template: `
    @if (state().map()) {
      <div class="pm">
        @if (notice(); as n) {
          <p class="pm__line pm__line--notice" [class.pm__line--free]="n.free">
            <mat-icon aria-hidden="true">{{ n.free ? 'open_with' : 'lock' }}</mat-icon>{{ n.text }}
          </p>
        } @else if (canMove()) {
          <p class="pm__line">
            <mat-icon aria-hidden="true">open_with</mat-icon>{{ freeLine() }}
          </p>
        } @else {
          <p class="pm__line">
            <mat-icon aria-hidden="true">lock</mat-icon>O mestre move os tokens agora.
          </p>
        }
        @if (canMove() && !open()) {
          <button type="button" class="pm__go" (click)="start()">
            <mat-icon aria-hidden="true">open_with</mat-icon>Mover {{ name() }}
          </button>
        }
        @if (canMove() && open() && !phone()) {
          <app-token-move-panel [mover]="mover()" (closed)="stop()" />
        }
        <p class="mr-visually-hidden" role="status" aria-live="polite">{{ announced() }}</p>
      </div>
    }
  `,
  styleUrl: './player-move.scss',
})
export class PlayerMove {
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);

  readonly mover = input.required<TokenMover>();
  readonly state = input.required<MapState>();
  readonly characterId = input.required<string>();
  /** The player's character, in "Mover Brisa". */
  readonly name = input.required<string>();
  /** The map takes a drag of the player's own token (a computer: the phone moves with the button). */
  readonly drag = input(false);

  protected readonly phone = mediaQuery(PHONE_QUERY);
  protected readonly open = signal(false);
  protected readonly notice = signal<{ readonly text: string; readonly free: boolean } | null>(
    null,
  );
  protected readonly announced = signal('');
  /** The master's switch for this map. */
  protected readonly canMove = computed(() => this.state().map()?.playersMoveTokens === true);
  protected readonly freeLine = computed(() =>
    this.drag()
      ? `Arraste ${this.name()} no mapa ou use “Mover ${this.name()}”. Os outros ficam com o mestre.`
      : `Use “Mover ${this.name()}”. Os outros ficam com o mestre.`,
  );

  private seen: boolean | null = null;
  private timer: ReturnType<typeof setTimeout> | null = null;

  constructor() {
    const destroyRef = inject(DestroyRef);
    destroyRef.onDestroy(() => this.forgetTimer());
    // The switch changes under the player's eyes: the one-line notice for 8 s, and a locked map puts the panel away.
    effect(() => {
      const now = this.canMove();
      untracked(() => {
        const before = this.seen;
        this.seen = now;
        if (before === null || before === now) {
          return;
        }
        if (!now) {
          this.open.set(false);
          this.mover().clear();
        }
        const text = now
          ? `O mestre liberou: agora você move ${this.name()}.`
          : 'O mestre voltou a mover os tokens.';
        this.announced.set(text);
        this.notice.set({ text, free: now });
        this.forgetTimer();
        this.timer = setTimeout(() => {
          this.notice.set(null);
          this.announced.set('');
        }, NOTICE_MS);
      });
    });
  }

  private forgetTimer(): void {
    if (this.timer !== null) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }

  /** "Mover Brisa": the panel, with the player's own token as the one chosen. */
  protected start(): void {
    const mover = this.mover();
    mover.clear();
    mover.pick(this.characterId());
    if (!this.phone()) {
      this.open.set(true);
      return;
    }
    const data: PlayerMoveSheetData = {
      mover,
      name: this.name(),
      canMove: () => this.canMove(),
    };
    this.open.set(true);
    openSheet<PlayerMoveSheet, PlayerMoveSheetData, undefined>(
      this.dialog,
      this.bottomSheet,
      PlayerMoveSheet,
      { data, ariaLabel: `Mover ${this.name()}`, labelledBy: 'move-title' },
    ).subscribe(() => this.stop());
  }

  protected stop(): void {
    this.open.set(false);
    this.mover().clear();
  }
}
