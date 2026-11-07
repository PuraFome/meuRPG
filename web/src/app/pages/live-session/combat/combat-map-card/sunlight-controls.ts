import { ChangeDetectionStrategy, Component, computed, input, model, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { SunlightScope } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { type PaintTool, paintStatus } from '../../../../core/combat/sunlight-paint';

/**
 * The master's sunlight controls over the map (#704), shown only while the sun is on and the combat has a map: "Todo o mapa" or
 * "Onde eu pintar", and, for the second, "Pintar", which opens paint mode: "Pintar" / "Apagar" (the tool), "Pronto" (out) and the
 * count of lit squares. The map card draws the squares; this only asks. Every button is 44px.
 */
@Component({
  selector: 'app-sunlight-controls',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="sc__scope" role="radiogroup" aria-label="Onde a luz do sol vale">
      <button type="button" role="radio" class="sc__btn" [attr.aria-checked]="!squares()" [class.sc__btn--on]="!squares()" [disabled]="painting()" (click)="scopeChange.emit(map)">
        Todo o mapa
      </button>
      <button type="button" role="radio" class="sc__btn" [attr.aria-checked]="squares()" [class.sc__btn--on]="squares()" [disabled]="painting()" (click)="scopeChange.emit(spots)">
        Onde eu pintar
      </button>
    </div>
    @if (squares()) {
      @if (painting()) {
        <div class="sc__tools" role="group" aria-label="Ferramenta de pintura">
          <button type="button" class="sc__btn" [class.sc__btn--on]="tool() === 'paint'" [attr.aria-pressed]="tool() === 'paint'" aria-label="Pintar quadrados de luz do sol" (click)="tool.set('paint')">
            <mat-icon aria-hidden="true">brush</mat-icon>Pintar
          </button>
          <button type="button" class="sc__btn" [class.sc__btn--on]="tool() === 'erase'" [attr.aria-pressed]="tool() === 'erase'" aria-label="Apagar quadrados de luz do sol" (click)="tool.set('erase')">
            <mat-icon aria-hidden="true">ink_eraser</mat-icon>Apagar
          </button>
          <button type="button" class="sc__btn sc__done" aria-label="Pronto: sair da pintura da luz do sol" (click)="painting.set(false)">
            <mat-icon aria-hidden="true">check</mat-icon>Pronto
          </button>
        </div>
        <p class="sc__status" role="status">{{ status() }}</p>
      } @else {
        <button type="button" class="sc__btn sc__start" aria-label="Pintar luz do sol no mapa" (click)="painting.set(true)">
          <mat-icon aria-hidden="true">brush</mat-icon>Pintar
        </button>
      }
    }
  `,
  styles: `
    :host {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: var(--mr-space-2) var(--mr-space-4);
    }
    .sc__scope,
    .sc__tools {
      display: flex;
      flex-wrap: wrap;
      gap: var(--mr-space-2);
    }
    .sc__btn {
      display: inline-flex;
      align-items: center;
      gap: var(--mr-space-1);
      box-sizing: border-box;
      min-width: 44px;
      min-height: 44px;
      padding: 0 var(--mr-space-4);
      border: 1px solid var(--mr-control-line);
      border-radius: var(--mr-radius-pill);
      background: var(--mr-surface);
      color: var(--mr-ink);
      font: inherit;
      font-weight: 700;
      cursor: pointer;
    }
    .sc__btn--on {
      border-color: var(--mr-ink);
      box-shadow: inset 0 0 0 1px var(--mr-ink);
    }
    .sc__btn:disabled {
      cursor: default;
      opacity: 0.6;
    }
    .sc__btn:focus-visible {
      outline: 3px solid var(--mr-focus, currentColor);
      outline-offset: 2px;
    }
    .sc__status {
      flex: 1 0 100%;
      margin: 0;
      font-size: var(--mr-font-14);
      color: var(--mr-ink-muted);
    }
  `,
})
export class SunlightControls {
  readonly scope = input.required<SunlightScope>();
  /** Squares lit now (the server's), for the status line. */
  readonly count = input(0);
  readonly painting = model(false);
  readonly tool = model<PaintTool>('paint');
  /** "Todo o mapa" or "Onde eu pintar" was picked. */
  readonly scopeChange = output<SunlightScope>();

  protected readonly map = SunlightScope.MAP;
  protected readonly spots = SunlightScope.SQUARES;
  protected readonly squares = computed(() => this.scope() === SunlightScope.SQUARES);
  protected readonly status = computed(() => paintStatus(this.count()));
}
