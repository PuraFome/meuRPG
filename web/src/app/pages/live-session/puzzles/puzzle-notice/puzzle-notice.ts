import {
  ChangeDetectionStrategy,
  Component,
  effect,
  input,
  signal,
  untracked,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import type { PuzzleSummary } from '../../../../../gen/meurpg/play/v1/puzzles_pb';
import { PLAYED_TOGETHER, kindName } from '../../../../core/puzzles/puzzle-format';
import { joinDots } from '../../../../core/format/text';

/**
 * "O mestre mostrou um quebra-cabeça" (MR-038, E10-06 state 6): on the player's session page, above the board, one card for each
 * puzzle the master shows, with the garnet frame that says "session" (the same as the campaign's "Sessão" panel) and "Abrir o quebra-cabeça" in 48 px
 * (the artboard fills the first one, the one the table is playing; the others are outlined, so there is never more than one filled button). It never takes the focus or scrolls the
 * page: a screen reader hears "O mestre mostrou …" once, in a polite live region, when a new one arrives (the first
 * read is the baseline, so a puzzle shown before the player came is not news). A solved or stopped puzzle stays in the list
 * with its state in a word, so the way back to what the table solved is still there.
 */
@Component({
  selector: 'app-puzzle-notice',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule, RouterLink],
  template: `
    @for (p of puzzles(); track p.puzzleId; let first = $first) {
      <section class="card" [attr.aria-labelledby]="'pn-' + p.puzzleId">
        <p class="card__lead"><mat-icon aria-hidden="true">extension</mat-icon>{{ lead(p) }}</p>
        <h2 class="card__name" [id]="'pn-' + p.puzzleId">{{ p.name }}</h2>
        <p class="card__sub">{{ sub(p) }}</p>
        <a [matButton]="first && !p.solved ? 'filled' : 'outlined'" class="card__open" [routerLink]="['/campaigns', campaignId(), 'session']" [queryParams]="{ 'puzzle': p.puzzleId }">
          {{ p.solved ? 'Ver o quebra-cabeça' : 'Abrir o quebra-cabeça' }}
        </a>
      </section>
    }
    <p class="mr-visually-hidden" role="status" aria-live="polite">{{ announce() }}</p>
  `,
  styleUrl: './puzzle-notice.scss',
})
export class PuzzleNotice {
  readonly campaignId = input.required<string>();
  readonly puzzles = input.required<readonly PuzzleSummary[]>();

  protected readonly announce = signal('');
  private known: ReadonlySet<string> | null = null;

  protected readonly lead = (p: PuzzleSummary): string =>
    p.solved
      ? 'Resolvido pelo grupo'
      : p.stopped
        ? 'O quebra-cabeça parou'
        : 'O mestre mostrou um quebra-cabeça';
  protected readonly sub = (p: PuzzleSummary): string =>
    joinDots([kindName(p.kind), PLAYED_TOGETHER]);

  constructor() {
    effect(() => {
      const list = this.puzzles();
      untracked(() => {
        const ids = new Set(list.map((p) => p.puzzleId));
        if (this.known) {
          const fresh = list.find((p) => !this.known?.has(p.puzzleId));
          if (fresh) {
            this.announce.set(`O mestre mostrou o quebra-cabeça ${fresh.name}.`);
          }
        }
        this.known = ids;
      });
    });
  }
}
