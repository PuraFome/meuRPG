import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { SequencePlayback } from '../../../gen/meurpg/play/v1/puzzles_pb';
import type { SymbolFace } from '../../core/puzzles/puzzle-symbols';
import { BellsBoard } from './bells-board';
import { SequenceStrip } from './sequence-strip';
import { SymbolGlyph } from './symbol-glyph';

/**
 * The sequence on a player's phone (MR-038, RN-27, E10-12 state 7): watch it play, then repeat it.
 *
 * - **Not played yet:** the bells are dashed and the page says the master has not played it.
 * - **Playing:** the steps the server has revealed so far (the browser never knows the next one), the step number ("passo 3 de 6"),
 *   the bell playing in big with its name, written and announced for a screen reader: no sound is needed, nothing is lost without it.
 * - **Repeating:** "Agora é com vocês." Every player taps the bells; the server checks the order, and the count of right steps is the
 *   server's `progress`.
 *
 * It draws what `SequencePlayback` says and decides nothing; a bell tapped is emitted as the bell's number.
 */
@Component({
  selector: 'app-sequence-board',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [BellsBoard, MatIconModule, SequenceStrip, SymbolGlyph],
  template: `
    @let pb = playback();
    @if (pb.playing) {
      <p class="line"><strong>O mestre está tocando os sinos.</strong> Observe e escute: passo {{ pb.shown.length }} de {{ pb.totalSteps }}.</p>
      <app-sequence-strip [faces]="faces()" [steps]="pb.shown" [total]="pb.totalSteps" [current]="pb.shown.length - 1" />
      @if (nowPlaying(); as bell) {
        <!-- A new node for each step, so the little pulse plays again even when two steps in a row are the same bell. -->
        @for (step of [pb.shown.length]; track step) {
          <div class="big">
            <app-symbol-glyph class="big__glyph" [face]="bell.key" aria-hidden="true" />
            <span class="big__name">{{ bell.namePt }}</span>
          </div>
        }
        <p class="mr-visually-hidden" role="status" aria-live="polite">Passo {{ pb.shown.length }} de {{ pb.totalSteps }}: {{ bell.namePt }}.</p>
      }
    } @else if (pb.plays === 0) {
      <p class="line"><strong>O mestre ainda não tocou os sinos.</strong> Esperem ele tocar a sequência.</p>
      <app-bells-board [faces]="faces()" [disabled]="true" label="Os sinos, ainda parados" />
    } @else {
      <p class="line"><strong>Agora é com vocês.</strong> Toquem os sinos na mesma ordem. Qualquer um pode tocar.</p>
      @if (note(); as n) {
        <div class="mr-notice mr-notice--danger" role="alert">
          <mat-icon aria-hidden="true">close</mat-icon>
          <p><strong>{{ n.lead }}</strong> {{ n.text }}</p>
        </div>
      }
      <app-bells-board [faces]="faces()" [disabled]="disabled()" (strike)="strike.emit($event)" />
      <p class="count"><span>Passos certos</span><strong>{{ progress() }} de {{ pb.totalSteps }}</strong></p>
    }
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
    }

    .line {
      margin: 0;
      font-size: 17px;
      line-height: 24px;
    }

    .big {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: var(--mr-space-2);
      padding: var(--mr-space-3) 0;
      animation: strike 240ms ease-out;
    }

    .big__glyph {
      --glyph: 72px;
    }

    .big__name {
      font-size: 16px;
      color: var(--mr-ink-muted);
    }

    .count {
      display: flex;
      justify-content: space-between;
      margin: 0;
      font-size: 16px;
    }

    @keyframes strike {
      from {
        transform: scale(0.9);
        opacity: 0.4;
      }
    }

    @media (prefers-reduced-motion: reduce) {
      .big {
        animation: none;
      }
    }
  `,
})
export class SequenceBoard {
  readonly playback = input.required<SequencePlayback>();
  readonly faces = input.required<readonly SymbolFace[]>();
  /** How many steps the attempt has right so far (the server's). */
  readonly progress = input(0);
  /** Solved or stopped. */
  readonly disabled = input(false);
  /** "Errou o passo 4." and what came of it, above the bells, when the last bell was wrong. */
  readonly note = input<{ readonly lead: string; readonly text: string } | null>(null);

  /** A bell struck, from 0. */
  readonly strike = output<number>();

  protected readonly nowPlaying = computed(() => {
    const shown = this.playback().shown;
    return shown.length > 0 ? (this.faces()[shown[shown.length - 1]] ?? null) : null;
  });
}
