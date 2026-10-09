import { Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { LogGroup } from '../../../../core/combat/combat-log';
import { PoolCardView } from './pool-card';

/**
 * The rounds of the log (E6-11, E6-14, E6-15): the latest round first, each
 * with "em andamento" or "encerrada", and a line for each entry: an icon for
 * the kind, the actor in bold and the sentence. What only the master sees
 * says so ("Só o mestre vê"). A `role="log"` region, scrollable and
 * focusable, so the keyboard can reach what is below.
 */
@Component({
  selector: 'app-log-list',
  imports: [MatIconModule, PoolCardView],
  template: `
    <div class="log" role="log" tabindex="0" [attr.aria-label]="'Registro do combate'">
      @for (g of groups(); track g.round) {
        <section class="round" [attr.aria-label]="g.title">
          <div class="round__head">
            <h3 class="round__title">{{ g.title }}</h3>
            <span class="round__status">{{ g.status }}</span>
          </div>
          <ul class="lines">
            @for (l of g.lines; track l.id) {
              <li class="line">
                <span class="line__icon" aria-hidden="true"><mat-icon>{{ l.icon }}</mat-icon></span>
                <span class="line__text">
                  @if (l.actor) {<b>{{ l.actor }}</b>}{{ l.text }}
                  @if (l.note; as note) {
                    <span class="line__note">{{ note }}</span>
                  }
                  @if (l.hidden) {
                    <span class="line__secret"><mat-icon aria-hidden="true">visibility_off</mat-icon>Só o mestre vê</span>
                  }
                </span>
                <!-- The card has the whole width of the line, under it, not the text's column. -->
                @if (l.card; as card) {
                  <app-pool-card class="line__card" [card]="card" (change)="conditions.emit($event)" />
                }
              </li>
            }
          </ul>
        </section>
      } @empty {
        <p class="empty">Ainda não aconteceu nada neste combate.</p>
      }
    </div>
  `,
  styleUrl: './log-list.scss',
})
export class LogList {
  readonly groups = input.required<readonly LogGroup[]>();
  /** "Mudar as condições" under a pool spell: the creature whose conditions the master opens. */
  readonly conditions = output<string>();
}
