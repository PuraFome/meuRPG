import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { ListState } from '../../../core/free-dice/free-dice-state';
import { type FreeRollVm, logParts } from '../../../core/free-dice/free-roll';

/** How many rolls the log shows before "Mostrar as outras N". */
const SHOWN = 10;

let nextId = 0;

/**
 * "Registro da sessão" (MR-052, B5-04): the free rolls of the session as the table's log, newest first
 * ("Rolagem livre · Brisa: 2d6 + 3 = 11 (4, 4)"). The master's own secret rolls are in it with the "Só você" badge;
 * a player's list never holds one. It sits under the stage, so no screen's layout moves for it. It is a small
 * section, not a panel: ten lines, the rest on request.
 */
@Component({
  selector: 'app-free-roll-log',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule],
  template: `
    <section class="log" [attr.aria-labelledby]="headingId">
      <h2 class="log__title" [id]="headingId">Registro da sessão</h2>
      @if (rolls().length > 0) {
        <ul class="log__list">
          @for (r of shown(); track r.id) {
            <li class="log__row">
              @let p = parts(r);
              @if (p.who) {
                <strong>{{ p.who }}</strong>
              }
              {{ p.rest }}
              @if (r.secret) {
                <span class="badge"><mat-icon aria-hidden="true">visibility_off</mat-icon>Só você</span>
              }
            </li>
          }
        </ul>
        @if (hidden() > 0) {
          <button mat-button type="button" (click)="all.set(true)">
            Mostrar as outras {{ hidden() }}
          </button>
        }
      } @else if (listState() === 'loading') {
        <p class="mr-muted" role="status">Carregando o registro…</p>
      } @else if (listState() === 'failed') {
        <div class="mr-notice mr-notice--danger" role="alert">
          <mat-icon aria-hidden="true">error</mat-icon>
          <p><strong>Não deu para carregar o registro.</strong> As rolagens novas aparecem aqui mesmo assim.</p>
        </div>
        <button mat-button type="button" (click)="retry.emit()">Tentar de novo</button>
      } @else {
        <p class="mr-muted">Ninguém rolou dados nesta sessão ainda. As rolagens da mesa aparecem aqui.</p>
      }
    </section>
  `,
  styleUrl: './free-roll-log.scss',
})
export class FreeRollLog {
  /** Newest first: the table's rolls, with the master's secret ones when the caller is the master. */
  readonly rolls = input.required<readonly FreeRollVm[]>();
  readonly listState = input.required<ListState>();
  readonly retry = output<void>();

  protected readonly headingId = `free-log-${nextId++}`;
  protected readonly all = signal(false);
  protected readonly shown = computed(() =>
    this.all() ? this.rolls() : this.rolls().slice(0, SHOWN),
  );
  protected readonly hidden = computed(() => this.rolls().length - this.shown().length);

  protected parts(roll: FreeRollVm): { who: string; rest: string } {
    return logParts(roll);
  }
}
