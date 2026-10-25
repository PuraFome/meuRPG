import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { DIRECTIONS, MAX_STEP, MIN_STEP } from '../../../core/maps/token-move';
import type { TokenMover } from '../../../core/maps/token-mover';

/**
 * "Mover Tor" and "3 selecionados" (B2-01, B2-02, B2-03, B2-13, B2-11) in the master's "Tokens" tab: what the master
 * says about the move (the direction pad and "Quadrados por clique", or a tap on a square of the map), what the server
 * will be asked ("Tor vai 3 quadrados para a direita · Distância: 4,5 m"), the button and what it answered.
 * Moving is the server's: nothing walks on a guess, so while a move is on its way the button waits ("Movendo Tor…")
 * and the token stays where it was. The notice says where a move stopped and why, in words, with an amber tone; a
 * refusal is a red one and says the token went back. Without a grid there are no squares to count, and it says so.
 */
@Component({
  selector: 'app-token-move-panel',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    @if (mover().chosen().length > 0) {
      <section class="mp" [attr.aria-labelledby]="'mp-title'">
        <h3 class="mp__title" id="mp-title">{{ title() }}</h3>
        @if (!mover().hasGrid()) {
          <p class="mp__lead">Este mapa não tem grade: sem quadrados não dá para contar o movimento. Arraste o token no mapa.</p>
        } @else if (several()) {
          <p class="mp__lead">
            Arraste um dos {{ mover().chosen().length }}: os outros vão junto, na mesma formação. Desmarque um para tirá-lo da seleção.
          </p>
        } @else {
          <p class="mp__lead">
            Toque num quadrado do mapa ou ande com os botões. Fora do combate não há limite de deslocamento.
          </p>
          <div class="mp__controls">
            <div>
              <p class="mp__label" id="mp-dir">Direção</p>
              <div class="pad" role="group" aria-labelledby="mp-dir">
                @for (d of directions; track d.id; let i = $index) {
                  @if (i === 4) {
                    <span class="pad__mid" aria-hidden="true"><mat-icon>open_with</mat-icon></span>
                  }
                  <button
                    type="button"
                    class="pad__btn"
                    [class.pad__btn--on]="mover().direction() === d.id"
                    [attr.aria-pressed]="mover().direction() === d.id"
                    [attr.aria-label]="'Ir ' + d.words"
                    (click)="mover().choose(d.id)"
                  >
                    <mat-icon aria-hidden="true">{{ d.icon }}</mat-icon>
                  </button>
                }
              </div>
            </div>
            <div class="steps">
              <p class="mp__label" id="mp-steps">Quadrados por toque</p>
              <div class="steps__row" role="group" aria-labelledby="mp-steps">
                <button
                  type="button"
                  class="pad__btn"
                  aria-label="Menos um quadrado"
                  [attr.aria-disabled]="mover().steps() <= min ? 'true' : null"
                  (click)="mover().setSteps(mover().steps() - 1)"
                >
                  <mat-icon aria-hidden="true">remove</mat-icon>
                </button>
                <output class="steps__n" aria-live="polite">{{ mover().steps() }}</output>
                <button
                  type="button"
                  class="pad__btn"
                  aria-label="Mais um quadrado"
                  [attr.aria-disabled]="mover().steps() >= max ? 'true' : null"
                  (click)="mover().setSteps(mover().steps() + 1)"
                >
                  <mat-icon aria-hidden="true">add</mat-icon>
                </button>
              </div>
              <p class="mp__hint">de 1 a 20 · cada quadrado tem 1,5 m</p>
            </div>
          </div>
        }

        @if (mover().summary(); as s) {
          <div class="box">
            <p class="box__title">{{ s.title }}</p>
            <p class="box__detail">{{ s.detail }}</p>
          </div>
        }

        @if (mover().busy()) {
          <p class="busy"><mat-icon aria-hidden="true">schedule</mat-icon>Movendo {{ firstName() }}…</p>
        }

        @if (mover().last(); as last) {
          <div class="box">
            @for (line of last.lines; track line) {
              <p class="box__detail">{{ line }}</p>
            }
          </div>
          <div class="mr-notice" [class.mr-notice--warning]="last.notice.tone === 'stopped'" [class.mr-notice--success]="last.notice.tone === 'done'" role="status">
            <mat-icon aria-hidden="true">{{ last.notice.tone === 'stopped' ? 'layers' : 'check_circle' }}</mat-icon>
            <p>
              @if (last.notice.lead) {
                <strong>{{ last.notice.lead }}</strong>
              }
              {{ last.notice.text }}
            </p>
          </div>
        }
        @if (mover().error(); as message) {
          <div class="mr-notice mr-notice--danger" role="alert">
            <mat-icon aria-hidden="true">error</mat-icon>
            <p>{{ message }}</p>
          </div>
        }

        @if (mover().hasGrid()) {
          <button
            type="button"
            class="go"
            [class.go--off]="!ready()"
            [attr.aria-disabled]="ready() ? null : 'true'"
            [attr.aria-describedby]="ready() ? null : 'mp-why'"
            (click)="move()"
          >
            {{ goLabel() }}
          </button>
          @if (!ready() && !mover().busy()) {
            <p class="mp__hint" id="mp-why">Escolha o quadrado de destino.</p>
          }
        }
        @if (several()) {
          <button type="button" class="clear" (click)="mover().clear()">Limpar seleção</button>
        }
        <p class="mr-visually-hidden" role="status" aria-live="polite">{{ mover().status() }}</p>
      </section>
    }
  `,
  styleUrl: './token-move-panel.scss',
})
export class TokenMovePanel {
  readonly mover = input.required<TokenMover>();
  protected readonly min = MIN_STEP;
  protected readonly max = MAX_STEP;
  protected readonly directions = DIRECTIONS;

  protected readonly several = computed(() => this.mover().chosen().length > 1);
  protected readonly firstName = computed(() => this.mover().anchor()?.name ?? '');
  protected readonly title = computed(() =>
    this.several() ? `${this.mover().chosen().length} selecionados` : `Mover ${this.firstName()}`,
  );
  protected readonly ready = computed(() => this.mover().target() !== null && !this.mover().busy());
  protected readonly goLabel = computed(() => {
    if (this.mover().busy()) {
      return `Movendo ${this.firstName()}…`;
    }
    return this.several()
      ? `Mover os ${this.mover().chosen().length} juntos`
      : `Mover ${this.firstName()}`;
  });

  protected move(): void {
    if (this.ready()) {
      void this.mover().move();
    }
  }
}
