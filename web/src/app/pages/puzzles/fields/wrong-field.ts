import { ChangeDetectionStrategy, Component, OnInit, inject, input, output, signal } from '@angular/core';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

import { type WrongDraft, type WrongOption, ATTEMPTS_MAX, MINUTES_MAX, MOVES_LIMIT_MAX } from '../../../core/puzzles/puzzle-draft';
import { type TrapChoice, SolveTargets } from '../../../core/puzzles/solve-targets';
import { Stepper } from './stepper';

interface Choice {
  readonly value: WrongOption;
  readonly title: string;
}

const NONE: Choice = { value: 'none', title: 'Nada acontece' };
const TRAP: Choice = { value: 'trap', title: 'Disparar uma armadilha do mapa' };
const ATTEMPTS: Choice = { value: 'attempts', title: 'Gastar uma tentativa do jogador' };
const LIMITS: Choice = { value: 'limits', title: 'Limite de jogadas ou de tempo' };

type Load = 'idle' | 'loading' | 'ready' | 'error';

let nextId = 0;

/**
 * "Ao errar" of every form (MR-038, RN-27, E10-12 states 1 to 3): what a wrong answer or a wrong bell does. One option at a time, as
 * a group of native radios; only the marked one opens its own fields, inside its card.
 *
 * - **A trap** (the riddle, the sequence and the cipher only): a trap point of one of the campaign's maps fires when a player errs. The
 *   master decides what it does, as with any trap fired by hand; it fires once, and he arms it again on the map.
 * - **An attempt** (the same three kinds): each player may err 1 to 10 times in a round; with none left their moves are refused, and
 *   "Recomeçar" gives them back.
 * - **A limit** (every kind): of moves in the round (1 to 200), of time (minutes, up to 4 h), or both; reaching one stops the puzzle.
 *
 * The lights, the lock and the pillars judge no move, so they offer only "Nada acontece" and the limits. What the server refuses
 * (`error`) stands under the options, in words.
 */
