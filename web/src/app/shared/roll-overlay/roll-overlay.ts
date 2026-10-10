import { ChangeDetectionStrategy, Component, DestroyRef, inject } from '@angular/core';

import { RollAnimator, outcomeLine } from './roll-animator';

/**
 * The dice roll animation, mounted once in the shell (docs/design.md#roll-animation). It is not a dialog: focus never
 * moves into it, the picture is hidden from assistive technology and the live region announces one sentence when the
 * dice land. It exists only while a roll plays (the landing takes under 2 s; the result stays up to 10 s), so it never blocks the page after that; a tap or Esc closes it.
 * Each die has its own silhouette (d4 triangle, d6 square, d8 diamond, d10 kite, d12 pentagon, d20 hexagon with facets).
 */
@Component({
  selector: 'app-roll-overlay',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { '(document:keydown.escape)': 'animator.dismiss()' },
  template: `
    <p class="sr" role="status" aria-live="polite">{{ animator.announcement() }}</p>
    @if (animator.current(); as r) {
      <div class="scrim" aria-hidden="true" (click)="animator.dismiss()">
        <div class="stage">
          <div class="dice" [attr.data-count]="animator.shapes().length">
            @for (d of animator.shapes(); track $index) {
              <div
                class="die"
                [class.die--tumble]="$index >= animator.landed()"
                [class.die--landed]="$index < animator.landed()"
                [class.die--crit]="$index < animator.landed() && !!r.critical && d.counts"
                [class.die--fumble]="$index < animator.landed() && !!r.fumble && d.counts"
                [class.die--dim]="$index < animator.landed() && !d.counts"
              >
                <svg viewBox="0 0 100 100" focusable="false">
                  @switch (d.shape) {
                    @case (4) {
                      <polygon class="body" points="50,8 94,88 6,88" />
                      <path class="edge" d="M50 8L50 66M6 88L50 66M94 88L50 66" />
                    }
                    @case (6) {
                      <rect class="body" x="12" y="12" width="76" height="76" rx="12" />
                      <rect class="face" x="24" y="24" width="52" height="52" rx="6" />
                      <path class="edge" d="M12 12L24 24M88 12L76 24M12 88L24 76M88 88L76 76" />
                    }
                    @case (8) {
                      <polygon class="body" points="50,4 92,50 50,96 8,50" />
                      <polygon class="face" points="50,18 78,50 22,50" />
                      <path class="edge" d="M8 50H92M50 4V18M50 96L78 50M50 96L22 50" />
                    }
                    @case (10) {
                      <polygon class="body" points="50,4 92,40 50,96 8,40" />
                      <polygon class="face" points="50,18 76,40 50,60 24,40" />
                      <path class="edge" d="M50 4V18M92 40L76 40M8 40L24 40M50 96V60" />
                    }
                    @case (12) {
                      <polygon class="body" points="50,5 95,38 78,92 22,92 5,38" />
                      <polygon class="face" points="50,26 74,43 65,72 35,72 26,43" />
                      <path class="edge" d="M50 5V26M95 38L74 43M78 92L65 72M22 92L35 72M5 38L26 43" />
                    }
                    @default {
                      <polygon class="body" points="50,4 91,27 91,73 50,96 9,73 9,27" />
                      <polygon class="face" points="50,22 80,70 20,70" />
                      <path
                        class="edge"
                        d="M50 4V22M91 27L80 70M9 27L50 22M91 27L50 22M91 73L80 70M50 96L80 70M50 96L20 70M9 73L20 70M9 27L20 70"
                      />
                    }
                  }
                  <text
                    class="num"
                    [class.num--low]="d.shape === 4"
                    [class.num--long]="d.text.length > 1"
                    x="50"
                    [attr.y]="d.shape === 4 ? 78 : d.shape === 20 ? 63 : 58"
                    text-anchor="middle"
                  >
                    {{ $index < animator.landed() ? d.text : animator.spinning($index, d.spin) }}
                  </text>
                </svg>
              </div>
            }
            @if (animator.hidden() > 0) {
              <span class="more">+{{ animator.hidden() }}</span>
            }
          </div>
          @if (animator.done()) {
            <div class="result" [class.result--crit]="!!r.critical" [class.result--fumble]="!!r.fumble">
              <span class="result__label">{{ r.label }}</span>
              @if (r.line) {
                <b class="result__line">{{ r.line }}</b>
              }
              @if (r.note) {
                <span class="result__note">{{ r.note }}</span>
              }
              @if (outcome(r); as word) {
                <b
                  class="result__word"
                  [class.result__word--good]="r.outcome?.good && !r.fumble"
                  [class.result__word--bad]="(r.outcome && !r.outcome.good) || r.fumble"
                  >{{ word }}</b
                >
              }
            </div>
            <span class="hint">Toque para fechar</span>
          }
        </div>
      </div>
    }
  `,
  styleUrl: './roll-overlay.scss',
})
export class RollOverlay {
  protected readonly animator = inject(RollAnimator);
  protected readonly outcome = outcomeLine;

  constructor() {
    inject(DestroyRef).onDestroy(this.animator.attach());
  }
}
