import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { MoveTrail } from '../../core/maps/token-mover';
import type { Point } from '../../core/maps/token-move';

const FULL = 10000;
const PERCENT = 100;
/** The ring around a chosen token is this many squares wide. */
const RING_SQUARES = 1.5;

/**
 * What the master's moving of tokens draws over the map (B2-01, B2-03, B2-13): a dashed ring on each token chosen
 * ("Selecionado"), the plan (a dashed line to the destination, which is a dashed ring too: "Para onde vai"), and
 * after a move the line the token walked, with an X and "7,5 m · parou antes" where it was stopped. Pure drawing in
 * the map's stage, `aria-hidden`: the panel beside the map says the same in words. It sits in `mapOverlay`.
 */
@Component({
  selector: 'app-move-overlay',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <svg class="lines" viewBox="0 0 100 100" preserveAspectRatio="none">
      @if (plan(); as p) {
        <line class="line line--plan" [attr.x1]="x(p.from)" [attr.y1]="y(p.from)" [attr.x2]="x(p.to)" [attr.y2]="y(p.to)" />
      }
      @if (trail(); as t) {
        <line class="line line--done" [attr.x1]="x(t.from)" [attr.y1]="y(t.from)" [attr.x2]="x(t.to)" [attr.y2]="y(t.to)" />
        @if (t.stop; as stop) {
          <line class="line line--plan" [attr.x1]="x(t.to)" [attr.y1]="y(t.to)" [attr.x2]="x(stop.at)" [attr.y2]="y(stop.at)" />
        }
      }
    </svg>
    @for (r of rings(); track $index) {
      <span class="ring" [style.left.%]="x(r)" [style.top.%]="y(r)" [style.width.%]="size()" [style.aspect-ratio]="1"></span>
    }
    @if (plan(); as p) {
      <span class="ring ring--dest" [style.left.%]="x(p.to)" [style.top.%]="y(p.to)" [style.width.%]="size()" [style.aspect-ratio]="1"></span>
    }
    @if (trail()?.stop; as stop) {
      <span class="x" [style.left.%]="x(stop.at)" [style.top.%]="y(stop.at)"><mat-icon>cancel</mat-icon></span>
      <span class="tag" [style.left.%]="mid().x" [style.top.%]="mid().y">{{ stop.label }}</span>
    }
  `,
  styles: `
    :host {
      position: absolute;
      inset: 0;
      z-index: 2;
      display: block;
      pointer-events: none;
    }

    .lines {
      position: absolute;
      inset: 0;
      width: 100%;
      height: 100%;
      overflow: visible;
    }

    .line {
      fill: none;
      stroke: var(--mr-accent);
      stroke-width: 3;
      stroke-linecap: round;
      vector-effect: non-scaling-stroke;
    }

    .line--plan {
      stroke-dasharray: 6 5;
    }

    .ring {
      position: absolute;
      box-sizing: border-box;
      transform: translate(-50%, -50%);
      border: 2px dashed var(--mr-accent);
      border-radius: 50%;
      background: color-mix(in srgb, var(--mr-accent) 16%, transparent);
    }

    .ring--dest {
      background: none;
    }

    // The zoom of the map's stage scales what it holds; the X and the label keep their size (the variable --s is the zoom).
    .x {
      position: absolute;
      display: flex;
      transform: translate(-50%, -50%) scale(calc(1 / var(--s, 1)));
      color: var(--mr-accent);
      filter: drop-shadow(0 0 2px var(--mr-surface));

      .mat-icon {
        width: 22px;
        height: 22px;
        font-size: var(--mr-font-22);
      }
    }

    .tag {
      position: absolute;
      transform: translate(-50%, -140%) scale(calc(1 / var(--s, 1)));
      padding: 2px 10px;
      border: 1px solid var(--mr-accent);
      border-radius: 999px;
      background: var(--mr-surface);
      color: var(--mr-ink);
      font-size: var(--mr-font-12);
      font-weight: 700;
      line-height: 16px;
      white-space: nowrap;
    }
  `,
  host: { 'aria-hidden': 'true' },
})
export class MoveOverlay {
  /** The tokens chosen. */
  readonly rings = input<readonly Point[]>([]);
  readonly plan = input<MoveTrail | null>(null);
  readonly trail = input<MoveTrail | null>(null);
  /** The grid's columns: a ring is 1,5 squares wide. */
  readonly columns = input(0);

  protected readonly size = computed(() =>
    this.columns() > 0 ? (RING_SQUARES / this.columns()) * PERCENT : 0,
  );
  /** Where the stop label sits: over the line, half way to the X. */
  protected readonly mid = computed(() => {
    const t = this.trail();
    const stop = t?.stop;
    return t && stop
      ? { x: (this.x(t.to) + this.x(stop.at)) / 2, y: (this.y(t.to) + this.y(stop.at)) / 2 }
      : { x: 0, y: 0 };
  });

  protected x(p: Point): number {
    return (p.xBp / FULL) * PERCENT;
  }

  protected y(p: Point): number {
    return (p.yBp / FULL) * PERCENT;
  }
}