@Component({
  selector: 'app-wrong-field',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatFormFieldModule, MatInputModule, Stepper],
  providers: [SolveTargets],
  template: `
    <fieldset class="wf">
      <legend class="wf__title">Ao errar</legend>
      <p class="wf__help">O que um erro faz: uma resposta errada, ou um sino errado.</p>
      @for (o of choices(); track o.value) {
        <div class="opt" [class.opt--on]="o.value === wrong().option">
          <label class="opt__head">
            <input type="radio" [name]="name" [value]="o.value" [checked]="o.value === wrong().option" (change)="choose(o.value)" />
            <span class="opt__dot" aria-hidden="true"></span>
            <span class="opt__title">{{ o.title }}</span>
          </label>
          @if (o.value === wrong().option) {
            @switch (o.value) {
              @case ('trap') {
                <div class="opt__body">
                  @if (load() === 'loading') {
                    <p class="wf__note" role="status">Procurando as armadilhas dos mapas...</p>
                  } @else if (load() === 'error') {
                    <p class="wf__note wf__note--bad" role="alert">Não deu para ler os mapas. Volte e tente de novo.</p>
                  } @else if (traps().length === 0) {
                    <p class="wf__note">Nenhum mapa da campanha tem uma armadilha. Ponha um ponto de armadilha no editor do mapa.</p>
                  } @else {
                    <mat-form-field appearance="outline" subscriptSizing="dynamic" class="wf__field" [class.field-bad]="error()">
                      <mat-label>Armadilha do mapa</mat-label>
                      <select matNativeControl [value]="trapKey()" [attr.aria-invalid]="error() ? 'true' : null" (change)="pickTrap($any($event.target).value)">
                        <option value="" [selected]="trapKey() === ''">Escolha uma armadilha</option>
                        @for (t of traps(); track t.pointId) {
                          <option [value]="t.mapId + '|' + t.pointId" [selected]="t.mapId + '|' + t.pointId === trapKey()">{{ t.name }} · {{ t.mapName }}</option>
                        }
                      </select>
                    </mat-form-field>
                    <p class="wf__note">Quando um jogador errar, ela dispara e você decide o efeito, como em qualquer armadilha. Ela dispara uma vez: arme-a de novo no mapa para disparar outra.</p>
                  }
                </div>
              }
              @case ('attempts') {
                <div class="opt__body">
                  <app-stepper label="Tentativas por jogador" noun="tentativa" [value]="wrong().attempts" [min]="1" [max]="attemptsMax" (valueChange)="patch({ attempts: $event })" />
                  <p class="wf__note">Cada erro gasta uma. Sem tentativas, o jogador não joga mais até você recomeçar.</p>
                </div>
              }
              @case ('limits') {
                <div class="opt__body">
                  <div class="limits">
                    <mat-form-field appearance="outline" subscriptSizing="dynamic" class="wf__num" [class.field-bad]="error()">
                      <mat-label>Jogadas</mat-label>
                      <input matInput type="text" inputmode="numeric" autocomplete="off" [value]="wrong().movesText" [attr.aria-invalid]="error() ? 'true' : null" (input)="patch({ movesText: $any($event.target).value })" />
                      <mat-hint>De 1 a {{ movesMax }}</mat-hint>
                    </mat-form-field>
                    <mat-form-field appearance="outline" subscriptSizing="dynamic" class="wf__num" [class.field-bad]="error()">
                      <mat-label>Minutos</mat-label>
                      <input matInput type="text" inputmode="numeric" autocomplete="off" [value]="wrong().minutesText" [attr.aria-invalid]="error() ? 'true' : null" (input)="patch({ minutesText: $any($event.target).value })" />
                      <mat-hint>De 1 a {{ minutesMax }}</mat-hint>
                    </mat-form-field>
                  </div>
                  <p class="wf__note">Chegando ao limite, o quebra-cabeça para e você é avisado. Deixe um em branco para só contar o outro.</p>
                </div>
              }
            }
          }
        </div>
      }
      @if (wrong().combined) {
        <p class="wf__note">Este quebra-cabeça tem mais de uma regra de “Ao errar”. Ao salvar, fica só a escolhida.</p>
      }
      @if (error()) {
        <p class="wf__note wf__note--bad field-error" role="alert">{{ error() }}</p>
      }
    </fieldset>
  `,
  styleUrl: './wrong-field.scss',
})
export class WrongField implements OnInit {
  private readonly targets = inject(SolveTargets);
  protected readonly name = `wrong-${nextId++}`;
  protected readonly attemptsMax = ATTEMPTS_MAX;
  protected readonly movesMax = MOVES_LIMIT_MAX;
  protected readonly minutesMax = MINUTES_MAX;

  readonly campaignId = input.required<string>();
  /** The riddle, the sequence and the cipher judge a move: only they may fire a trap or spend an attempt. */
  readonly judged = input.required<boolean>();
  readonly wrong = input.required<WrongDraft>();
  /** What is wrong with the chosen option ("Escolha a armadilha que dispara."), after a try to save. */
  readonly error = input('');
  readonly wrongChange = output<WrongDraft>();

  protected readonly load = signal<Load>('idle');
  protected readonly traps = signal<readonly TrapChoice[]>([]);

  protected choices(): readonly Choice[] {
    return this.judged() ? [NONE, TRAP, ATTEMPTS, LIMITS] : [NONE, LIMITS];
  }

  ngOnInit(): void {
    this.targets.use(this.campaignId());
    if (this.wrong().option === 'trap') {
      this.loadTraps();
    }
  }

  protected trapKey(): string {
    const w = this.wrong();
    return w.mapId !== '' && w.pointId !== '' ? `${w.mapId}|${w.pointId}` : '';
  }

  protected choose(option: WrongOption): void {
    this.patch({ option });
    if (option === 'trap') {
      this.loadTraps();
    }
  }

  protected pickTrap(key: string): void {
    const [mapId = '', pointId = ''] = key.split('|');
    this.patch({ mapId, pointId });
  }

  protected patch(partial: Partial<WrongDraft>): void {
    this.wrongChange.emit({ ...this.wrong(), ...partial });
  }

  private loadTraps(): void {
    if (this.load() !== 'idle') {
      return;
    }
    this.load.set('loading');
    this.targets.traps().then(
      (traps) => {
        this.traps.set(traps);
        this.load.set('ready');
      },
      () => this.load.set('error'),
    );
  }
}
