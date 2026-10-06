import { ChangeDetectionStrategy, Component, DestroyRef, computed, effect, inject, input, signal, untracked } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { RouterLink } from '@angular/router';
import { timestampDate } from '@bufbuild/protobuf/wkt';

import { PuzzleKind } from '../../../../../gen/meurpg/play/v1/puzzles_pb';
import { PuzzlePlay } from '../../../../core/puzzles/puzzle-play';
import { PuzzleSessionState } from '../../../../core/puzzles/puzzle-session';
import {
  clockOf,
  kindName,
  lastMoveParts,
  litCount,
  litWords,
  pillarsChangedText,
  agoText,
} from '../../../../core/puzzles/puzzle-format';
import { PuzzlesClient } from '../../../../core/puzzles/puzzles-client';
import { LivePill } from '../../../../shared/live-pill/live-pill';
import { PillarsBoard } from '../../../../shared/puzzle-boards/pillars-board';
import { PuzzleHost } from '../../../../shared/puzzle-boards/puzzle-host';
import { sessionSince } from '../../../../shared/session-time/session-time';
import type { LiveSessionVm } from '../../live-session.types';

/** How long the dashed frame of "just changed" stays on what another person moved. */
const CHANGED_MS = 6000;

/**
 * A shown puzzle, played by a player (MR-038, RN-27, RN-10; E10-06 states 6 to 9): the master's clue, the board, what is
 * going on (who moved last, how many lights are lit), the hints the master released, and, once solved, "Resolvido" with the
 * master's own words. It sits in the session page's place (the session page keeps the one stream and says when the puzzle
 * changed; this page reads the run again then), and "Voltar para a sessão" is a link back.
 *
 * - **The board follows the server.** Every move goes with its own idempotency key and retries with the same one (`PuzzlePlay`).
 * - **Nothing is computed here.** The lights, the wheels and the pillars are what the server sent; the win is the server's; the
 *   page never reads or guesses a solution (it never receives one).
 * - A puzzle the master closed says so and leaves the way back; one solved or stopped is frozen (the board is `aria-disabled`).
 * - 7 × 7 keeps 48 px lights at 390 px, and at 320 px the board goes edge to edge with 44 px lights (`app-lights-board`).
 */
@Component({
  selector: 'app-puzzle-play',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [LivePill, MatButtonModule, MatIconModule, MatProgressSpinnerModule, PillarsBoard, PuzzleHost, RouterLink],
  templateUrl: './puzzle-play.html',
  styleUrl: './puzzle-play.scss',
})
export class PuzzlePlayPage {
  private readonly api = inject(PuzzlesClient);
  private readonly destroyRef = inject(DestroyRef);

  readonly campaignId = input.required<string>();
  readonly puzzleId = input.required<string>();
  readonly session = input.required<LiveSessionVm>();
  /** The session page's own list state: its `tick` says when this puzzle changed. */
  readonly state = input.required<PuzzleSessionState>();
  /** The player's character, so "Você tocou numa luz". */
  readonly ownName = input('');
  readonly reconnecting = input(false);

  protected readonly Kind = PuzzleKind;
  /** The puzzle's moves and run; `open` starts it over for another puzzle (going from `?quebra-cabeca=A` to `B` loads B). */
  protected readonly play = new PuzzlePlay(this.api, () => this.campaignId());
  protected readonly now = signal(new Date());

