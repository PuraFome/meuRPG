import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import type { MessageInitShape } from '@bufbuild/protobuf';

import type { PuzzleMoveSchema, PuzzleRun } from '../../../gen/meurpg/play/v1/puzzles_pb';
import type { CounterRow } from '../../core/puzzles/puzzle-format';
import type { SymbolFace } from '../../core/puzzles/puzzle-symbols';
import { CipherBoard } from './cipher-board';
import { LightsBoard, type LightPress } from './lights-board';
import { LockBoard } from './lock-board';
import { PillarsBoard } from './pillars-board';
import { RiddleBoard } from './riddle-board';
import { SequenceBoard } from './sequence-board';
import type { Turn } from './symbol-columns';

/**
 * The one place that knows which board a puzzle uses (MR-038, E10-06): the player's page and the master's live panel both put
 * a puzzle's run in here and get the right board, `play` for a player (every piece is a control and a move comes out) or `view`
 * for the master (the same board, static). The riddle, the sequence and the cipher (slice 10.15b) come out as the typed answer or
 * the bell struck; a kind the app does not draw says so instead of drawing nothing, and a new kind adds one case here and its own
 * board, and touches no other screen.
 *
 * It draws what the server sent and decides nothing: a press, a turn, an answer or a bell is turned into the move the server takes
 * and emitted. Whether the last answer was wrong (`verdict`) and why nothing can be typed (`blocked`) are the page's to say.
 */
@Component({
  selector: 'app-puzzle-host',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CipherBoard, LightsBoard, LockBoard, PillarsBoard, RiddleBoard, SequenceBoard],
  template: `
    @switch (state()?.kind?.case) {
      @case ('lights') {
        @let lights = lightsOf();
        <app-lights-board [size]="lights.size" [lit]="lights.lit" [mode]="mode()" [changed]="changed()" [hints]="hints()" [disabled]="disabled()" (press)="onPress($event)" />
      }
      @case ('lock') {
        <app-lock-board [wheels]="positions()" [faces]="faces()" [mode]="mode()" [changed]="changed()" [disabled]="disabled()" (turn)="onTurn('lock', $event)" />
      }
      @case ('pillars') {
        <app-pillars-board [pillars]="positions()" [faces]="faces()" [mode]="mode()" [changed]="changed()" [disabled]="disabled()" (turn)="onTurn('pillars', $event)" />
      }
      @case ('riddle') {
        <app-riddle-board [text]="riddleText()" [mode]="mode()" [verdict]="verdict()" [blocked]="blocked()" [busy]="busy()" [counters]="counters()" (answer)="onAnswer('riddle', $event)" />
      }
      @case ('cipher') {
        <app-cipher-board [ciphertext]="ciphertext()" [mode]="mode()" [verdict]="verdict()" [blocked]="blocked()" [busy]="busy()" [counters]="counters()" (answer)="onAnswer('cipher', $event)" />
      }
      @case ('sequence') {
        @if (run().sequence; as playback) {
          @if (mode() === 'play') {
            <app-sequence-board [playback]="playback" [faces]="faces()" [progress]="progress()" [disabled]="disabled()" [blocked]="blocked()" [waiting]="busyCount()" [note]="note()" (strike)="onStrike($event)" />
          } @else {
            <p class="count">Passos certos <strong>{{ progress() }} de {{ playback.totalSteps }}</strong></p>
          }
        }
      }
      @default {
        <p class="unknown">Este tipo de quebra-cabeça ainda não abre aqui.</p>
      }
    }
  `,
  styles: `
    :host {
      display: block;
    }

    .unknown,
    .count {
      margin: 0;
      color: var(--mr-ink-muted);
    }

    .count strong {
      color: var(--mr-ink);
    }
  `,
})
export class PuzzleHost {
  /** The run as a player reads it (the master's own read carries the same message in `MasterPuzzleRun.run`). */
  readonly run = input.required<PuzzleRun>();
  readonly mode = input<'play' | 'view'>('view');
  /** The lights, wheels or pillars the last move changed (the server's `changed`). */
  readonly changed = input<readonly number[]>([]);
  /** The lights the master's hint rings (a shortest way). Only the master's page ever passes it. */
  readonly hints = input<readonly number[]>([]);
  readonly disabled = input(false);
  /** The last typed answer was wrong, for the riddle and the cipher. */
  readonly verdict = input<'' | 'wrong'>('');
  /** Why nothing can be typed now, for the riddle and the cipher. */
  readonly blocked = input('');
  /** A move is on its way. */
  readonly busy = input(false);
  /** Moves made and not answered yet (the sequence's bells wait in a line). */
  readonly busyCount = input(0);
  /** The counters of "Ao errar" the riddle and the cipher draw beside their button. */
  readonly counters = input<readonly CounterRow[]>([]);
  /** The sequence's "Errou o passo 4." line, when the last bell was wrong. */
  readonly note = input<{ readonly lead: string; readonly text: string } | null>(null);

  /** A move, as `MakePuzzleMove` takes it. */
  readonly move = output<MessageInitShape<typeof PuzzleMoveSchema>>();

  protected readonly state = computed(() => this.run().state);

  protected readonly lightsOf = computed(() => {
    const config = this.run().config?.kind;
    const state = this.state()?.kind;
    return {
      size: config?.case === 'lights' ? config.value.size : 0,
      lit: state?.case === 'lights' ? state.value.lit : [],
    };
  });

  /** The wheels or the pillars, from the state. */
  protected readonly positions = computed<readonly number[]>(() => {
    const state = this.state()?.kind;
    return state?.case === 'lock'
      ? state.value.wheels
      : state?.case === 'pillars'
        ? state.value.pillars
        : [];
  });

  protected readonly riddleText = computed(() => {
    const config = this.run().config?.kind;
    return config?.case === 'riddle' ? config.value.text : '';
  });
  protected readonly ciphertext = computed(() => {
    const config = this.run().config?.kind;
    return config?.case === 'cipher' ? config.value.ciphertext : '';
  });
  protected readonly progress = computed(() => {
    const state = this.state()?.kind;
    return state?.case === 'sequence' ? state.value.progress : 0;
  });

  protected readonly faces = computed<readonly SymbolFace[]>(() =>
    this.run().symbols.map((s) => ({ key: s.key, namePt: s.namePt })),
  );

  protected onPress(press: LightPress): void {
    this.move.emit({ kind: { case: 'lights', value: { row: press.row, col: press.col } } });
  }

  protected onAnswer(kind: 'riddle' | 'cipher', text: string): void {
    this.move.emit(
      kind === 'riddle'
        ? { kind: { case: 'riddle', value: { answer: text } } }
        : { kind: { case: 'cipher', value: { text } } },
    );
  }

  protected onStrike(bell: number): void {
    this.move.emit({ kind: { case: 'sequence', value: { bell } } });
  }

  protected onTurn(kind: 'lock' | 'pillars', turn: Turn): void {
    this.move.emit(
      kind === 'lock'
        ? { kind: { case: 'lock', value: { wheel: turn.index, delta: turn.delta } } }
        : { kind: { case: 'pillars', value: { pillar: turn.index, delta: turn.delta } } },
    );
  }
}
