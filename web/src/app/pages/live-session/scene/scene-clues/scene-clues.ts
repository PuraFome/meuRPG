import {
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  input,
  signal,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import type { SceneClue } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import {
  type CluePlayer,
  clueAudience,
  clueCount,
  playersWithout,
  revealedLine,
} from '../../../../core/maps/scene-clues';
import type { SceneState } from '../../../../core/play/scene-state';
import { openRevealSheet } from '../reveal-sheet/reveal-sheet';

/**
 * "Pistas" in the open scene (E8-05, MR-029): the clues the master prepared,
 * each with who has it in words ("Ninguém ainda", "Revelada para todos às
 * 21:20", "Revelada só para Brisa às 21:26") and "Revelar", which opens the
 * dialog (a bottom sheet on a phone) where the master picks who gets it. The
 * button is outlined and 44px (48px on a phone, full width under the status):
 * the one filled button of the flow lives in the dialog. A clue revealed to
 * everyone has no button; one revealed to some keeps "Revelar aos outros".
 * There is no "Esconder de novo": what was said at the table stays said.
 *
 * After revealing, a green strip at the top of the list says what happened
 * (`role="status"`) and stays until the next action; the focus goes back to
 * the clue's button, or to the next one, or to the title.
 */
@Component({
  selector: 'app-scene-clues',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './scene-clues.html',
  styleUrl: './scene-clues.scss',
})
export class SceneClues {
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly campaignId = input.required<string>();
  readonly state = input.required<SceneState>();
  /** The campaign's player characters (the master sees them all). */
  readonly players = input<readonly CluePlayer[]>([]);
  /** A phone: the hint under the title is shorter. */
  readonly phone = input(false);

  protected readonly count = clueCount;
  protected readonly confirmation = signal('');
  protected readonly scene = computed(() => this.state().scene());
  protected readonly rows = computed(() => {
    const players = this.players();
    return (this.scene()?.clues ?? []).map((clue, i) => {
      const audience = clueAudience(clue, players);
      return {
        clue,
        n: i + 1,
        audience,
        line: revealedLine(clue, players),
        button: audience.kind === 'all' ? null : audience.kind === 'none' ? 'Revelar' : 'Revelar aos outros',
      };
    });
  });
  protected readonly icon = (kind: 'all' | 'some' | 'none') =>
    kind === 'all' ? 'groups' : kind === 'some' ? 'person' : 'visibility_off';

  protected reveal(clue: SceneClue, n: number): void {
    const scene = this.scene();
    if (!scene) {
      return;
    }
    openRevealSheet(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      clue,
      number: n,
      total: scene.clues.length,
      sceneName: scene.name,
      players: this.players(),
    }).subscribe((revealed) => {
      if (!revealed) {
        return;
      }
      this.state().clueRevealed(revealed);
      const line = revealedLine(revealed, this.players());
      this.confirmation.set(`Pista r${line.slice(1)}.`);
      this.focusAfter(revealed.id);
    });
  }

  /** The clue's button when it is still there, else the next one's, else the title. */
  private focusAfter(clueId: string): void {
    afterNextRender(
      () => {
        const root = this.host.nativeElement;
        const own = root.querySelector<HTMLElement>(`[data-reveal="${clueId}"]`);
        if (own) {
          own.focus();
          return;
        }
        const clues = this.scene()?.clues ?? [];
        const index = clues.findIndex((c) => c.id === clueId);
        const next = clues
          .slice(index + 1)
          .map((c) => root.querySelector<HTMLElement>(`[data-reveal="${c.id}"]`))
          .find((e) => e);
        (next ?? root.querySelector<HTMLElement>('.sc__title'))?.focus();
      },
      { injector: this.injector },
    );
  }

  protected withoutCount(clue: SceneClue): number {
    return playersWithout(clue, this.players()).length;
  }
}
