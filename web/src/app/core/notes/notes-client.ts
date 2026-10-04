import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import {
  type Note,
  type NoteScene,
  NotesService,
} from '../../../gen/meurpg/notes/v1/notes_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** The player's whole list: their notes and the clues the master revealed. */
export interface NotesList {
  /** Newest first. */
  readonly notes: readonly Note[];
  /** How many notes the player has (a clue is not counted). */
  readonly noteCount: number;
  /** The most notes a player may have: 300. */
  readonly maxNotes: number;
}

/**
 * Thin wrapper around the generated `NotesService` client (MR-030, RN-20),
 * in the same shape as `MapsClient`. `providedIn: 'root'`, imported only by
 * lazy code (the session page and the character sheet). A call by the
 * master answers `not_found`: the master has no notes. Callers map errors
 * (`notes-errors.ts`). Tests replace it with `{ provide: NotesClient, useValue }`.
 */
@Injectable({ providedIn: 'root' })
export class NotesClient {
  private readonly client = createClient(NotesService, inject(CONNECT_TRANSPORT));

  /** `ListNotes`: the caller's notes and received clues. The filter by scene
   * is the screen's own (it needs the counts of all of them). */
  async list(campaignId: string): Promise<NotesList> {
    const res = await this.client.listNotes({ campaignId });
    return { notes: res.notes, noteCount: res.noteCount, maxNotes: res.maxNotes };
  }

  /** `CreateNote`; an empty `scenePointId` is a note with no scene. */
  async create(campaignId: string, text: string, scenePointId: string): Promise<Note> {
    const res = await this.client.createNote({ campaignId, text, scenePointId });
    return need(res.note, 'CreateNote');
  }

  /** `UpdateNote`: only the fields given change; `scenePointId: ''` removes the tag. */
  async update(
    campaignId: string,
    noteId: string,
    changes: { text?: string; scenePointId?: string },
  ): Promise<Note> {
    const res = await this.client.updateNote({ campaignId, noteId, ...changes });
    return need(res.note, 'UpdateNote');
  }

  async delete(campaignId: string, noteId: string): Promise<void> {
    await this.client.deleteNote({ campaignId, noteId });
  }

  /** `ListNoteScenes`: the scenes the group discovered, the tag picker's list. */
  async scenes(campaignId: string): Promise<readonly NoteScene[]> {
    return (await this.client.listNoteScenes({ campaignId })).scenes;
  }
}

function need<T>(value: T | undefined, call: string): T {
  if (value === undefined) {
    throw new Error(`${call} answered without its result`);
  }
  return value;
}
