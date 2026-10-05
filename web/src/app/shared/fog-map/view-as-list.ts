import { Component, ElementRef, computed, inject, input, output, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { tight } from '../../core/format/text';

/** One character of the "Ver como" list: who it is and whose it is. */
export interface ViewAsPerson {
  readonly id: string;
  readonly name: string;
  /** The player's name ("Caio"), shown under the character's. */
  readonly sub: string;
}

/**
 * "Ver como" (MR-036, E9-03 state 5): the master's list of what each player's
 * character sees of the map — "Todos" (the master's own map, with no fog) and one
 * row for each character with how many squares they see ("22 quadrados vistos",
 * counted from the packed states, not worked out). Choosing a row shows the map
 * as that player gets it; only the master, and nothing changes for anyone else.
 * The session page and the map editor (9.12) use the same list.
 *
 * It is a radio group with one stop in the tab order: the arrows move between the
 * rows, Enter or Space chooses (a row is chosen on purpose, since each choice reads
 * the map again). "Visão do grupo" is the map's setting, said in words.
 */
@Component({
  selector: 'app-view-as-list',
  imports: [MatIconModule],
  templateUrl: './view-as-list.html',
  styleUrl: './view-as-list.scss',
})
export class ViewAsList {
  readonly people = input.required<readonly ViewAsPerson[]>();
  /** The squares each character sees; `null` for one whose read failed, absent while it is read. */
  readonly counts = input<ReadonlyMap<string, number | null>>(new Map());
  /** The map's squares ("de 384"). */
  readonly total = input(0);
  /** The character the master is looking as; `null` for "Todos". */
  readonly selected = input<string | null>(null);
  /** The map's "Visão do grupo" switch. */
  readonly groupVision = input(false);
  /** A line about the chosen view ("Toren vê 22 quadrados e nenhum inimigo além do Goblin 2"). */
  readonly note = input('');

  /** The master chose a row: a character's ID, or `null` for "Todos". */
  readonly choose = output<string | null>();

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  /** The row the arrows are on (the one stop of the tab order); `selected` until an arrow moves it. */
  private readonly cursor = signal<number | null>(null);

  protected readonly rows = computed(() => [
    { id: null as string | null, name: 'Todos', sub: 'Sem névoa: o seu mapa de mestre', count: null as string | null },
    ...this.people().map((p) => ({ id: p.id as string | null, name: p.name, sub: p.sub, count: this.countText(p.id) })),
  ]);
  protected readonly groupText = computed(() =>
    this.groupVision()
      ? 'Ligada neste mapa: cada jogador vê o que o grupo todo vê.'
      : 'Desligada neste mapa.',
  );

  private selectedIndex(): number {
    const id = this.selected();
    const i = this.rows().findIndex((r) => r.id === id);
    return i < 0 ? 0 : i;
  }

  protected stop(index: number): number {
    return (this.cursor() ?? this.selectedIndex()) === index ? 0 : -1;
  }

  private countText(id: string): string | null {
    const counts = this.counts();
    if (!counts.has(id)) {
      return null;
    }
    const n = counts.get(id);
    if (n === null || n === undefined) {
      return 'Não deu para ler';
    }
    return tight(`${n.toLocaleString('pt-BR')} ${n === 1 ? 'quadrado visto' : 'quadrados vistos'}`);
  }

  protected onKey(event: KeyboardEvent): void {
    const buttons = Array.from(this.host.nativeElement.querySelectorAll<HTMLElement>('[role="radio"]'));
    const here = buttons.findIndex((b) => b === event.target);
    if (here < 0) {
      return;
    }
    let next = here;
    switch (event.key) {
      case 'ArrowDown':
      case 'ArrowRight':
        next = (here + 1) % buttons.length;
        break;
      case 'ArrowUp':
      case 'ArrowLeft':
        next = (here - 1 + buttons.length) % buttons.length;
        break;
      case 'Home':
        next = 0;
        break;
      case 'End':
        next = buttons.length - 1;
        break;
      default:
        return;
    }
    event.preventDefault();
    this.cursor.set(next);
    buttons[next]?.focus();
  }

  protected pick(id: string | null, index: number): void {
    this.cursor.set(index);
    if (id !== this.selected()) {
      this.choose.emit(id);
    }
  }
}
