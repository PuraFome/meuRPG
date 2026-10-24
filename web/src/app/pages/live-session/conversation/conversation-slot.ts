import { type Signal, computed, signal } from '@angular/core';

import type { SessionTab } from './session-tabs';

/** What the slot reads from the session page; each one is a signal of the page. */
export interface SlotInputs {
  /** The session is live (the stream is open and the page is not an ended or blocked one). */
  readonly live: Signal<boolean>;
  /** Under 768 px: the phone's tabs. */
  readonly phone: Signal<boolean>;
  /** The browser tab is visible. */
  readonly pageVisible: Signal<boolean>;
  readonly isMaster: Signal<boolean>;
  readonly campaignId: Signal<string>;
  readonly ownCharacterId: Signal<string>;
  /** The reader's account. */
  readonly userId: Signal<string>;
  /** The character ids of the NPCs on the stage (the master gets them). */
  readonly stageCharacterIds: Signal<readonly string[]>;
  /** The lines that came while the conversation was not on screen. */
  readonly unread: Signal<number>;
  /** The conversation sits in the right rail beside "Ficha" (a player from 1024 px): the rail's other tab hides it. */
  readonly railed?: Signal<boolean>;
}

/**
 * Where the conversation sits in the session page (MR-051): a column beside the stage on a
 * computer, the "Conversa" tab on a phone. It decides when the panel is on screen (so unread
 * counts only what arrived while it was not), when its code is loaded at all (at once on a
 * computer; on a phone, when the tab is first opened), and where "Ficha" goes.
 */
export class ConversationSlot {
  readonly tab = signal<SessionTab>('scene');
  /** The rail's open tab, from 1024 px: "Conversa" until the reader opens "Ficha". */
  readonly railTab = signal('chat');
  private readonly opened = signal(false);

  /** The phone shows the conversation, full screen. */
  readonly chatOpen = computed(() => this.inputs.phone() && this.tab() === 'chat');
  /** The phone shows the character's panel inside the session. */
  readonly sheetOpen = computed(() => this.inputs.phone() && this.tab() === 'sheet');
  /** The reader can see the conversation now; the unread count waits for it to be false. */
  readonly onScreen = computed(
    () =>
      this.inputs.live() &&
      (this.inputs.phone()
        ? this.tab() === 'chat'
        : this.inputs.pageVisible() && !(this.inputs.railed?.() && this.railTab() === 'sheet')),
  );
  /** The count the notice at the top of the page shows: only on a phone, with the chat closed, in a live session; else 0. */
  readonly news = computed(() =>
    this.inputs.live() && this.inputs.phone() && this.tab() !== 'chat' ? this.inputs.unread() : 0,
  );
  /** The panel's code is loaded: the lazy block opens on it. */
  readonly wanted = computed(() => this.inputs.live() && (!this.inputs.phone() || this.opened()));
  readonly sheetLink = computed<readonly string[]>(() => {
    const { isMaster, campaignId, ownCharacterId } = this.inputs;
    return !isMaster() && ownCharacterId() !== ''
      ? ['/campaigns', campaignId(), 'characters', ownCharacterId()]
      : ['/campaigns', campaignId()];
  });
  /** The conversation is what the reader looks at (a phone's open tab, or the column on a computer): the voice stops when not. */
  readonly shown = computed(() =>
    this.inputs.phone()
      ? this.tab() === 'chat'
      : !(this.inputs.railed?.() && this.railTab() === 'sheet'),
  );
  readonly phone = computed(() => this.inputs.phone());
  readonly userId = computed(() => this.inputs.userId());
  readonly stageIds = computed<ReadonlySet<string>>(() => new Set(this.inputs.stageCharacterIds()));

  constructor(private readonly inputs: SlotInputs) {}

  choose(tab: SessionTab): void {
    if (tab === 'chat') {
      this.opened.set(true);
    }
    this.tab.set(tab);
  }
}