  protected readonly run = computed(() => this.play.run());
  protected readonly lights = computed(() => litCount(this.run()?.state));
  protected readonly last = computed(() => lastMoveParts(this.run()?.lastMove, this.ownName()));
  protected readonly lastAgo = computed(() => {
    const at = this.run()?.lastMove?.at;
    return at ? agoText(timestampDate(at), this.now()) : '';
  });
  /** What another person just changed, for a few seconds; your own moves are not outlined. */
  protected readonly changed = computed<readonly number[]>(() => {
    const last = this.run()?.lastMove;
    if (!last?.at || (this.ownName() !== '' && last.characterName === this.ownName())) {
      return [];
    }
    return this.now().getTime() - timestampDate(last.at).getTime() < CHANGED_MS ? last.changed : [];
  });
  protected readonly frozen = computed(() => !!this.run()?.solved || !!this.run()?.stopped);
  protected readonly since = computed(() => sessionSince(this.session().startedAt));
  protected readonly kind = computed(() => this.run()?.kind ?? PuzzleKind.UNSPECIFIED);
  protected readonly kindWord = computed(() => kindName(this.kind()));
  protected readonly solvedAt = computed(() => {
    const at = this.run()?.solvedAt;
    return at ? clockOf(timestampDate(at)) : '';
  });
  /** The words of a lock's wheels now, "Lua · Lua · Onda · Estrela". */
  protected readonly wheelWords = computed(() => {
    const run = this.run();
    const state = run?.state?.kind;
    return state?.case === 'lock' ? state.value.wheels.map((w) => run?.symbols[w]?.namePt ?? '').join(' · ') : '';
  });
  protected readonly faceList = computed(() => (this.run()?.symbols ?? []).map((s) => ({ key: s.key, namePt: s.namePt })));
  protected readonly instruction = computed(() => {
    switch (this.kind()) {
      case PuzzleKind.LIGHTS:
        return 'Toque numa luz: ela e as quatro vizinhas trocam. Apague todas.';
      case PuzzleKind.LOCK:
        return 'Gire as rodas até a combinação certa.';
      default:
        return 'Gire os pilares até ficarem como o mural.';
    }
  });
  /** What the page says once it is solved, by kind. */
  protected readonly doneText = computed(() => {
    switch (this.kind()) {
      case PuzzleKind.LIGHTS:
        return 'O quebra-cabeça terminou. Todas as luzes estão apagadas.';
      case PuzzleKind.LOCK:
        return 'O quebra-cabeça terminou. A fechadura abriu.';
      default:
        return 'O quebra-cabeça terminou. Os pilares combinam com o mural.';
    }
  });
  protected readonly mural = computed(() => this.run()?.mural?.pillars ?? []);
  protected readonly linked = computed(() => {
    const config = this.run()?.config?.kind;
    return config?.case === 'pillars' && config.value.links.some((l) => l.alsoTurns.length > 0);
  });
  protected readonly changedPillars = computed(() => (this.kind() === PuzzleKind.PILLARS ? pillarsChangedText(this.run()?.lastMove?.changed ?? []) : ''));
  /** The first hint a player won alone (10.15b): everything before it is shared. */
  protected readonly sharedHints = computed(() => this.run()?.sharedHints ?? 0);
  /** What a screen reader hears after a move: who moved and, for the lights, how many are lit. */
  protected readonly announce = computed(() => {
    const last = this.last();
    if (!last) {
      return '';
    }
    const base = `${last.who} ${last.what}.`;
    return this.kind() === PuzzleKind.LIGHTS ? `${base} ${litWords(this.lights())}.` : base;
  });

  constructor() {
    // Another puzzle in the address: open it, and forget the one before (a late answer of that one is ignored).
    effect(() => {
      const id = this.puzzleId();
      untracked(() => void this.play.open(id));
    });
    // The session page says "this puzzle changed" (the stream hint, or a reconnection): read the run again. The version is a counter
    // for this puzzle, so two hints in one turn are both seen.
    let seen = 0;
    effect(() => {
      const version = this.state().versionOf(this.puzzleId());
      untracked(() => {
        if (version > seen) {
          void this.play.refresh();
        }
        seen = version;
      });
    });
    const timer = setInterval(() => this.now.set(new Date()), 1000);
    this.destroyRef.onDestroy(() => clearInterval(timer));
  }

  protected onMove(move: Parameters<PuzzlePlay['move']>[0]): void {
    void this.play.move(move);
  }
}
