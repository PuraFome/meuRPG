import { ChangeDetectionStrategy, Component, DestroyRef, inject } from '@angular/core';

import { type DieShape, RollAnimator, outcomeLine } from './roll-animator';

/** The tone of one visible face: `face` is the one the number is on; the others are lit from the upper left. */
type FacetTone = 'face' | 'lit' | 'mid' | 'shade';

/** One die drawn in a 100 × 100 box. */
interface DieDrawing {
  /** The faces turned toward the viewer, each with its tone and its corners. */
  readonly facets: readonly { readonly tone: FacetTone; readonly points: string }[];
  /** The edges between two visible faces. */
  readonly edges: string;
  /** The outline. */
  readonly rim: string;
  /** The middle of the number's face, and the number's size with one digit and with two. */
  readonly x: number;
  readonly y: number;
  readonly size: number;
  readonly long: number;
}

/**
 * Each die as the real solid seen from the front, projected from its true geometry: the d4 a pyramid with two faces,
 * the d6 a cube in three-quarter view, the d8 an octahedron, the d10 a pentagonal trapezohedron (the long kite face and
 * the zig-zag of its neighbours), the d12 and the d20 face on. The number sits inside the face toward the viewer.
 */
const DRAWINGS: Record<DieShape['shape'], DieDrawing> = {
  4: {
    facets: [
      { tone: 'shade', points: '95,71.5 60.5,5.3 81.5,94.7' },
      { tone: 'face', points: '81.5,94.7 60.5,5.3 5,80.3' },
    ],
    edges: 'M60.5 5.3L81.5 94.7',
    rim: '5,80.3 60.5,5.3 95,71.5 81.5,94.7',
    x: 49,
    y: 60.1,
    size: 26,
    long: 22,
  },
  6: {
    facets: [
      { tone: 'lit', points: '92.7,14 33.6,5 7.3,25.2 66.4,34.2' },
      { tone: 'shade', points: '66.4,95 92.7,74.8 92.7,14 66.4,34.2' },
      { tone: 'face', points: '7.3,25.2 7.3,86 66.4,95 66.4,34.2' },
    ],
    edges: 'M66.4 95L66.4 34.2M7.3 25.2L66.4 34.2M92.7 14L66.4 34.2',
    rim: '7.3,25.2 33.6,5 92.7,14 92.7,74.8 66.4,95 7.3,86',
    x: 36.8,
    y: 60.1,
    size: 28,
    long: 24,
  },
  8: {
    facets: [
      { tone: 'shade', points: '47.5,95 91.2,55.2 30.6,55.2' },
      { tone: 'lit', points: '52.5,5 8.8,44.8 30.6,55.2' },
      { tone: 'mid', points: '30.6,55.2 8.8,44.8 47.5,95' },
      { tone: 'face', points: '30.6,55.2 91.2,55.2 52.5,5' },
    ],
    edges: 'M91.2 55.2L30.6 55.2M52.5 5L30.6 55.2M47.5 95L30.6 55.2M8.8 44.8L30.6 55.2',
    rim: '8.8,44.8 52.5,5 91.2,55.2 47.5,95',
    x: 58.1,
    y: 38.5,
    size: 26,
    long: 22,
  },
  10: {
    facets: [
      { tone: 'lit', points: '33.3,54.2 50,5 17.4,43.1 19.6,58.9' },
      { tone: 'mid', points: '82.6,56.9 80.4,41.1 50,5 72.3,52.9' },
      { tone: 'mid', points: '19.6,58.9 50,95 53.5,65 33.3,54.2' },
      { tone: 'shade', points: '53.5,65 50,95 82.6,56.9 72.3,52.9' },
      { tone: 'face', points: '53.5,65 72.3,52.9 50,5 33.3,54.2' },
    ],
    edges:
      'M72.3 52.9L53.5 65M50 5L72.3 52.9M50 5L33.3 54.2M33.3 54.2L53.5 65M33.3 54.2L19.6 58.9M72.3 52.9L82.6 56.9M50 95L53.5 65',
    rim: '17.4,43.1 50,5 80.4,41.1 82.6,56.9 50,95 19.6,58.9',
    x: 52.5,
    y: 46,
    size: 26,
    long: 21,
  },
  12: {
    facets: [
      { tone: 'shade', points: '39.4,66.9 26.1,84.3 49.7,95 77.6,84.3 71.3,66.9' },
      { tone: 'lit', points: '39.4,66.9 29,36.6 9.2,35.3 7.4,64.7 26.1,84.3' },
      { tone: 'lit', points: '50.3,5 22.4,15.7 9.2,35.3 29,36.6 54.4,17.9' },
      { tone: 'shade', points: '71.3,66.9 77.6,84.3 90.8,64.7 92.6,35.3 80.5,36.6' },
      { tone: 'mid', points: '92.6,35.3 73.9,15.7 50.3,5 54.4,17.9 80.5,36.6' },
      { tone: 'face', points: '54.4,17.9 29,36.6 39.4,66.9 71.3,66.9 80.5,36.6' },
    ],
    edges:
      'M39.4 66.9L26.1 84.3M77.6 84.3L71.3 66.9M39.4 66.9L71.3 66.9M39.4 66.9L29 36.6M9.2 35.3L29 36.6M29 36.6L54.4 17.9M50.3 5L54.4 17.9M80.5 36.6L71.3 66.9M80.5 36.6L54.4 17.9M80.5 36.6L92.6 35.3',
    rim: '7.4,64.7 9.2,35.3 22.4,15.7 50.3,5 73.9,15.7 92.6,35.3 90.8,64.7 77.6,84.3 49.7,95 26.1,84.3',
    x: 54.9,
    y: 45,
    size: 28,
    long: 25,
  },
  20: {
    facets: [
      { tone: 'shade', points: '79,59 51.6,95 89.6,74.4' },
      { tone: 'shade', points: '9.2,74.4 51.6,95 29.3,59' },
      { tone: 'shade', points: '29.3,59 51.6,95 79,59' },
      { tone: 'lit', points: '48.4,5 10.4,25.6 53.5,16.2' },
      { tone: 'lit', points: '29.3,59 10.4,25.6 9.2,74.4' },
      { tone: 'lit', points: '53.5,16.2 10.4,25.6 29.3,59' },
      { tone: 'mid', points: '53.5,16.2 90.8,25.6 48.4,5' },
      { tone: 'shade', points: '89.6,74.4 90.8,25.6 79,59' },
      { tone: 'mid', points: '79,59 90.8,25.6 53.5,16.2' },
      { tone: 'face', points: '53.5,16.2 29.3,59 79,59' },
    ],
    edges:
      'M51.6 95L79 59M89.6 74.4L79 59M51.6 95L29.3 59M9.2 74.4L29.3 59M29.3 59L79 59M10.4 25.6L53.5 16.2M48.4 5L53.5 16.2M10.4 25.6L29.3 59M29.3 59L53.5 16.2M90.8 25.6L53.5 16.2M90.8 25.6L79 59M53.5 16.2L79 59',
    rim: '9.2,74.4 10.4,25.6 48.4,5 90.8,25.6 89.6,74.4 51.6,95',
    x: 53.9,
    y: 44.7,
    size: 26,
    long: 22,
  },
};

