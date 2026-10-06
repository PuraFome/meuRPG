import { NgTemplateOutlet } from '@angular/common';
import { ChangeDetectionStrategy, Component, ElementRef, computed, inject, input, output, signal } from '@angular/core';

import { lightLabel } from '../../core/puzzles/puzzle-format';

/** A light a player pressed: from 0, from the top left. */
export interface LightPress {
  readonly row: number;
  readonly col: number;
}

/**
 * The board of "Apagar as luzes" (MR-038, E10-06): a square of lights, a lit one a sun on a warm ground and an off one an
 * empty ring, so the state is never the color alone. It draws what the server sends and does no rules math: the page
 * decides what a press does by sending it.
 *
 * - `play` is the player's: every light is a button named "Luz na linha 2, coluna 3, acesa", one tab stop and the arrow
 *   keys walk the squares (the grid pattern); `view` is the master's static board.
 * - `changed` marks the lights that moved in the last move (a dashed frame, so the table sees what just happened); `hints` marks
 *   the master's "toques que resolvem" (a solid garnet frame; only the master ever gets them).
 * - 48 px lights with 3 px between them; on a phone narrower than 375 px the lights touch (each keeps its own 1,5 px border)
 *   and the board bleeds 6 px from the screen's edge, which is what lets a 7 × 7 board keep 44 px lights at 320 px.
 */
@Component({
  selector: 'app-lights-board',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { '[class.lb--play]': "mode() === 'play'", '[style.--n]': 'size()' },
  template: `
    <div class="lb" role="group" [attr.aria-label]="'Painel de luzes, ' + size() + ' por ' + size()" [attr.aria-disabled]="disabled() ? 'true' : null">
      @for (on of lit(); track $index) {
        @let row = floor($index / size());
        @let col = $index % size();
        @if (mode() === 'play') {
          <button
            type="button"
            class="cell"
            [class.cell--lit]="on"
            [class.cell--changed]="changedSet().has($index)"
            [class.cell--hint]="hintSet().has($index)"
            [attr.data-i]="$index"
            [attr.aria-label]="label(row, col, on)"
            [attr.aria-disabled]="disabled() ? 'true' : null"
            [tabindex]="$index === tabStop() ? 0 : -1"
            (focus)="active.set($index)"
            (keydown)="onKey($event, $index)"
            (click)="!disabled() && press.emit({ row, col })"
          >
            <ng-container *ngTemplateOutlet="mark; context: { on }" />
          </button>
        } @else {
          <span class="cell" role="img" [class.cell--lit]="on" [class.cell--changed]="changedSet().has($index)" [class.cell--hint]="hintSet().has($index)" [attr.aria-label]="label(row, col, on)">
            <ng-container *ngTemplateOutlet="mark; context: { on }" />
          </span>
        }
      }
    </div>
    <ng-template #mark let-on="on">
      <svg viewBox="0 0 24 24" aria-hidden="true">
        @if (on) {
          <circle cx="12" cy="12" r="4.5" fill="currentColor" />
          <path d="M12 2.5v3M12 18.5v3M2.5 12h3M18.5 12h3M5.3 5.3l2.1 2.1M16.6 16.6l2.1 2.1M18.7 5.3l-2.1 2.1M7.4 16.6l-2.1 2.1" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" fill="none" />
        } @else {
          <circle cx="12" cy="12" r="4" fill="none" stroke="currentColor" stroke-width="2" />
        }
      </svg>
    </ng-template>
  `,
  styleUrl: './lights-board.scss',
  imports: [NgTemplateOutlet],
})
export class LightsBoard {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  /** Lights a side, 3 to 7. */
  readonly size = input.required<number>();
  /** The board row by row from the top left, as the server sends it. */
  readonly lit = input.required<readonly boolean[]>();
  readonly mode = input<'play' | 'view'>('view');
  /** Squares (row * size + col) that changed in the last move. */
  readonly changed = input<readonly number[]>([]);
  /** Squares the master's hint rings (a shortest way). */
  readonly hints = input<readonly number[]>([]);
  /** Solved or stopped: nothing moves any more. */
  readonly disabled = input(false);

  readonly press = output<LightPress>();

  protected readonly floor = Math.floor;
  protected readonly changedSet = computed(() => new Set(this.changed()));
  protected readonly hintSet = computed(() => new Set(this.hints()));
  /** The light that holds the board's one tab stop (the last one focused). */
  protected readonly active = signal(0);
  protected readonly tabStop = computed(() => Math.min(this.active(), Math.max(0, this.lit().length - 1)));
  protected readonly label = lightLabel;

  protected onKey(event: KeyboardEvent, index: number): void {
    const n = this.size();
    const row = Math.floor(index / n);
    const col = index % n;
    let next = index;
    switch (event.key) {
      case 'ArrowRight':
        next = col < n - 1 ? index + 1 : index;
        break;
      case 'ArrowLeft':
        next = col > 0 ? index - 1 : index;
        break;
      case 'ArrowDown':
        next = row < n - 1 ? index + n : index;
        break;
      case 'ArrowUp':
        next = row > 0 ? index - n : index;
        break;
      case 'Home':
        next = row * n;
        break;
      case 'End':
        next = row * n + n - 1;
        break;
      default:
        return;
    }
    event.preventDefault();
    this.active.set(next);
    this.host.nativeElement.querySelector<HTMLElement>(`[data-i="${next}"]`)?.focus();
  }
}
