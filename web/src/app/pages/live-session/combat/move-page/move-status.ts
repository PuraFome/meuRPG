import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

/** What the page says about the square chosen (all of it from the server's options). */
export interface MoveSummary {
  readonly kind: 'idle' | 'ok' | 'refused';
  /** "Mover 2,1 m", or "Sem caminho reto". */
  readonly title: string;
  /** "Depois restam 6,9 m.", or the way out of a refusal. */
  readonly detail: string;
  /** "Sair do alcance do Goblin 2 pode provocar um ataque de oportunidade." (or ''). */
  readonly warning: string;
  /** The known trap this square is inside ("Fosso escondido"), or ''. */
  readonly trap: string;
}

/**
 * The line under the map on the "Mover" page (E6-10, E9-05, E9-13): the move
 * in words (a live region, so it works without seeing the map), a refusal as an
 * alert with the way out, the warning that leaving a reach may provoke an
 * opportunity attack (a warning: the server says "pode"), the known trap the
 * square is in, and "Desengajar", the text action that stops the warning.
 */
@Component({
  selector: 'app-move-status',
  imports: [MatButtonModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @switch (summary().kind) {
      @case ('ok') {
        <div class="status" role="status">
          <p class="status__title">{{ summary().title }}</p>
          <p class="status__text">{{ summary().detail }}</p>
        </div>
      }
      @case ('refused') {
        <div class="mr-notice mr-notice--danger" role="alert">
          <mat-icon aria-hidden="true">block</mat-icon>
          <p><strong class="err__title">{{ summary().title }}</strong>{{ summary().detail }}</p>
        </div>
      }
      @default {
        <p class="status__idle" role="status">{{ idle() }}</p>
      }
    }
    @if (summary().kind === 'ok' && summary().warning) {
      <div class="mr-notice mr-notice--warning">
        <mat-icon aria-hidden="true">warning</mat-icon>
        <p>
          {{ summary().warning }}
          @if (canDisengage()) {
            <span class="note">Com Desengajar, nenhum movimento deste turno provoca isso.</span>
          }
        </p>
      </div>
      @if (canDisengage()) {
        <button mat-button type="button" class="disengage" [disabled]="busy()" (click)="disengage.emit()">
          Desengajar (gasta a ação)
        </button>
      }
    }
    @if (summary().kind === 'ok' && summary().trap) {
      <div class="mr-notice mr-notice--warning">
        <mat-icon aria-hidden="true">warning</mat-icon>
        <p>Esse quadrado fica dentro do {{ summary().trap }}.</p>
      </div>
    }
  `,
  styleUrl: './move-status.scss',
})
export class MoveStatus {
  readonly summary = input.required<MoveSummary>();
  /** What to say when nothing is chosen yet. */
  readonly idle = input('Toque num quadrado destacado para escolher onde parar.');
  readonly canDisengage = input(false);
  readonly busy = input(false);
  readonly disengage = output<void>();
}
