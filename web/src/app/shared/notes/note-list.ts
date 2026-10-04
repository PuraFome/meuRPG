import { Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Note } from '../../../gen/meurpg/notes/v1/notes_pb';
import { isClue, noteStamp, noteTime } from '../../core/notes/notes-view';

/** The label of a pencil: the start of the note's own words. */
function snippet(text: string): string {
  const flat = text.replace(/\s+/g, ' ').trim();
  return flat.length > 60 ? `${flat.slice(0, 59)}…` : flat;
}

/**
 * The list of the player's notes and of the clues the master revealed to them
 * (E8-06, E8-07), newest first. A note shows its words, its scene (an icon and
 * the name, or "Sem cena") and when it was written; the pencil (44px, named
 * with the note's own words) opens it for editing. A clue is the master's
 * text: it carries "Pista do mestre" (a magnifier and words), "Só leitura"
 * and a lock where the pencil would be, and cannot be edited or deleted.
 */
@Component({
  selector: 'app-note-list',
  imports: [MatIconModule],
  template: `
    <ul class="nl" [attr.aria-label]="label()">
      @for (r of rows(); track r.note.id) {
        <li class="nl__row" [class.nl__row--clue]="r.clue">
          <div class="nl__main">
            @if (r.clue) {
              <span class="nl__tag"><mat-icon aria-hidden="true">search</mat-icon>Pista do mestre</span>
            }
            <p class="nl__text">{{ r.note.text }}</p>
            <p class="nl__meta">
              @if (r.note.sceneName) {
                <span class="nl__scene"><mat-icon aria-hidden="true">chat_bubble_outline</mat-icon>{{ r.note.sceneName }}</span>
              } @else {
                <span class="nl__scene">Sem cena</span>
              }
              <span>{{ r.stamp }}</span>
              @if (r.clue) {
                <span>Só leitura</span>
              }
            </p>
          </div>
          @if (r.clue) {
            <span class="nl__lock" role="img" aria-label="Só leitura: a pista é do mestre">
              <mat-icon aria-hidden="true">lock</mat-icon>
            </span>
          } @else {
            <button
              type="button"
              class="nl__edit"
              [attr.aria-label]="'Editar a anotação: ' + r.label"
              [attr.data-note]="r.note.id"
              (click)="edit.emit(r.note)"
            >
              <mat-icon aria-hidden="true">edit</mat-icon>
            </button>
          }
        </li>
      }
    </ul>
  `,
  styleUrl: './note-list.scss',
})
export class NoteList {
  readonly notes = input.required<readonly Note[]>();
  /** The list's name for a screen reader. */
  readonly label = input('Anotações');
  /** The pencil of a note was pressed. */
  readonly edit = output<Note>();

  protected readonly rows = computed(() =>
    this.notes().map((note) => ({
      note,
      clue: isClue(note),
      stamp: noteStamp(noteTime(note)),
      label: snippet(note.text),
    })),
  );
}
