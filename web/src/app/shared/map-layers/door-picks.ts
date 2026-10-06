import { ChangeDetectionStrategy, Component, ElementRef, inject, input, output, signal } from '@angular/core';

import type { DoorSquare, MapLayers } from '../../core/maps/layers';
import { DOOR_NAME } from './door-mark';

/**
 * The master's way to a door in the session (E10-05 10, 11): one button over each door of the map, so a tap, Enter or Space opens the door's
 * sheet (`DoorSheet`). The doors are drawn by the map's own layers (`app-map-layers`); these buttons are transparent and only catch the
 * tap, at least 44 px wide on the screen even when the square is smaller. The doors are one tab stop: Tab enters the map's doors once and
 * the arrow keys (Right and Down, Left and Up) walk them in reading order, each named by its kind and its place ("Porta fechada, coluna 5,
 * linha 3"); Enter or Space opens the sheet. It sits in a surface that sets `--cols` (the map view's stage, the combat map).
 */
@Component({
  selector: 'app-door-picks',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @for (d of doors(); track d.row * 1000 + d.col) {
      <button
        type="button"
        class="dp"
        [style.left.%]="((d.col + 0.5) / columns()) * 100"
        [style.top.%]="((d.row + 0.5) / rows()) * 100"
        [attr.aria-label]="label(d)"
        [attr.data-door]="d.col + ',' + d.row"
        [tabindex]="$index === tabStop() ? 0 : -1"
        (focus)="active.set($index)"
        (keydown)="onKey($event, $index)"
        (click)="pick.emit(d)"
        (pointerdown)="$event.stopPropagation()"
      ></button>
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

    .dp {
      position: absolute;
      width: max(calc(100% / var(--cols)), 44px);
      aspect-ratio: 1;
      padding: 0;
      transform: translate(-50%, -50%);
      border: 0;
      border-radius: 4px;
      background: transparent;
      cursor: pointer;
      pointer-events: auto;
    }

    .dp:hover {
      background: color-mix(in srgb, var(--mr-map-accent) 14%, transparent);
    }

    .dp:focus-visible {
      outline: 3px solid var(--mr-focus);
      outline-offset: 1px;
    }
  `,
})
export class DoorPicks {
  readonly layers = input.required<MapLayers>();
  readonly columns = input.required<number>();
  readonly rows = input.required<number>();

  readonly pick = output<DoorSquare>();

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  /** The door that holds the tab stop (the last one focused). */
  protected readonly active = signal(0);

  protected tabStop(): number {
    return Math.min(this.active(), Math.max(0, this.doors().length - 1));
  }

  protected onKey(event: KeyboardEvent, index: number): void {
    const step = event.key === 'ArrowRight' || event.key === 'ArrowDown' ? 1 : event.key === 'ArrowLeft' || event.key === 'ArrowUp' ? -1 : 0;
    if (step === 0) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    const buttons = Array.from(this.host.nativeElement.querySelectorAll<HTMLElement>('button'));
    buttons[Math.min(buttons.length - 1, Math.max(0, index + step))]?.focus();
  }

  protected doors(): readonly DoorSquare[] {
    return this.layers().doors ?? [];
  }

  protected label(d: DoorSquare): string {
    return `${DOOR_NAME[d.state]}, coluna ${d.col + 1}, linha ${d.row + 1}. Abrir as opções da porta`;
  }
}
