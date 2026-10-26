import { ChangeDetectionStrategy, Component, computed, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { MapFormation } from '../../../../gen/meurpg/maps/v1/maps_pb';
import type { MapToken } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { type MapLayers, NO_LAYERS } from '../../../core/maps/layers';
import { putPartyErrorMessage } from '../../../core/maps/map-errors';
import {
  type Grid,
  type Point,
  centerOf,
  formationRect,
  squareAt,
} from '../../../core/maps/token-move';
import { MapLayersOverlay } from '../../../shared/map-layers/map-layers';
import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet } from '../../../shared/sheet/sheet-host';

/** One character the master may put on the map: the account's character, and "Mago 5" under the name. */
export interface PartyMember {
  readonly id: string;
  readonly name: string;
  readonly sub: string;
}

export interface PutPartyData {
  readonly mapName: string;
  readonly members: readonly PartyMember[];
  readonly image: { readonly url: string; readonly width: number; readonly height: number } | null;
  readonly layers: MapLayers | null;
  readonly grid: Grid;
  /** The tokens on the map now, drawn small on the picture so the master sees where the group will land. */
  readonly tokens: readonly MapToken[];
  /** The square the corner starts on. */
  readonly start: Point;
  /** `PutPartyOnMap`: answers who had to go to a nearer square, or throws. */
  readonly put: (
    ids: readonly string[],
    formation: MapFormation,
    at: Point,
  ) => Promise<{ readonly movedForRoom: number }>;
}

const FORMATIONS: readonly { value: MapFormation; label: string; hint: string }[] = [
  { value: MapFormation.LINE, label: 'Em fila', hint: 'Lado a lado, à direita do primeiro.' },
  { value: MapFormation.BLOCK, label: 'Em bloco', hint: 'Juntos, em até 2 fileiras.' },
  { value: MapFormation.ARC, label: 'Em arco', hint: 'Num meio círculo ao redor do primeiro.' },
];

/**
 * "Pôr o grupo no mapa" (B2-05, B2-06): who comes (the characters of the table, all chosen to begin with), the
 * formation (Em fila, Em bloco, Em arco), and the square of the map where the group's corner goes, chosen on the map
 * itself. A dialog from a tablet up, a sheet on a phone (`openSheet`). The server places each one on its spot or on the
 * nearest free square; this only previews the rectangle a line or a block would take.
 */
@Component({
  selector: 'app-put-party-dialog',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MapLayersOverlay, MatIconModule, SheetFrame],
  template: `
    <app-sheet-frame
      title="Pôr o grupo no mapa"
      titleId="party-title"
      [subtitle]="data.mapName + ' · fora de combate'"
      [phone]="sheet.inSheet"
      (closed)="cancel()"
    >
      <div class="body">
        <section class="who" aria-labelledby="party-who">
          <div class="who__head">
            <h3 class="who__title" id="party-who">Quem vem</h3>
            <span class="who__count">{{ chosen().length }} de {{ data.members.length }}</span>
          </div>
          <ul class="who__list">
            @for (m of data.members; track m.id) {
              <li>
                <label class="who__row">
                  <input type="checkbox" [checked]="picked().has(m.id)" (change)="toggle(m.id)" />
                  <span class="who__mark" aria-hidden="true">{{ initial(m.name) }}</span>
                  <span class="who__text">
                    <span class="who__name">{{ m.name }}</span>
                    <span class="who__sub">{{ m.sub }}{{ picked().has(m.id) ? '' : ' · fora desta vez' }}</span>
                  </span>
                </label>
              </li>
            }
          </ul>
        </section>

        <section class="how">
          <p class="how__label" id="party-formation">Formação</p>
          <div class="seg" role="radiogroup" aria-labelledby="party-formation">
            @for (f of formations; track f.value) {
              <button
                type="button"
                role="radio"
                class="seg__item"
                [class.seg__item--on]="formation() === f.value"
                [attr.aria-checked]="formation() === f.value"
                (click)="formation.set(f.value)"
              >
                @if (formation() === f.value) {
                  <mat-icon aria-hidden="true">check</mat-icon>
                }
                {{ f.label }}
              </button>
            }
          </div>
          <p class="how__hint">{{ hint() }}</p>

          <p class="how__label" id="party-where">Onde ficam: toque num quadrado do mapa</p>
          @if (data.image) {
            <div
              class="pick"
              role="group"
              aria-labelledby="party-where"
              tabindex="0"
              [style.aspect-ratio]="data.image.width + ' / ' + data.image.height"
              [style.--cols]="data.grid.columns"
              [style.--rows]="data.grid.rows"
              (click)="tap($event)"
              (keydown)="key($event)"
            >
              <img class="pick__img" [src]="data.image.url" alt="" />
              <app-map-layers [layers]="layers()" />
              @for (t of data.tokens; track $index) {
                <span class="dot" aria-hidden="true" [style.left.%]="t.xBp / 100" [style.top.%]="t.yBp / 100"></span>
              }
              @if (rect(); as r) {
                <span
                  class="rect"
                  aria-hidden="true"
                  [style.left.%]="(r.col / data.grid.columns) * 100"
                  [style.top.%]="(r.row / data.grid.rows) * 100"
                  [style.width.%]="(r.cols / data.grid.columns) * 100"
                  [style.height.%]="(r.rows / data.grid.rows) * 100"
                ></span>
              } @else {
                <span
                  class="rect"
                  aria-hidden="true"
                  [style.left.%]="(at().col / data.grid.columns) * 100"
                  [style.top.%]="(at().row / data.grid.rows) * 100"
                  [style.width.%]="100 / data.grid.columns"
                  [style.height.%]="100 / data.grid.rows"
                ></span>
              }
            </div>
          }
          <p class="how__hint">
            O quadrado marcado é o canto de cima à esquerda do bloco. Quem não cabe vai para o quadrado livre mais perto.
          </p>
          <p class="mr-visually-hidden" role="status">{{ where() }}</p>
        </section>

        @if (error()) {
          <div class="mr-notice mr-notice--danger" role="alert">
            <mat-icon aria-hidden="true">error</mat-icon>
            <p>{{ error() }}</p>
          </div>
        }
      </div>
      <div foot class="foot">
        <button type="button" class="foot__button" (click)="cancel()">Cancelar</button>
        <button
          type="button"
          class="foot__button foot__button--primary"
          [attr.aria-disabled]="chosen().length === 0 || busy() ? 'true' : null"
          (click)="done()"
        >
          {{ label() }}
        </button>
      </div>
    </app-sheet-frame>
  `,
  styleUrl: './put-party-dialog.scss',
})
export class PutPartyDialog {
  protected readonly sheet = injectSheet<PutPartyData, boolean | undefined>();
  protected readonly data = this.sheet.data;
  protected readonly formations = FORMATIONS;
  protected readonly formation = signal(MapFormation.BLOCK);
  protected readonly picked = signal<ReadonlySet<string>>(
    new Set(this.data.members.map((m) => m.id)),
  );
  protected readonly at = signal(squareAt(this.data.start, this.data.grid));
  protected readonly busy = signal(false);
  protected readonly error = signal('');

