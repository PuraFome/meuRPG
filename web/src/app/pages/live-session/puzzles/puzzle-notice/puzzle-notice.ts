import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  input,
  signal,
  untracked,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { NgTemplateOutlet } from '@angular/common';
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
 * with its state in a word, so the way back to what the table solved is still there: the latest result keeps its card, the older ones
 * fold into "Ver os N quebra-cabeças anteriores" (a row each, with its state and "Ver"), so they never pile up above the board.
 */
@Component({
  selector: 'app-puzzle-notice',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule, NgTemplateOutlet, RouterLink],
  template: `
    @for (p of current(); track p.puzzleId; let first = $first) {
      <ng-container *ngTemplateOutlet="card; context: { p: p, filled: first && !p.solved }" />
    }
    @if (older().length > 0) {
      <details class="older">
        <summary class="older__sum">
          {{ older().length === 1 ? 'Ver o quebra-cabeça anterior' : 'Ver os ' + older().length + ' quebra-cabeças anteriores' }}
        </summary>
        <ul class="older__list">
          @for (p of older(); track p.puzzleId) {
            <li class="older__row">
              <span class="older__name">{{ p.name }}</span>
              <span class="older__state">{{ lead(p) }}</span>
              <a class="older__open" [routerLink]="['/campaigns', campaignId(), 'session']" [queryParams]="{ 'puzzle': p.puzzleId }" [attr.aria-label]="'Ver o quebra-cabeça ' + p.name">Ver</a>
            </li>
          }
        </ul>
      </details>
    }
    <ng-template #card let-p="p" let-filled="filled">
      <section class="card" [attr.aria-labelledby]="'pn-' + p.puzzleId">
        <p class="card__lead"><mat-icon aria-hidden="true">extension</mat-icon>{{ lead(p) }}</p>
        <h2 class="card__name" [id]="'pn-' + p.puzzleId">{{ p.name }}</h2>
        <p class="card__sub">{{ sub(p) }}</p>
        <a [matButton]="filled ? 'filled' : 'outlined'" class="card__open" [routerLink]="['/campaigns', campaignId(), 'session']" [queryParams]="{ 'puzzle': p.puzzleId }">
          {{ p.solved ? 'Ver o quebra-cabeça' : 'Abrir o quebra-cabeça' }}
        </a>
      </section>
    </ng-template>
    <p class="mr-visually-hidden" role="status" aria-live="polite">{{ announce() }}</p>
  `,
  styleUrl: './puzzle-notice.scss',
})
export class PuzzleNotice {
  readonly campaignId = input.required<string>();
  readonly puzzles = input.required<readonly PuzzleSummary[]>();

  protected readonly announce = signal('');
  /** What the table can still play (in the order shown) and then the latest result; the older results are folded away. */
  private readonly finished = computed(() => this.puzzles().filter((p) => p.solved || p.stopped));
  protected readonly current = computed(() => {
    const done = this.finished();
    return [...this.puzzles().filter((p) => !p.solved && !p.stopped), ...done.slice(-1)];
  });
  protected readonly older = computed(() => this.finished().slice(0, -1));
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
