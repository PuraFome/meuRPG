import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import { type Note, NoteKind, NoteSchema, type NoteScene, NoteSceneSchema } from '../../../gen/meurpg/notes/v1/notes_pb';
import type { NotesList } from './notes-client';

/**
 * Builders and a stand-in for the notes specs (never imported by the app
 * itself, so never bundled): a note and a clue as the server sends them, a
 * discovered scene, and a `NotesClient` over an in-memory list that remembers
 * its calls. Always built with `create`, so a new proto field never leaves a
 * spec with a hand-built object missing it.
 */
export function note(
  id: string,
  text: string,
  at: Date,
  partial: { sceneId?: string; sceneName?: string; clue?: boolean } = {},
): Note {
  return create(NoteSchema, {
    id,
    kind: partial.clue ? NoteKind.CLUE : NoteKind.NOTE,
    text,
    scenePointId: partial.sceneId ?? '',
    sceneName: partial.sceneName ?? '',
    createdAt: timestampFromDate(at),
    updatedAt: timestampFromDate(at),
  });
}

export function scene(id: string, name: string): NoteScene {
  return create(NoteSceneSchema, { id, name });
}

/** A `NotesClient` that keeps notes in memory; `calls` records what was asked. */
export class FakeNotesClient {
  notes: Note[] = [];
  scenesList: NoteScene[] = [];
  maxNotes = 300;
  calls: string[] = [];
  /** Set to make the next call fail (once). */
  failWith: unknown = null;
  private next = 1;
  now = new Date(2026, 9, 3, 21, 30);

  private record(call: string): void {
    this.calls.push(call);
    if (this.failWith) {
      const err = this.failWith;
      this.failWith = null;
      throw err;
    }
  }

  async list(_campaignId: string): Promise<NotesList> {
    this.record('list');
    return {
      notes: this.notes,
      noteCount: this.notes.filter((n) => n.kind === NoteKind.NOTE).length,
      maxNotes: this.maxNotes,
    };
  }

  async scenes(_campaignId: string): Promise<readonly NoteScene[]> {
    this.record('scenes');
    return this.scenesList;
  }

  /** Makes the next `create` fail, as a lost answer does (the note is not added). */
  failNextCreate = false;
  /** The idempotency key of each `create`, in order. */
  readonly createKeys: string[] = [];

  async create(_campaignId: string, text: string, scenePointId: string, idempotencyKey: string): Promise<Note> {
    this.record(`create ${text} ${scenePointId}`);
    this.createKeys.push(idempotencyKey);
    if (this.failNextCreate) {
      this.failNextCreate = false;
      throw new Error('the answer was lost');
    }
    const sceneName = this.scenesList.find((s) => s.id === scenePointId)?.name ?? '';
    const created = note(`n${this.next++}`, text, this.now, { sceneId: scenePointId, sceneName });
    this.notes = [created, ...this.notes];
    return created;
  }

  async update(
    _campaignId: string,
    noteId: string,
    changes: { text?: string; scenePointId?: string },
  ): Promise<Note> {
    this.record(`update ${noteId} ${JSON.stringify(changes)}`);
    const old = this.notes.find((n) => n.id === noteId)!;
    const sceneId = changes.scenePointId ?? old.scenePointId;
    const updated = note(noteId, changes.text ?? old.text, this.now, {
      sceneId,
      sceneName: this.scenesList.find((s) => s.id === sceneId)?.name ?? '',
    });
    this.notes = this.notes.map((n) => (n.id === noteId ? updated : n));
    return updated;
  }

  async delete(_campaignId: string, noteId: string): Promise<void> {
    this.record(`delete ${noteId}`);
    this.notes = this.notes.filter((n) => n.id !== noteId);
  }
}
