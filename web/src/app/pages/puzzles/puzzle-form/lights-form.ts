import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Draft } from '../../../core/puzzles/puzzle-draft';
import { litCount, litWords, moveWord } from '../../../core/puzzles/puzzle-format';
import { PuzzleKind } from '../../../../gen/meurpg/play/v1/puzzles_pb';
import { LightsBoard } from '../../../shared/puzzle-boards/lights-board';
import { type PickOption, PickGroup } from '../fields/pick-group';
import type { StartPreview } from './start-preview';

const SIZES: readonly PickOption<number>[] = [3, 4, 5, 6, 7].map((n) => ({
  value: n,
  title: String(n),
}));

/**
 * The form of "Apagar as luzes" (E10-06 state 2): the size of the board (3 to 7 a side) and the start the server drew. The
 * start is only a preview (no light is a button): "Gerar outro começo" asks for another, and what is on screen is what is saved
 * (the seed goes with it). The count of lit lights and the fewest touches come from the server's answer.
 */
@Component({
  selector: 'app-lights-form',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [LightsBoard, MatButtonModule, MatIconModule, PickGroup],
  template: `
    <app-pick-group legend="Tamanho do painel" layout="segments" [options]="sizes" [value]="draft().size" (valueChange)="patch.emit({ size: $event })" />

    <section class="start" aria-labelledby="start-title">
      <h3 class="start__title" id="start-title">O começo</h3>
      @switch (preview().status) {
        @case ('ready') {
          @if (lit(); as lights) {
            <app-lights-board class="start__board" [size]="draft().size" [lit]="lights" mode="view" />
          }
          <p class="start__line" role="status">
            <strong>{{ litText() }}</strong>, {{ off() }} {{ off() === 1 ? 'apagada' : 'apagadas' }}.
            @if (preview().solvable) {
              <br />Dá para resolver em <strong>{{ preview().moves }} {{ word() }}</strong>.
            }
          </p>
        }
        @case ('error') {
          <p class="start__line start__line--bad" role="alert">{{ preview().message }}</p>
        }
        @default {
          <p class="start__line" role="status">Sorteando um começo...</p>
        }
      }
      <button matButton="outlined" type="button" class="start__again" [disabled]="preview().status === 'loading'" disabledInteractive [class.mr-button--off]="preview().status === 'loading'" (click)="preview().status !== 'loading' && again.emit()">
        <mat-icon aria-hidden="true">shuffle</mat-icon>Gerar outro começo
      </button>
      <p class="start__help">É só uma prévia: os jogadores é que jogam.</p>
    </section>
  `,
  styleUrl: './lights-form.scss',
})
export class LightsForm {
  protected readonly sizes = SIZES;
  readonly draft = input.required<Draft>();
  readonly preview = input.required<StartPreview>();
  readonly patch = output<Partial<Draft>>();
  readonly again = output<void>();

  protected readonly lit = computed(() => {
    const kind = this.preview().start?.kind;
    return kind?.case === 'lights' ? kind.value.lit : null;
  });
  protected readonly litText = computed(() => litWords(litCount(this.preview().start)));
  protected readonly off = computed(
    () => (this.lit()?.length ?? 0) - litCount(this.preview().start),
  );
  protected readonly word = computed(() => moveWord(PuzzleKind.LIGHTS, this.preview().moves));
}