const TENS = 10;

/**
 * The dice roll animation, mounted once in the shell (docs/design.md#roll-animation). It is not a dialog: focus never
 * moves into it, the picture is hidden from assistive technology and the live region announces one sentence when the
 * dice land. It exists only while a roll plays (the landing takes under 2 s; the result stays up to 10 s), so it never blocks the page after that; a tap or Esc closes it.
 * Each die is drawn as the real polyhedron (`DRAWINGS`); a d100 is the two percentile d10, captioned "dezenas" and "unidades".
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
                [class.die--part]="!!d.part"
              >
                @let g = drawings[d.shape];
                @let text = shown(d, $index);
                <svg viewBox="0 0 100 100" focusable="false" [attr.data-die]="'d' + d.shape">
                  @for (f of g.facets; track f) {
                    <polygon [attr.class]="f.tone" [attr.points]="f.points" />
                  }
                  <path class="edge" [attr.d]="g.edges" />
                  <polygon class="rim" [attr.points]="g.rim" />
                  <text
                    class="num"
                    [attr.x]="g.x"
                    [attr.y]="g.y"
                    dy="0.35em"
                    [attr.font-size]="text.length > 1 ? g.long : g.size"
                    text-anchor="middle"
                    >{{ text }}</text
                  >
                </svg>
                @if (d.part) {
                  <span class="part">{{ d.part === 'tens' ? 'dezenas' : 'unidades' }}</span>
                }
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
  protected readonly drawings = DRAWINGS;

  constructor() {
    inject(DestroyRef).onDestroy(this.animator.attach());
  }

  /** What a die shows: its face once it has landed; while it tumbles, a face it has ("00" to "90" on the tens die of a d100). */
  protected shown(d: DieShape, index: number): string {
    if (index < this.animator.landed()) {
      return d.text;
    }
    const n = this.animator.spinning(index, d.spin);
    if (d.part === 'tens') {
      return String((n % TENS) * TENS).padStart(2, '0');
    }
    return String(d.part === 'units' ? n % TENS : n);
  }
}
