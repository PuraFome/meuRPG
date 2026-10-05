import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import type { Map as MapMessage } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { focusWithRing } from '../../../core/creatures/focus-ring';
import { gridRows } from '../../../core/combat/combat-grid';
import { editorErrorMessage } from '../../../core/maps/map-errors';
import { MapsClient } from '../../../core/maps/maps-client';
import { MapAsk } from '../map-ask/map-ask';

/** The server takes 4 to 200 columns (`SetMapGrid`). */
const MIN = 4;
const MAX = 200;
const DEFAULT = 20;

/**
 * "Grade" (E9-01 1, 4, 5, 6): how many squares the map has and the one way to change it. Without a grid it asks
 * for the columns and "Definir a grade" (nothing to paint or light before it exists). With one, "Mudar a grade"
 * opens the question in place: the new columns (the rows follow the image's proportions), what it erases when
 * something is painted or seen (the layers and the players' memory; points and tokens stay), "Voltar" and the one
 * filled button, "Apagar e mudar a grade". With nothing to lose it does not warn and the button just says "Mudar a
 * grade". While a combat runs on the map the change is off, with the reason in a line (the server refuses it too).
 */
@Component({
  selector: 'app-grid-panel',
  imports: [MapAsk, MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './grid-panel.html',
  styleUrl: './grid-panel.scss',
})
export class GridPanel {
  private readonly api = inject(MapsClient);
  private readonly injector = inject(Injector);

  readonly campaignId = input.required<string>();
  readonly map = input.required<MapMessage>();
  /** Something is painted or seen: a new grid would erase it. */
  readonly erases = input(false);
  readonly combatRunning = input(false);

  /** The map as the server has it after the change. */
  readonly changed = output<MapMessage>();

  protected readonly asking = signal(false);
  protected readonly typed = signal(String(DEFAULT));
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  private readonly opener = viewChild('opener', { read: ElementRef<HTMLButtonElement> });
  private readonly field = viewChild('field', { read: ElementRef<HTMLInputElement> });

  protected readonly hasGrid = computed(() => this.map().gridColumns > 0);
  protected readonly image = computed(() => this.map().image);
  protected readonly columns = computed(() => {
    const t = this.typed().trim();
    const n = /^\d{1,3}$/.test(t) ? Number(t) : NaN;
    return n >= MIN && n <= MAX ? n : null;
  });
  protected readonly rows = computed(() => {
    const image = this.image();
    const columns = this.columns();
    return image && columns !== null ? gridRows(columns, image.width, image.height) : null;
  });
  protected readonly invalid = computed(() => this.columns() === null);
  protected readonly min = MIN;
  protected readonly max = MAX;

  /** Opens the question (a map with a grid), or just the field (without one). */
  protected start(): void {
    this.typed.set(String(this.hasGrid() ? this.map().gridColumns : DEFAULT));
    this.error.set('');
    this.asking.set(true);
  }

  protected cancel(): void {
    this.asking.set(false);
    this.error.set('');
    afterNextRender(() => focusWithRing(this.opener()?.nativeElement), { injector: this.injector });
  }

  protected onType(event: Event): void {
    this.typed.set((event.target as HTMLInputElement).value);
  }

  protected async confirm(): Promise<void> {
    const columns = this.columns();
    if (columns === null || this.busy()) {
      this.field()?.nativeElement.focus();
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      this.changed.emit(await this.api.setGrid(this.campaignId(), this.map().id, columns));
      this.asking.set(false);
      afterNextRender(() => focusWithRing(this.opener()?.nativeElement), { injector: this.injector });
    } catch (err) {
      this.error.set(editorErrorMessage(err, 'mudar a grade'));
    } finally {
      this.busy.set(false);
    }
  }
}
