import { computed, signal } from '@angular/core';

import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import type { PlayerDirectory } from '../../../core/conversation/message-view';
import type { RosterEntry } from '../../../core/maps/roster-client';

/** A player's character the master may choose as a reader. */
export interface ChooserPlayer {
  readonly userId: string;
  readonly name: string;
  /** Class and race, small and muted under the name. */
  readonly sub: string;
}

/** One choice of "Falar como": `value` is the NPC's character id, empty for the narrator. */
export interface Speaker {
  readonly value: string;
  readonly label: string;
  readonly onStage: boolean;
}

export interface MemberName {
  readonly userId: string;
  readonly name: string;
  readonly master: boolean;
}

/** What the roster reads; the panel builds it from the clients, a spec hands a fake. */
export interface RosterSource {
  roster(campaignId: string): Promise<readonly RosterEntry[]>;
  members(campaignId: string): Promise<readonly MemberName[]>;
}

const NARRATOR: Speaker = { value: '', label: 'Narrador', onStage: false };

const isPlayerCharacter = (e: RosterEntry) => e.kind === CharacterKind.PLAYER;

export function chooserPlayers(entries: readonly RosterEntry[]): ChooserPlayer[] {
  return entries
    .filter((e) => isPlayerCharacter(e) && e.playerUserId !== '')
    .map((e) => ({
      userId: e.playerUserId,
      name: e.name,
      sub: [e.classSummary, e.raceName].filter(Boolean).join(' · '),
    }));
}

/** The narrator, the NPCs on the stage, then the other NPCs by name. */
export function speakersOf(
  entries: readonly RosterEntry[],
  stageIds: ReadonlySet<string>,
): Speaker[] {
  const npcs = entries
    .filter((e) => !isPlayerCharacter(e) && e.kind !== CharacterKind.UNSPECIFIED)
    .map((e) => ({ value: e.id, label: e.name, onStage: stageIds.has(e.id) }));
  const byName = (a: Speaker, b: Speaker) => a.label.localeCompare(b.label, 'pt-BR');
  return [
    NARRATOR,
    ...npcs.filter((s) => s.onStage).sort(byName),
    ...npcs.filter((s) => !s.onStage).sort(byName),
  ];
}

/** An account by its character's name, else by the name the person chose. */
export function directoryOf(
  entries: readonly RosterEntry[],
  members: readonly MemberName[],
): PlayerDirectory {
  const names = new Map<string, string>();
  for (const m of members) {
    if (m.name) {
      names.set(m.userId, m.name);
    }
  }
  for (const e of entries) {
    if (isPlayerCharacter(e) && e.playerUserId) {
      names.set(e.playerUserId, e.name);
    }
  }
  return names;
}

/**
 * Who sits at the table, as the conversation panel needs it: the count in its
 * header (everyone), and, for the master only, the players a line can be
 * limited to, the characters one can speak as and the names of the readers of a
 * line. A player is never given the roster: the characters of others are not
 * theirs to list (RN-10).
 */
export class ConversationRoster {
  private readonly entries = signal<readonly RosterEntry[]>([]);
  private readonly members = signal<readonly MemberName[]>([]);
  private readonly loadedMembers = signal(false);

  readonly players = computed(() => chooserPlayers(this.entries()));
  readonly speakers = computed(() => speakersOf(this.entries(), this.stageIds()));
  readonly directory = computed(() => directoryOf(this.entries(), this.members()));
  /** The players at the table; `null` until the members were read. */
  readonly tableSize = computed(() =>
    this.loadedMembers() ? this.members().filter((m) => !m.master).length : null,
  );

  constructor(
    private readonly source: RosterSource,
    private readonly campaignId: () => string,
    private readonly isMaster: () => boolean,
    private readonly stageIds: () => ReadonlySet<string>,
  ) {}

  async load(): Promise<void> {
    const id = this.campaignId();
    const [entries, members] = await Promise.all([
      this.isMaster() ? this.source.roster(id).catch(() => []) : Promise.resolve([]),
      this.source.members(id).catch(() => null),
    ]);
    this.entries.set(entries);
    if (members) {
      this.members.set(members);
      this.loadedMembers.set(true);
    }
  }
}
