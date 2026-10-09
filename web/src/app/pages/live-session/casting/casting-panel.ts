import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  signal,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { type OutsideCast, OutsideCastStatus } from '../../../../gen/meurpg/play/v1/casting_pb';
import {
  ACTIVE_NOTE,
  activeChip,
  isActive,
  isCasting,
  logLine,
  minutesWords,
  QUEUE_NOTE_AFTER,
  QUEUE_NOTE_BEFORE,
  QUEUE_NOTE_STRONG,
  queueLine,
  queueTitle,
} from '../../../core/casting/cast-out-flow';
import { RosterClient } from '../../../core/maps/roster-client';
import { CastingClient } from '../../../core/casting/casting-client';
import { castingErrorText } from '../../../core/casting/casting-errors';
import { ActionKey } from '../../../core/connect/idempotency';
import { openCastOut } from './cast-out-sheet';
import { openCastConfirm } from './confirm-sheet';

/** How many lines of the log the panel shows. */
const LOG_LINES = 8;

/**
 * The casts outside a combat on the session page (MR-048). For a player: "Conjurar", the cast going on ("Conjurando", with
 * "Parar a conjuração"), the spells that last with "Encerrar", and the log of what the table cast. For the master: the
 * queue "Conjurações em andamento" with "Concluir conjuração", every spell that lasts, the log, and "Conjurar como NPC".
 * What a viewer reads is what the server sends them (RN-10): the panel filters nothing. It reads the casts again on every
 * `spell_casts_changed` hint (`tick`); the idempotency key of each action is kept while the same action is retried.
 */
@Component({
  selector: 'app-casting-panel',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './casting-panel.html',
  styleUrl: './casting-panel.scss',
})
export class CastingPanel {
  private readonly api = inject(CastingClient);
  private readonly roster = inject(RosterClient);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);

  readonly campaignId = input.required<string>();
  /** The player's own character; empty for the master, who casts for any. */
  readonly characterId = input('');
  readonly characterName = input('');
  readonly master = input(false);
  readonly tick = input(0);
  readonly diceMode = input(DiceMode.UNSPECIFIED);
  readonly preference = input(DicePreference.UNSPECIFIED);

  protected readonly active = signal<readonly OutsideCast[]>([]);
  protected readonly log = signal<readonly OutsideCast[]>([]);
  protected readonly loaded = signal(false);
  protected readonly error = signal('');
  protected readonly busyId = signal('');
  /** A typed sum of physical dice, by cast, for a "Concluir conjuração" that rolls. */
  protected readonly typed = signal<Readonly<Record<string, string | undefined>>>({});
  private readonly keys = new Map<string, ActionKey>();

  protected readonly physical = computed(() => this.diceMode() === DiceMode.PHYSICAL);
  protected readonly note = ACTIVE_NOTE;
  protected readonly going = computed(() => this.active().filter(isCasting));
  protected readonly lasting = computed(() => this.active().filter(isActive));
  protected readonly lines = computed(() =>
    this.log()
      .slice(0, LOG_LINES)
      .map((c) => ({ id: c.id, text: logLine(c), failed: c.status === OutsideCastStatus.FAILED })),
  );

  protected readonly reload = effect(() => {
    this.tick();
    void this.load();
  });

  protected chip = activeChip;
  protected minutesWords = minutesWords;
  protected queueTitle = queueTitle;
  protected queueLine = queueLine;
  protected readonly noteBefore = QUEUE_NOTE_BEFORE;
  protected readonly noteStrong = QUEUE_NOTE_STRONG;
  protected readonly noteAfter = QUEUE_NOTE_AFTER;

  private keyOf(what: string): ActionKey {
    let k = this.keys.get(what);
    if (!k) {
      k = new ActionKey();
      this.keys.set(what, k);
    }
    return k;
  }

  private async load(): Promise<void> {
    try {
      const res = await this.api.list(this.campaignId(), this.master() ? '' : this.characterId());
      this.active.set(res.active);
      this.log.set(res.log);
      this.error.set('');
    } catch (err) {
      this.error.set(castingErrorText(err));
    } finally {
      this.loaded.set(true);
    }
  }

  protected async open(npc = false): Promise<void> {
    let npcCasters: { id: string; name: string; detail: string }[] | undefined;
    if (npc) {
      try {
        // The NPCs with a full sheet (an enemy or a boss) are the ones that cast.
        const all = await this.roster.list(this.campaignId());
        npcCasters = all
          .filter((c) => c.kind === CharacterKind.ENEMY || c.kind === CharacterKind.BOSS)
          .map((c) => ({ id: c.id, name: c.name, detail: c.classSummary }));
      } catch (err) {
        this.error.set(castingErrorText(err));
        return;
      }
    }
    openCastOut(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      casterId: npc ? '' : this.characterId(),
      casterName: npc ? '' : this.characterName(),
      master: this.master(),
      diceMode: this.diceMode(),
      preference: this.preference(),
      ...(npcCasters ? { npcCasters } : {}),
    }).subscribe(() => void this.load());
  }

  protected setTyped(id: string, value: string): void {
    this.typed.update((t) => ({ ...t, [id]: value }));
  }

  /** "Concluir conjuração": the master says the casting time has passed. */
  protected async finish(c: OutsideCast): Promise<void> {
    const sum = Number(this.typed()[c.id]);
    const dice = this.physical()
      ? Number.isFinite(sum) && sum > 0
        ? { typedSum: sum }
        : null
      : { inApp: true as const };
    await this.act(c.id, 'finish', () =>
      this.api.confirm(
        this.campaignId(),
        c.id,
        dice,
        this.keyOf(`finish-${c.id}-${JSON.stringify(dice)}`).keyFor(dice),
      ),
    );
  }

  /** "Parar a conjuração": the casting fails and spends no slot. */
  protected async stop(c: OutsideCast): Promise<void> {
    await this.act(c.id, 'stop', () =>
      this.api.abandon(this.campaignId(), c.id, this.keyOf(`stop-${c.id}`).keyFor(c.id)),
    );
  }

  /** "Encerrar": asks first when it is a concentration (its effects go with it). */
  protected end(c: OutsideCast): void {
    const body = c.concentrating
      ? `A concentração de ${c.casterName} em ${c.spellNamePt} termina e os efeitos dela acabam.`
      : `${c.spellNamePt} de ${c.casterName} termina e os efeitos dela acabam.`;
    openCastConfirm(this.dialog, this.bottomSheet, {
      title: `Encerrar ${c.spellNamePt} de ${c.casterName}?`,
      body,
      confirm: 'Encerrar',
      cancel: 'Cancelar',
    }).subscribe((sure) => {
      if (sure === true) {
        void this.act(c.id, 'end', () =>
          this.api.end(this.campaignId(), c.id, this.keyOf(`end-${c.id}`).keyFor(c.id)),
        );
      }
    });
  }

  private async act(id: string, what: string, run: () => Promise<unknown>): Promise<void> {
    this.busyId.set(`${what}-${id}`);
    this.error.set('');
    try {
      await run();
      await this.load();
    } catch (err) {
      this.error.set(castingErrorText(err));
    } finally {
      this.busyId.set('');
    }
  }

  protected busy(what: string, id: string): boolean {
    return this.busyId() === `${what}-${id}`;
  }

  /** The player may stop or end a cast of their own character; the master any. */
  protected mine(c: OutsideCast): boolean {
    return this.master() || c.casterId === this.characterId();
  }
}
