import { computed, signal } from '@angular/core';

import type {
  ListCharactersInTheDarkResponse,
  MapToken,
} from '../../../gen/meurpg/maps/v1/maps_pb';
import {
  type DarkCharacter,
  type DarkSnapshot,
  darkAnnouncements,
  darkCharacters,
  darkSnapshot,
} from './in-the-dark';

function sameList(a: readonly DarkCharacter[], b: readonly DarkCharacter[]): boolean {
  return (
    a.length === b.length &&
    a.every(
      (c, i) =>
        c.characterId === b[i].characterId &&
        c.name === b[i].name &&
        c.col === b[i].col &&
        c.row === b[i].row,
    )
  );
}

/**
 * Who sees nothing where they stand on the map the master has open (`ListCharactersInTheDark`, master only: RN-10). The
 * page asks it to read again on the hints that can change it; a burst is one read after the one in the air (the
 * newest answer wins). `target` gives the map to read, or `null` where the warning does not apply (a player, no map,
 * the fog off): then nothing is asked at all. A read that fails shows nothing: not an error, and not a guess that nobody
 * is in the dark. It also says, in `announcement`, what changed between two reads: a light that went out, a character
 * who left the dark with a light.
 */
export class DarkWatch {
  readonly characters = signal<readonly DarkCharacter[]>([]);
  readonly ids = computed<ReadonlySet<string>>(
    () => new Set(this.characters().map((c) => c.characterId)),
  );
  /** The last status message ("A tocha de bilbo apagou. ..."); empty until something changes and when the map changes. */
  readonly announcement = signal('');

  private pump: Promise<void> | null = null;
  private again = false;
  private mapId: string | null = null;
  private seen: DarkSnapshot | null = null;

  constructor(
    private readonly load: (mapId: string) => Promise<ListCharactersInTheDarkResponse>,
    private readonly target: () => string | null,
    private readonly tokens: () => readonly MapToken[],
  ) {}

  /** Reads again; resolves when the newest read is on screen. */
  refresh(): Promise<void> {
    this.again = true;
    this.pump ??= this.drain().finally(() => (this.pump = null));
    return this.pump;
  }

  private async drain(): Promise<void> {
    while (this.again) {
      this.again = false;
      await this.readOnce();
    }
  }

  private async readOnce(): Promise<void> {
    const mapId = this.target();
    if (mapId !== this.mapId) {
      this.mapId = mapId;
      this.seen = null;
      this.announcement.set('');
    }
    if (mapId === null) {
      this.set([]);
      return;
    }
    try {
      const res = await this.load(mapId);
      // A hint came while this read was out: its answer is stale; the next read replaces it.
      if (!this.again) {
        this.set(darkCharacters(res));
      }
    } catch {
      // Nothing is claimed, so nothing is compared with it either.
      this.seen = null;
      this.show([]);
    }
  }

  private show(next: readonly DarkCharacter[]): void {
    if (!sameList(this.characters(), next)) {
      this.characters.set(next);
    }
  }

  private set(next: readonly DarkCharacter[]): void {
    this.show(next);
    const snapshot = darkSnapshot(this.tokens(), next);
    const said = darkAnnouncements(this.seen, snapshot);
    this.seen = snapshot;
    if (said.length > 0) {
      this.announcement.set(said.join(' '));
    }
  }
}
