import { ChangeDetectionStrategy, Component, input } from '@angular/core';

import type { DoorKind } from '../../core/maps/layers';

/** The words for a door's kind, as the legend and the sheet say them (MAP-LANGUAGE-E10.md). */
export const DOOR_NAME: Readonly<Record<DoorKind, string>> = {
  1: 'Porta aberta',
  2: 'Porta fechada',
  3: 'Porta trancada',
  4: 'Grade',
  5: 'Porta secreta',
};

/**
 * One door, drawn in the one way MAP-LANGUAGE-E10.md gives its kind, on the map and in the legend:
 *
 * - **Fechada:** a solid ink bar across the gap, the full width of the square, 4 px thick.
 * - **Aberta:** a leaf against one side, as long as the gap is wide, and the dashed quarter arc it swept, of the same radius.
 * - **Trancada** (the master only): the closed bar and a padlock drawn over about 45 % of the square.
 * - **Grade:** a comb of four short bars across the gap.
 * - **Secreta** (the master only): the wall's hatch, a dashed frame and a keyhole; never a letter or a disc (those are tokens).
 *
 * `axis` says which way the gap runs: `h` draws the bar left to right (the passage goes up and down), `v` turns the drawing.
 * Presentational and decorative: the screens name a door in words (the legend, the sheet, the buttons' labels).
 */
@Component({
  selector: 'app-door-mark',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @switch (state()) {
      @case (5) {
        <span class="dm__secret"><span class="dm__frame"></span></span>
        <svg class="dm__key" viewBox="0 0 100 100" aria-hidden="true">
          <circle cx="50" cy="42" r="10" />
          <path d="M45 48h10l4 24H41z" />
        </svg>
      }
      @default {
        <svg class="dm__svg" [class.dm__svg--v]="axis() === 'v'" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
          @switch (state()) {
            @case (1) {
              <!-- Aberta: the leaf swung against the left side, as long as the gap is wide, and the quarter arc it swept, of the same radius. -->
              <path class="dm__halo dm__leafw" d="M8 8V92" />
              <path class="dm__leaf" d="M8 8V92" />
              <path class="dm__arc" d="M92 8A84 84 0 0 1 8 92" />
            }
            @case (4) {
              <!-- Grade: a comb of four bars across the gap. -->
              <path class="dm__ink dm__thin" d="M0 50H100M14 14V86M38 14V86M62 14V86M86 14V86" />
            }
            @default {
              <path class="dm__halo" d="M0 50H100" />
              <path class="dm__ink" d="M0 50H100" />
            }
          }
        </svg>
        @if (state() === 3) {
          <!-- Trancada: a padlock about 45 % of the square on the middle of the bar, a drawing that scales with the square. -->
          <svg class="dm__lock" viewBox="0 0 100 100" aria-hidden="true">
            <rect class="dm__chip" x="24" y="22" width="52" height="58" rx="8" />
            <path class="dm__shackle" d="M39 48V40A11 11 0 0 1 61 40V48" />
            <rect class="dm__body" x="34" y="48" width="32" height="22" rx="3" />
            <circle class="dm__hole" cx="50" cy="58" r="3.5" />
          </svg>
        }
      }
    }
  `,
  styles: `
    :host {
      position: relative;
      display: block;
      width: 100%;
      height: 100%;
    }

    .dm__svg,
    .dm__key {
      position: absolute;
      inset: 0;
      width: 100%;
      height: 100%;
      overflow: visible;
      fill: none;
    }

    // A turned gap: the same drawing, a quarter turn.
    .dm__svg--v {
      transform: rotate(90deg);
    }

    .dm__ink,
    .dm__leaf,
    .dm__arc,
    .dm__halo {
      vector-effect: non-scaling-stroke;
      stroke-linecap: butt;
    }

    .dm__ink {
      stroke: var(--mr-map-token-ink);
      stroke-width: 4px;
    }

    // A pale edge under the ink, so a bar on a dark picture still reads.
    .dm__halo {
      stroke: var(--mr-map-token-surface);
      stroke-width: 7px;
      opacity: 0.85;
    }

    .dm__thin.dm__ink {
      stroke-width: 2px;
    }

    .dm__leaf {
      stroke: var(--mr-map-token-ink);
      stroke-width: 3px;
    }

    .dm__leafw {
      stroke-width: 6px;
    }

    .dm__arc {
      stroke: var(--mr-map-token-ink);
      stroke-width: 1.5px;
      stroke-dasharray: 3 2.5;
    }

    // Trancada: the padlock is a drawing over the whole square (about 45 % of it), so it grows and shrinks with the square.
    .dm__lock {
      position: absolute;
      inset: 0;
      width: 100%;
      height: 100%;
      overflow: visible;
    }

    .dm__chip {
      fill: var(--mr-map-token-surface);
      stroke: var(--mr-map-token-ink);
      stroke-width: 1.5px;
      vector-effect: non-scaling-stroke;
    }

    .dm__shackle {
      fill: none;
      stroke: var(--mr-map-token-ink);
      stroke-width: 6;
    }

    .dm__body {
      fill: var(--mr-map-token-ink);
    }

    .dm__hole {
      fill: var(--mr-map-token-surface);
    }

    // Secreta: the wall's own hatch, a dashed frame inside it and a keyhole.
    .dm__secret {
      position: absolute;
      inset: 0;
      background:
        repeating-linear-gradient(45deg, color-mix(in srgb, var(--mr-map-token-ink) 70%, transparent) 0 2px, transparent 2px 6px),
        color-mix(in srgb, var(--mr-map-token-ink) 28%, transparent);
    }

    .dm__frame {
      position: absolute;
      inset: 16%;
      border: 1.5px dashed var(--mr-map-token-ink);
      background: color-mix(in srgb, var(--mr-map-token-surface) 78%, transparent);
    }

    .dm__key {
      inset: 22%;
      width: 56%;
      height: 56%;
      fill: var(--mr-map-token-ink);
    }
  `,
})
export class DoorMark {
  readonly state = input.required<DoorKind>();
  readonly axis = input<'h' | 'v'>('h');
}