  protected readonly layers = computed(() => this.data.layers ?? NO_LAYERS);
  /** The characters chosen, in the list's order: the first is the one the formation is built around. */
  protected readonly chosen = computed(() =>
    this.data.members.filter((m) => this.picked().has(m.id)),
  );
  protected readonly hint = computed(
    () => FORMATIONS.find((f) => f.value === this.formation())?.hint ?? '',
  );
  protected readonly rect = computed(() =>
    formationRect(this.formation(), this.chosen().length, this.at(), this.data.grid),
  );
  protected readonly label = computed(() => `Pôr ${this.chosen().length} no mapa`);
  protected readonly where = computed(() => {
    const r = this.rect() ?? { col: this.at().col, row: this.at().row };
    return `Canto de cima à esquerda: coluna ${r.col + 1}, linha ${r.row + 1}.`;
  });

  protected initial(name: string): string {
    return name.trim().charAt(0).toUpperCase();
  }

  protected toggle(id: string): void {
    this.picked.update((set) => {
      const next = new Set(set);
      if (!next.delete(id)) {
        next.add(id);
      }
      return next;
    });
  }

  /** A tap on the picture: the square it fell on. */
  protected tap(event: MouseEvent): void {
    const box = (event.currentTarget as HTMLElement).getBoundingClientRect();
    const col = Math.floor(((event.clientX - box.left) / box.width) * this.data.grid.columns);
    const row = Math.floor(((event.clientY - box.top) / box.height) * this.data.grid.rows);
    this.at.set(
      squareAt(centerOf(Math.max(0, col), Math.max(0, row), this.data.grid), this.data.grid),
    );
  }

  /** The arrows move the corner a square, for whoever does not use a pointer. */
  protected key(event: KeyboardEvent): void {
    const step: Record<string, [number, number]> = {
      ArrowLeft: [-1, 0],
      ArrowRight: [1, 0],
      ArrowUp: [0, -1],
      ArrowDown: [0, 1],
    };
    const d = step[event.key];
    if (!d) {
      return;
    }
    event.preventDefault();
    const { columns, rows } = this.data.grid;
    this.at.update((a) => ({
      col: Math.max(0, Math.min(columns - 1, a.col + d[0])),
      row: Math.max(0, Math.min(rows - 1, a.row + d[1])),
    }));
  }

  protected cancel(): void {
    this.sheet.close(undefined);
  }

  protected async done(): Promise<void> {
    if (this.chosen().length === 0 || this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const r = this.rect() ?? this.at();
      await this.data.put(
        this.chosen().map((m) => m.id),
        this.formation(),
        centerOf(r.col, r.row, this.data.grid),
      );
      this.sheet.close(true);
    } catch (err) {
      this.error.set(putPartyErrorMessage(err));
    } finally {
      this.busy.set(false);
    }
  }
}
