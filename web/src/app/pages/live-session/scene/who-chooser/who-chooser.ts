import { ChangeDetectionStrategy, Component, computed, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { ChoosablePlayer } from '../../../../core/play/scene-audience';
import { SheetFrame } from '../../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet } from '../../../../shared/sheet/sheet-host';

export interface WhoData {
  /** "Quem está na cena?" or "Quem vê “Vista do salão”?". */
  readonly title: string;
  readonly subtitle: string;
  readonly players: readonly ChoosablePlayer[];
  /** The accounts chosen now; every account is the whole table. */
  readonly selected: readonly string[];
  /** The live line, from the names chosen (`everyone` when all are). */
  readonly line: (names: readonly string[], everyone: boolean) => string;
}

/** What "Pronto" answers: the whole table, or the accounts chosen in the table's order. */
export interface WhoChoice {
  readonly everyone: boolean;
  readonly userIds: readonly string[];
}

/**
 * "Quem está na cena?" and "Quem vê “<imagem>”?" (C3-01, C6-11): the master's chooser of some of the
 * table's players, a bottom sheet on a phone and a dialog from a tablet up (`openSheet`). "Todos" is
 * the first row (a tri-state checkbox), then one row per player with the character under the name;
 * the live line says in words who is chosen. "Cancelar" keeps what was; "Pronto" answers, and stays
 * dashed while nobody is chosen.
 */
@Component({
  selector: 'app-who-chooser',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule, SheetFrame],
  template: `
    <app-sheet-frame
      [title]="data.title"
      titleId="who-title"
      [subtitle]="data.subtitle"
      [phone]="sheet.inSheet"
      [handle]="false"
      [small]="sheet.inSheet"
      (closed)="cancel()"
    >
      <fieldset class="group">
        <legend class="mr-visually-hidden">Escolha as pessoas</legend>
        <label class="person">
          <input
            type="checkbox"
            [checked]="all()"
            [indeterminate]="some()"
            (change)="toggleAll()"
          />
          <span class="person__mark"><mat-icon aria-hidden="true">groups</mat-icon></span>
          <span class="person__text">
            <span class="person__name">Todos</span>
            <span class="person__sub">Toda a mesa</span>
          </span>
        </label>
        @for (p of players; track p.userId) {
          <label class="person">
            <input type="checkbox" [checked]="checked().has(p.userId)" (change)="toggle(p.userId)" />
            <span class="person__mark" aria-hidden="true">{{ initial(p.name) }}</span>
            <span class="person__text">
              <span class="person__name">{{ p.name }}</span>
              <span class="person__sub">{{ p.sub }}</span>
            </span>
          </label>
        }
      </fieldset>
      <p class="live" role="status"><mat-icon aria-hidden="true">visibility</mat-icon>{{ line() }}</p>
      <div foot class="foot">
        <button type="button" class="foot__button" (click)="cancel()">
          <mat-icon aria-hidden="true">close</mat-icon>Cancelar
        </button>
        <button
          type="button"
          class="foot__button foot__button--primary"
          [attr.aria-disabled]="empty() ? 'true' : null"
          (click)="done()"
        >
          <mat-icon aria-hidden="true">check</mat-icon>Pronto
        </button>
      </div>
    </app-sheet-frame>
  `,
  styleUrl: './who-chooser.scss',
})
export class WhoChooser {
  protected readonly sheet = injectSheet<WhoData, WhoChoice | undefined>();
  protected readonly data = this.sheet.data;
  protected readonly players = this.data.players;
  protected readonly checked = signal<ReadonlySet<string>>(new Set(this.data.selected));

  /** The chosen accounts in the table's order. */
  private readonly chosen = computed(() =>
    this.players.filter((p) => this.checked().has(p.userId)),
  );
  protected readonly all = computed(
    () => this.players.length > 0 && this.chosen().length === this.players.length,
  );
  protected readonly some = computed(() => this.chosen().length > 0 && !this.all());
  protected readonly empty = computed(() => this.chosen().length === 0);
  protected readonly line = computed(() =>
    this.data.line(
      this.chosen().map((p) => p.name),
      this.all(),
    ),
  );

  protected initial(name: string): string {
    return name.trim().charAt(0).toUpperCase();
  }

  protected toggle(userId: string): void {
    this.checked.update((set) => {
      const next = new Set(set);
      if (!next.delete(userId)) {
        next.add(userId);
      }
      return next;
    });
  }

  protected toggleAll(): void {
    this.checked.set(this.all() ? new Set() : new Set(this.players.map((p) => p.userId)));
  }

  protected cancel(): void {
    this.sheet.close(undefined);
  }

  protected done(): void {
    if (this.empty()) {
      return;
    }
    this.sheet.close({ everyone: this.all(), userIds: this.chosen().map((p) => p.userId) });
  }
}
