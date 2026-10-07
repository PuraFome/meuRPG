import { timestampDate } from '@bufbuild/protobuf/wkt';

import { type Note, NoteKind, type NoteScene } from '../../../gen/meurpg/notes/v1/notes_pb';
import { formatInt } from '../format/text';
import { formatClock } from '../../shared/session-time/session-time';

/** The filter's special choices: everything, and the notes with no scene. */
export const FILTER_ALL = 'all';
export const FILTER_NONE = 'none';

export const isClue = (note: Note): boolean => note.kind === NoteKind.CLUE;

/** "Hoje, 21:24", "Ontem, 21:24" or "01/10, 22:03" (with the year when it is
 * not this one), in local time. */
export function noteStamp(at: Date, now: Date = new Date()): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  const sameDay = (a: Date, b: Date) =>
    a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
  const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1);
  if (sameDay(at, now)) {
    return `Hoje, ${formatClock(at)}`;
  }
  if (sameDay(at, yesterday)) {
    return `Ontem, ${formatClock(at)}`;
  }
  const day = `${pad(at.getDate())}/${pad(at.getMonth() + 1)}`;
  return `${at.getFullYear() === now.getFullYear() ? day : `${day}/${at.getFullYear()}`}, ${formatClock(at)}`;
}

/** The time a note is placed by: when it was last written, or the clue arrived. */
export function noteTime(note: Note): Date {
  const at = note.updatedAt ?? note.createdAt;
  return at ? timestampDate(at) : new Date(0);
}

/** Newest first. */
export function sortNotes(notes: readonly Note[]): readonly Note[] {
  return [...notes].sort((a, b) => noteTime(b).getTime() - noteTime(a).getTime());
}

/** One choice of the scene filter, with how many entries it holds. */
export interface FilterOption {
  readonly value: string;
  readonly label: string;
  readonly count: number;
}

/** "Todas as anotações", each discovered scene, and "Sem cena", each with its
 * count. The scenes are only the discovered ones (a note can only carry
 * those), so an undiscovered scene's name appears nowhere. */
export function filterOptions(notes: readonly Note[], scenes: readonly NoteScene[]): readonly FilterOption[] {
  const count = (match: (n: Note) => boolean) => notes.filter(match).length;
  return [
    { value: FILTER_ALL, label: 'Todas as anotações', count: notes.length },
    ...scenes.map((s) => ({ value: s.id, label: s.name, count: count((n) => n.scenePointId === s.id) })),
    { value: FILTER_NONE, label: 'Sem cena', count: count((n) => n.scenePointId === '') },
  ];
}

/** The entries a filter choice shows (the order does not change). */
export function applyFilter(notes: readonly Note[], filter: string): readonly Note[] {
  if (filter === FILTER_ALL) {
    return notes;
  }
  return notes.filter((n) => (filter === FILTER_NONE ? n.scenePointId === '' : n.scenePointId === filter));
}

/** "2.000 de 2.000". */
export function noteCounter(length: number, max: number): string {
  return `${formatInt(length)} de ${formatInt(max)}`;
}

/** "4 anotações" (the clue counts here: it is in the list). */
export function entryCount(n: number): string {
  return `${n} ${n === 1 ? 'anotação' : 'anotações'}`;
}
