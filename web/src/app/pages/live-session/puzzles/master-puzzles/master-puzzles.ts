import {
  ChangeDetectionStrategy,
  Component,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  input,
  signal,
  untracked,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import {
  type MasterPuzzleRun,
  PuzzleRunStatus,
} from '../../../../../gen/meurpg/play/v1/puzzles_pb';
import { kindIcon, puzzleSummary } from '../../../../core/puzzles/puzzle-format';
import { puzzleErrorMessage } from '../../../../core/puzzles/puzzle-errors';
import { PuzzleSessionState } from '../../../../core/puzzles/puzzle-session';
import { PuzzlesClient } from '../../../../core/puzzles/puzzles-client';
import { focusWithRing } from '../../../../core/creatures/focus-ring';

/**
 * "Quebra-cabeças" on the master's session page (MR-038, E10-06 states 3 and 4): the campaign's puzzles that are not archived,
 * each with where it stands in this session, and "Mostrar aos jogadores" on the ones not shown. A shown puzzle opens its live
 * card (`MasterRun`) under the list, and the stream keeps it current (`puzzle_changed` reads the puzzle again, through the
 * session's list state). Showing needs no confirmation: it is the one thing the master does here that hurts nothing, and
 * "Fechar" is one question away.
 */
@Component({
  selector: 'app-master-puzzles',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './master-puzzles.html',
  styleUrl: './master-puzzles.scss',
})
export class MasterPuzzles {
  private readonly api = inject(PuzzlesClient);
  private readonly injector = inject(Injector);

  readonly campaignId = input.required<string>();
  readonly state = input.required<PuzzleSessionState>();

  protected readonly notice = signal('');
  protected readonly showing = signal<string | null>(null);
  protected readonly Status = PuzzleRunStatus;
  protected readonly icon = kindIcon;

  protected readonly runs = computed(() => this.state().runs());
  /** The ones the players see or solved: the live cards. A closed one is back to being a row. */
  protected readonly live = computed(() =>
    this.runs().filter(
      (r) => r.status === PuzzleRunStatus.SHOWN || r.status === PuzzleRunStatus.SOLVED,
    ),
  );

  constructor() {
    // One card is open at a time: when the chosen puzzle is no longer shown (or none was chosen), the first one that is takes its place.
    effect(() => {
      const live = this.live();
      const chosen = this.state().selectedId();
      untracked(() => {
        if (!live.some((r) => r.puzzle?.id === chosen)) {
          this.state().select(live[0]?.puzzle?.id ?? null);
        }
      });
    });
  }

  protected idOf(run: MasterPuzzleRun): string {
    return run.puzzle?.id ?? '';
  }

  protected sub(run: MasterPuzzleRun): string {
    return run.puzzle ? puzzleSummary(run.puzzle) : '';
  }

  protected tag(run: MasterPuzzleRun): {
    readonly word: string;
    readonly tone: string;
    readonly icon: string;
  } {
    switch (run.status) {
      case PuzzleRunStatus.SHOWN:
        return { word: 'Mostrado agora', tone: 'success', icon: 'visibility' };
      case PuzzleRunStatus.SOLVED:
        return { word: 'Resolvido', tone: 'success', icon: 'check' };
      case PuzzleRunStatus.CLOSED:
        return { word: 'Fechado', tone: '', icon: 'visibility_off' };
      default:
        return { word: 'Não mostrado', tone: '', icon: '' };
    }
  }

  protected async show(run: MasterPuzzleRun): Promise<void> {
    if (this.showing() !== null) {
      return;
    }
    const id = this.idOf(run);
    this.showing.set(id);
    this.notice.set('');
    try {
      this.state().replace(await this.api.show(this.campaignId(), id));
      // The row's button is gone: the new card (in the main column) takes the focus.
      this.state().select(id);
      afterNextRender(
        () => focusWithRing(document.querySelector<HTMLElement>('app-master-live h3')),
        { injector: this.injector },
      );
    } catch (err) {
      this.notice.set(puzzleErrorMessage(err, 'mostrar o quebra-cabeça'));
    } finally {
      this.showing.set(null);
    }
  }

  protected isOpen(run: MasterPuzzleRun): boolean {
    return this.state().selectedId() === this.idOf(run);
  }

  protected open(run: MasterPuzzleRun): void {
    this.state().select(this.idOf(run));
    afterNextRender(
      () => focusWithRing(document.querySelector<HTMLElement>('app-master-live h3')),
      { injector: this.injector },
    );
  }
}
