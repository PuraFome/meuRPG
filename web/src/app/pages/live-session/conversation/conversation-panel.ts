import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { Role } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  type SendSessionMessageRequest,
  type SessionMessage,
  SessionMessageSpeakerKind as Kind,
} from '../../../../gen/meurpg/play/v1/session_messages_pb';
import { CampaignsService } from '../../../core/campaigns/campaigns.service';
import type { ConversationState, PendingSend } from '../../../core/conversation/conversation-state';
import { ListenController } from '../../../core/conversation/listen-controller';
import { type LineView, viewLine } from '../../../core/conversation/message-view';
import { RosterClient } from '../../../core/maps/roster-client';
import type { StreamStatus } from '../../../core/play/live-stream';
import { newTail } from './announce';
import { ConversationLine } from './conversation-line';
import { ConversationRoster, type RosterSource } from './conversation-roster';
import { MasterComposer } from './master-composer';
import { PendingLine } from './pending-line';
import { PlayerComposer } from './player-composer';

type Row =
  | { readonly kind: 'line'; readonly key: string; readonly view: LineView }
  | {
      readonly kind: 'pending';
      readonly key: string;
      readonly pending: PendingSend;
      readonly label: string;
    };

/** How close to the end counts as "at the end": a reader this near follows the newest line. */
const STICK_PX = 48;
/** How many announced lines the hidden log keeps. */
const ANNOUNCED_KEPT = 5;

let nextId = 0;

/** The words above a line of the reader's own that is still going or failed. */
function pendingLabel(
  request: SendSessionMessageRequest,
  ownName: string,
  speakerName: (npcId: string) => string,
): string {
  switch (request.speakerKind) {
    case Kind.CHARACTER:
      return `${ownName} · você`;
    case Kind.OUT_OF_CHARACTER:
      return 'Você · fora do personagem';
    case Kind.NPC:
      return `${speakerName(request.speakerId)} · você`;
    default:
      return 'Narração';
  }
}

/**
 * The conversation of the scene (MR-051, RN-36): the header, the feed of lines, the notices (the
 * connection, a failed read), the master's typing hint and the composer of the reader's role. It
 * draws what `ConversationState` holds and sends through it; the lines are worded by `viewLine`.
 *
 * - On a computer it is a column beside the stage; on a phone (`phone`) it is the full-screen view
 *   of the "Conversa" tab with a back arrow.
 * - The feed follows the newest line unless the reader scrolled up.
 * - New lines are read aloud by a polite log that never repeats the history; the feed itself is
 *   not a live region.
 * - "Ouvir" is one `ListenController` for the panel: one line plays at a time, and it stops when
 *   the panel leaves the screen.
 */
@Component({
  selector: 'app-conversation',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ConversationLine, MasterComposer, MatIconModule, PendingLine, PlayerComposer],
  templateUrl: './conversation-panel.html',
  styleUrl: './conversation-panel.scss',
})
export class ConversationPanel {
  readonly state = input.required<ConversationState>();
  readonly campaignId = input.required<string>();
  readonly isMaster = input.required<boolean>();
  /** The reader's account, to tell their own lines from others'. */
  readonly userId = input.required<string>();
  readonly ownCharacterId = input.required<string>();
  readonly ownCharacterName = input.required<string>();
  /** The character ids of the NPCs on the stage, for "Falar como". */
  readonly stageIds = input.required<ReadonlySet<string>>();
  readonly sceneName = input('');
  readonly connection = input<StreamStatus>('live');
  /** The phone's full-screen view, with the back arrow. */
  readonly phone = input(false);
  /** The panel is on the reader's screen; off it, the voice stops. */
  readonly active = input(true);
  readonly back = output<void>();

  private readonly injector = inject(Injector);
  private readonly feed = viewChild.required<ElementRef<HTMLElement>>('feed');
  protected readonly titleId = `conversation-title-${nextId++}`;
  protected readonly listen = new ListenController();
  protected readonly roster = new ConversationRoster(
    this.rosterSource(),
    () => this.campaignId(),
    () => this.isMaster(),
    () => this.stageIds(),
  );
  protected readonly announced = signal<readonly { id: string; text: string }[]>([]);
  private stuck = true;

  private readonly viewer = computed(() => ({
    userId: this.userId(),
    isMaster: this.isMaster(),
  }));
  protected readonly rows = computed<readonly Row[]>(() =>
    this.state()
      .entries()
      .map((entry) =>
        entry.kind === 'line'
          ? {
              kind: 'line' as const,
              key: entry.message.id,
              view: viewLine(entry.message, this.viewer(), this.roster.directory()),
            }
          : {
              kind: 'pending' as const,
              key: `pending-${entry.pending.key}`,
              pending: entry.pending,
              label: this.labelOf(entry.pending.request),
            },
      ),
  );
  protected readonly subtitle = computed(() =>
    this.isMaster() ? 'Você é o mestre' : this.sceneName(),
  );

  constructor() {
    inject(DestroyRef).onDestroy(() => this.listen.stop());
    // The table is read when the panel opens and again each time the stream comes back.
    effect(() => {
      if (this.connection() === 'live') {
        untracked(() => void this.roster.load());
      }
    });
    effect(() => {
      if (!this.active()) {
        this.listen.stop();
      }
    });
    this.announceNews();
    this.followNewest();
  }

  /** Reads the reader's scroll: near the end, the feed keeps following; scrolled up, it stays put. */
  protected onScroll(): void {
    const el = this.feed().nativeElement;
    this.stuck = el.scrollHeight - el.scrollTop - el.clientHeight < STICK_PX;
  }

  private labelOf(request: SendSessionMessageRequest): string {
    return pendingLabel(
      request,
      this.ownCharacterName(),
      (npcId) => this.roster.speakers().find((s) => s.value === npcId)?.label ?? 'NPC',
    );
  }

  private rosterSource(): RosterSource {
    const roster = inject(RosterClient);
    const campaigns = inject(CampaignsService);
    return {
      roster: (id) => roster.list(id),
      members: async (id) =>
        (await campaigns.listMembers(id)).members.map((m) => ({
          userId: m.userId,
          name: m.displayName,
          master: m.role === Role.MASTER,
        })),
    };
  }

  /** A line that arrives at the end is told politely, once; the first read, older pages and the reader's own lines are not. */
  private announceNews(): void {
    let previous: readonly SessionMessage[] = [];
    let wasLoaded = false;
    effect(() => {
      const lines = this.state().lines();
      const loaded = this.state().loaded();
      untracked(() => {
        const fresh = wasLoaded ? (previous.length === 0 ? lines : newTail(previous, lines)) : [];
        previous = lines;
        wasLoaded = loaded;
        const heard = fresh
          .map((m) => viewLine(m, this.viewer(), this.roster.directory()))
          .filter((v) => !v.mine)
          .map((v) => ({ id: v.id, text: `${v.label}. ${v.text}` }));
        if (heard.length > 0) {
          this.announced.update((list) => [...list, ...heard].slice(-ANNOUNCED_KEPT));
        }
      });
    });
  }

  /** The feed goes to the newest line when the reader is at the end; sending a line brings them back to the end. */
  private followNewest(): void {
    let sending = 0;
    effect(() => {
      this.rows();
      const pending = this.state().pending().length;
      untracked(() => {
        if (pending > sending) {
          this.stuck = true;
        }
        sending = pending;
        if (this.stuck) {
          afterNextRender(
            () => {
              const el = this.feed().nativeElement;
              el.scrollTop = el.scrollHeight;
            },
            { injector: this.injector },
          );
        }
      });
    });
  }
}
