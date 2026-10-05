import { Component, computed, input, output, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { CreatureArt } from './creature-art';

/** One creature of a choice list. */
export interface ChoiceRow {
  readonly key: string;
  /** The Portuguese name. */
  readonly title: string;
  /** The book's English name, shown in parentheses by the master's search. */
  readonly alias?: string;
  /** "Miúdo · 1 PV · 3 m, voo 15 m". */
  readonly subtitle: string;
  /** Shows the silhouette (the master's search); the forms of a spell are plain. */
  readonly art?: boolean;
}

let nextId = 0;

/**
 * A single-choice list of creatures (E9-10): native radios under the rows,
 * the chosen row with a frame AND a filled marker (never colour alone), the
 * name and a line of numbers. With `searchable`, a field above filters the
 * rows by name while typing (the forms of Encontrar Familiar); the master's
 * search filters on the server and brings the rows already filtered. The list
 * scrolls inside itself (`max-height`), so the sheet's title and buttons stay
 * in reach.
 */
@Component({
  selector: 'app-creature-choice-list',
  imports: [CreatureArt, MatIconModule],
  template: `
    @if (searchable()) {
      <label class="search">
        <mat-icon aria-hidden="true">search</mat-icon>
        <input
          type="search"
          autocomplete="off"
          [attr.aria-label]="searchLabel()"
          [placeholder]="searchLabel()"
          [value]="filter()"
          (input)="filter.set($any($event.target).value)"
        />
      </label>
    }
    <div class="rows" role="radiogroup" [attr.aria-label]="label()" [attr.aria-describedby]="empty() ? id + '-empty' : null">
      @for (r of shown(); track r.key) {
        <label class="row" [class.row--on]="r.key === chosen()">
          <input
            type="radio"
            class="mr-visually-hidden"
            [name]="id"
            [value]="r.key"
            [checked]="r.key === chosen()"
            (change)="pick.emit(r.key)"
          />
          <mat-icon class="row__mark" aria-hidden="true">{{ r.key === chosen() ? 'radio_button_checked' : 'radio_button_unchecked' }}</mat-icon>
          @if (r.art) {
            <app-creature-art [monsterKey]="r.key" />
          }
          <span class="row__text">
            <span class="row__title">{{ r.title }}@if (r.alias) { <span class="row__alias">({{ r.alias }})</span> }</span>
            <span class="row__sub">{{ r.subtitle }}</span>
          </span>
        </label>
      } @empty {
        <p class="empty" [id]="id + '-empty'">{{ emptyText() }}</p>
      }
    </div>
  `,
  styleUrl: './creature-choice-list.scss',
  host: { '[style.--list-max]': 'maxHeight()' },
})
export class CreatureChoiceList {
  readonly rows = input.required<readonly ChoiceRow[]>();
  readonly chosen = input('');
  /** The group's name for a screen reader: "Forma". */
  readonly label = input('Criatura');
  readonly searchable = input(false);
  readonly searchLabel = input('Buscar');
  readonly emptyText = input('Nenhuma criatura encontrada.');
  readonly maxHeight = input('none');
  readonly pick = output<string>();

  protected readonly id = `creature-choice-${nextId++}`;
  protected readonly filter = signal('');
  protected readonly shown = computed(() => {
    const needle = fold(this.filter());
    return needle ? this.rows().filter((r) => fold(`${r.title} ${r.alias ?? ''}`).includes(needle)) : this.rows();
  });
  protected readonly empty = computed(() => this.shown().length === 0);
}

/** Lower case without accents, so "aranha" finds "Aranha" and "cobra" finds "Cobra venenosa". */
function fold(text: string): string {
  return text.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase().trim();
}
